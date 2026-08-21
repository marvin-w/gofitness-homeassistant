// Package gamify turns raw tracking data into the streaks, levels and badges
// that make the app worth opening every day.
//
// Everything here rewards *consistency*, never severity: there is no badge for
// eating very little and no bonus for losing weight quickly. Points come from
// showing up, hitting a sensible calorie window and moving.
package gamify

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/nutrition"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

// Badge describes an achievement. Title and description are codes translated by
// the interface; Icon is an emoji rendered as-is.
type Badge struct {
	Code       string  `json:"code"`
	Icon       string  `json:"icon"`
	XP         int     `json:"xp"`
	Unlocked   bool    `json:"unlocked"`
	UnlockedAt string  `json:"unlocked_at,omitempty"`
	Progress   float64 `json:"progress"` // 0..1 towards unlocking
	Goal       float64 `json:"goal"`
	Current    float64 `json:"current"`
	// Group orders badges in the UI: "start", "streak", "weight", "food",
	// "sport", "mealprep", "health".
	Group string `json:"group"`
}

// catalogue is the full badge list. Codes are stable; never rename one that has
// shipped or users will silently lose a badge they earned.
var catalogue = []struct {
	Code  string
	Icon  string
	XP    int
	Group string
	Goal  float64
}{
	{"setup_done", "🚀", 50, "start", 1},
	{"first_weigh_in", "⚖️", 25, "start", 1},
	{"first_meal_logged", "🍽️", 25, "start", 1},
	{"first_workout", "💪", 25, "start", 1},
	{"first_plan", "📅", 50, "start", 1},

	{"streak_3", "🔥", 30, "streak", 3},
	{"streak_7", "🔥", 75, "streak", 7},
	{"streak_14", "🔥", 150, "streak", 14},
	{"streak_30", "🔥", 300, "streak", 30},
	{"streak_100", "🏆", 1000, "streak", 100},

	{"weigh_ins_10", "📈", 60, "weight", 10},
	{"weigh_ins_50", "📈", 200, "weight", 50},
	{"lost_1kg", "🎯", 100, "weight", 1},
	{"lost_5kg", "🎯", 250, "weight", 5},
	{"lost_10kg", "🎯", 500, "weight", 10},
	{"goal_reached", "🥇", 1000, "weight", 1},
	{"healthy_bmi", "💚", 750, "health", 1},

	{"meals_25", "📔", 50, "food", 25},
	{"meals_100", "📔", 150, "food", 100},
	{"meals_500", "📔", 500, "food", 500},
	{"protein_week", "🥩", 120, "food", 7},
	{"on_target_7", "🎯", 150, "food", 7},

	{"workouts_5", "🏃", 60, "sport", 5},
	{"workouts_25", "🏃", 200, "sport", 25},
	{"workouts_100", "🏅", 600, "sport", 100},

	{"cooked_10", "👩‍🍳", 100, "mealprep", 10},
	{"cooked_50", "👨‍🍳", 300, "mealprep", 50},
	{"plans_4", "🛒", 200, "mealprep", 4},
}

// Stats is everything the interface needs to draw the gamification layer.
type Stats struct {
	XP            int     `json:"xp"`
	Level         int     `json:"level"`
	LevelTitle    string  `json:"level_title"` // translation code
	XPIntoLevel   int     `json:"xp_into_level"`
	XPForNext     int     `json:"xp_for_next"`
	LevelProgress float64 `json:"level_progress"` // 0..1

	CurrentStreak int `json:"current_streak"`
	LongestStreak int `json:"longest_streak"`
	WeighInStreak int `json:"weigh_in_streak"`

	Badges         []Badge `json:"badges"`
	UnlockedCount  int     `json:"unlocked_count"`
	TotalCount     int     `json:"total_count"`
	RecentlyEarned []Badge `json:"recently_earned"`

	Milestones     []nutrition.Milestone `json:"milestones"`
	NextMilestone  *nutrition.Milestone  `json:"next_milestone,omitempty"`
	KgToNext       float64               `json:"kg_to_next"`
	KgLost         float64               `json:"kg_lost"`
	KgToGoal       float64               `json:"kg_to_goal"`
	GoalProgress   float64               `json:"goal_progress"` // 0..1
	DaysOnTarget7  int                   `json:"days_on_target_7"`
	WeekKcalAvg    float64               `json:"week_kcal_avg"`
	WeekProteinAvg float64               `json:"week_protein_avg"`
}

// levelXP is the cumulative XP needed to reach each level. Growth is gentle so
// the early levels come quickly and the later ones still mean something.
func levelXP(level int) int {
	if level <= 1 {
		return 0
	}
	// 100, 250, 450, 700, 1000, ... (100 * n(n+1)/2 shifted)
	n := level - 1
	return 50 * n * (n + 3)
}

// LevelFor returns the level reached with a given XP total.
func LevelFor(xp int) int {
	lvl := 1
	for levelXP(lvl+1) <= xp && lvl < 100 {
		lvl++
	}
	return lvl
}

// levelTitles are translation codes, one per band of five levels.
var levelTitles = []string{
	"level_starter", "level_beginner", "level_routine", "level_committed",
	"level_strong", "level_expert", "level_master", "level_legend",
}

// LevelTitle returns the translation code for a level's title.
func LevelTitle(level int) string {
	i := (level - 1) / 5
	if i >= len(levelTitles) {
		i = len(levelTitles) - 1
	}
	if i < 0 {
		i = 0
	}
	return levelTitles[i]
}

// Compute gathers everything and evaluates which badges are now unlocked. It
// persists newly earned badges as a side effect, so calling it is what actually
// awards them.
func Compute(ctx context.Context, st *store.Store, userID string, prof store.Profile, plan nutrition.Plan, now time.Time) (Stats, error) {
	var s Stats

	weights, err := st.Weights(ctx, userID, 400)
	if err != nil {
		return s, err
	}
	foodCount, err := st.CountFoodEntries(ctx, userID)
	if err != nil {
		return s, err
	}
	workoutCount, err := st.CountWorkouts(ctx, userID)
	if err != nil {
		return s, err
	}
	cooked, err := st.CountCookedMeals(ctx, userID)
	if err != nil {
		return s, err
	}
	planCount, err := st.CountPlans(ctx, userID)
	if err != nil {
		return s, err
	}

	from := store.Day(now.AddDate(0, 0, -180))
	today := store.Day(now)
	active, err := st.ActiveDays(ctx, userID, from, today)
	if err != nil {
		return s, err
	}
	weighDays, err := st.WeighInDays(ctx, userID, from, today)
	if err != nil {
		return s, err
	}

	s.CurrentStreak = currentStreak(active, now)
	s.LongestStreak = longestStreak(active, now, 180)
	s.WeighInStreak = currentStreak(weighDays, now)

	// Weight progress.
	current := prof.StartWeightKg
	if len(weights) > 0 {
		current = weights[0].WeightKg
	}
	s.KgLost = round1(prof.StartWeightKg - current)
	targetW := plan.TargetWeightKg
	s.KgToGoal = round1(math.Abs(current - targetW))
	if total := math.Abs(prof.StartWeightKg - targetW); total > 0.01 {
		done := math.Abs(prof.StartWeightKg-current) / total
		// Only count movement in the intended direction.
		if (targetW < prof.StartWeightKg && current > prof.StartWeightKg) ||
			(targetW > prof.StartWeightKg && current < prof.StartWeightKg) {
			done = 0
		}
		s.GoalProgress = clamp01(done)
	} else {
		s.GoalProgress = 1
	}

	s.Milestones = nutrition.Milestones(prof.StartWeightKg, current, targetW, prof.HeightCm)
	for i := range s.Milestones {
		if !s.Milestones[i].Reached {
			m := s.Milestones[i]
			s.NextMilestone = &m
			s.KgToNext = round1(math.Abs(current - m.WeightKg))
			break
		}
	}

	// Last seven days of eating.
	weekFrom := store.Day(now.AddDate(0, 0, -6))
	totals, err := st.TotalsRange(ctx, userID, weekFrom, today)
	if err != nil {
		return s, err
	}
	var kcalSum, proteinSum float64
	proteinDays := 0
	for _, t := range totals {
		kcalSum += t.Kcal
		proteinSum += t.ProteinG
		// "On target" is a window, not a single number: eating a little under
		// or over the goal is normal and should still count.
		if plan.TargetKcal > 0 && t.Kcal >= plan.TargetKcal*0.85 && t.Kcal <= plan.TargetKcal*1.15 {
			s.DaysOnTarget7++
		}
		if plan.ProteinG > 0 && t.ProteinG >= plan.ProteinG*0.9 {
			proteinDays++
		}
	}
	if n := len(totals); n > 0 {
		s.WeekKcalAvg = math.Round(kcalSum / float64(n))
		s.WeekProteinAvg = math.Round(proteinSum / float64(n))
	}

	// Evaluate the catalogue.
	unlocked, err := st.UnlockedAchievements(ctx, userID)
	if err != nil {
		return s, err
	}

	bmi := nutrition.BMI(current, prof.HeightCm)
	progressOf := func(code string) (cur, goal float64) {
		switch code {
		case "setup_done":
			return boolVal(prof.SetupDone), 1
		case "first_weigh_in":
			return boolVal(len(weights) > 0), 1
		case "first_meal_logged":
			return boolVal(foodCount > 0), 1
		case "first_workout":
			return boolVal(workoutCount > 0), 1
		case "first_plan":
			return boolVal(planCount > 0), 1
		case "streak_3", "streak_7", "streak_14", "streak_30", "streak_100":
			return float64(s.LongestStreak), goalOf(code)
		case "weigh_ins_10", "weigh_ins_50":
			return float64(len(weights)), goalOf(code)
		case "lost_1kg", "lost_5kg", "lost_10kg":
			return math.Max(0, s.KgLost), goalOf(code)
		case "goal_reached":
			return boolVal(s.GoalProgress >= 1), 1
		case "healthy_bmi":
			return boolVal(bmi >= 18.5 && bmi < 25), 1
		case "meals_25", "meals_100", "meals_500":
			return float64(foodCount), goalOf(code)
		case "protein_week":
			return float64(proteinDays), 7
		case "on_target_7":
			return float64(s.DaysOnTarget7), 7
		case "workouts_5", "workouts_25", "workouts_100":
			return float64(workoutCount), goalOf(code)
		case "cooked_10", "cooked_50":
			return float64(cooked), goalOf(code)
		case "plans_4":
			return float64(planCount), 4
		}
		return 0, 1
	}

	for _, def := range catalogue {
		cur, goal := progressOf(def.Code)
		b := Badge{
			Code:    def.Code,
			Icon:    def.Icon,
			XP:      def.XP,
			Group:   def.Group,
			Goal:    goal,
			Current: math.Min(cur, goal),
		}
		if goal > 0 {
			b.Progress = clamp01(cur / goal)
		}
		if at, ok := unlocked[def.Code]; ok {
			b.Unlocked = true
			b.UnlockedAt = at
		} else if cur >= goal {
			// Earned right now — persist it and flag it as fresh.
			isNew, err := st.UnlockAchievement(ctx, userID, def.Code)
			if err != nil {
				return s, err
			}
			b.Unlocked = true
			b.UnlockedAt = now.UTC().Format(time.RFC3339)
			if isNew {
				s.RecentlyEarned = append(s.RecentlyEarned, b)
			}
		}
		if b.Unlocked {
			b.Progress = 1
			b.Current = goal
			s.XP += def.XP
			s.UnlockedCount++
		}
		s.Badges = append(s.Badges, b)
	}
	s.TotalCount = len(catalogue)

	// A little XP for sustained consistency, on top of the badges.
	s.XP += s.CurrentStreak * 5
	s.XP += min(foodCount, 500) * 2
	s.XP += min(workoutCount, 200) * 5

	s.Level = LevelFor(s.XP)
	s.LevelTitle = LevelTitle(s.Level)
	base := levelXP(s.Level)
	next := levelXP(s.Level + 1)
	s.XPIntoLevel = s.XP - base
	s.XPForNext = next - base
	if s.XPForNext > 0 {
		s.LevelProgress = clamp01(float64(s.XPIntoLevel) / float64(s.XPForNext))
	}

	// Show the most useful badges first: nearly-earned ones before distant ones.
	sort.SliceStable(s.Badges, func(i, j int) bool {
		if s.Badges[i].Unlocked != s.Badges[j].Unlocked {
			return s.Badges[i].Unlocked
		}
		return s.Badges[i].Progress > s.Badges[j].Progress
	})

	return s, nil
}

// goalOf reads the threshold out of the catalogue for a code.
func goalOf(code string) float64 {
	for _, d := range catalogue {
		if d.Code == code {
			return d.Goal
		}
	}
	return 1
}

// currentStreak counts consecutive active days ending today or yesterday.
// Yesterday still counts so the streak does not appear broken before the user
// has had a chance to log anything today.
func currentStreak(days map[string]bool, now time.Time) int {
	start := now
	if !days[store.Day(now)] {
		if !days[store.Day(now.AddDate(0, 0, -1))] {
			return 0
		}
		start = now.AddDate(0, 0, -1)
	}
	n := 0
	for i := 0; i < 400; i++ {
		if !days[store.Day(start.AddDate(0, 0, -i))] {
			break
		}
		n++
	}
	return n
}

// longestStreak finds the longest run of active days in the given window.
func longestStreak(days map[string]bool, now time.Time, window int) int {
	best, run := 0, 0
	for i := window; i >= 0; i-- {
		if days[store.Day(now.AddDate(0, 0, -i))] {
			run++
			if run > best {
				best = run
			}
		} else {
			run = 0
		}
	}
	return best
}

func boolVal(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
