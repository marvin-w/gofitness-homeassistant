package gamify

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/nutrition"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

func newFixture(t *testing.T) (*store.Store, context.Context, store.Profile, nutrition.Plan) {
	t.Helper()
	ctx := t.Context()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureUser(ctx, "u1", "Test"); err != nil {
		t.Fatal(err)
	}

	prof := store.Profile{
		UserID: "u1", DisplayName: "Test", Sex: "female", BirthDate: "1992-01-01",
		HeightCm: 168, StartWeightKg: 85, TargetWeightKg: 65,
		Activity: "light", Goal: "lose", Breastfeeding: "none",
		Prefs: store.DefaultPrefs(), SetupDone: true,
	}
	if err := st.SaveProfile(ctx, prof); err != nil {
		t.Fatal(err)
	}

	plan := nutrition.Calculate(nutrition.Profile{
		Sex: "female", Age: 34, HeightCm: 168, WeightKg: 85,
		Activity: "light", Goal: "lose", TargetWeight: 65,
	})
	return st, ctx, prof, plan
}

func TestLevelProgression(t *testing.T) {
	if LevelFor(0) != 1 {
		t.Errorf("0 XP should be level 1, got %d", LevelFor(0))
	}
	// Levels must be monotonic in XP and never go backwards.
	last := 1
	for xp := 0; xp < 50000; xp += 137 {
		lvl := LevelFor(xp)
		if lvl < last {
			t.Fatalf("level dropped from %d to %d at %d XP", last, lvl, xp)
		}
		last = lvl
	}
	if LevelFor(100000) <= LevelFor(1000) {
		t.Error("more XP must eventually mean a higher level")
	}
}

func TestLevelTitleAlwaysResolves(t *testing.T) {
	seen := map[string]bool{}
	for lvl := 1; lvl <= 100; lvl++ {
		title := LevelTitle(lvl)
		if title == "" {
			t.Fatalf("level %d has no title", lvl)
		}
		seen[title] = true
	}
	if len(seen) < 4 {
		t.Errorf("only %d distinct level titles across 100 levels", len(seen))
	}
	// Out-of-range levels must not panic or return an empty code.
	if LevelTitle(0) == "" || LevelTitle(-5) == "" || LevelTitle(9999) == "" {
		t.Error("out-of-range levels must still return a title code")
	}
}

func TestSetupBadgesUnlock(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	now := time.Now()

	stats, err := Compute(ctx, st, "u1", prof, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if !badgeUnlocked(stats, "setup_done") {
		t.Error("setup_done should unlock for a completed profile")
	}
	if badgeUnlocked(stats, "first_weigh_in") {
		t.Error("first_weigh_in unlocked without a weigh-in")
	}
	if stats.TotalCount != len(catalogue) {
		t.Errorf("total badge count = %d, want %d", stats.TotalCount, len(catalogue))
	}
}

func TestBadgesUnlockOnceAndReportFresh(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	now := time.Now()

	first, err := Compute(ctx, st, "u1", prof, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.RecentlyEarned) == 0 {
		t.Error("the first computation should report freshly earned badges")
	}

	second, err := Compute(ctx, st, "u1", prof, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.RecentlyEarned) != 0 {
		t.Errorf("already-earned badges reported as new: %v", second.RecentlyEarned)
	}
	if second.UnlockedCount != first.UnlockedCount {
		t.Errorf("unlocked count changed on recompute: %d then %d",
			first.UnlockedCount, second.UnlockedCount)
	}
	if second.XP != first.XP {
		t.Errorf("XP changed without new activity: %d then %d", first.XP, second.XP)
	}
}

func TestStreakCounting(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	now := time.Now()

	// Five consecutive days ending today.
	for i := 4; i >= 0; i-- {
		day := store.Day(now.AddDate(0, 0, -i))
		if _, err := st.AddFood(ctx, "u1", store.FoodEntry{Day: day, Name: "x", Kcal: 400}); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := Compute(ctx, st, "u1", prof, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CurrentStreak != 5 {
		t.Errorf("current streak = %d, want 5", stats.CurrentStreak)
	}
	if stats.LongestStreak < 5 {
		t.Errorf("longest streak = %d, want at least 5", stats.LongestStreak)
	}
	if !badgeUnlocked(stats, "streak_3") {
		t.Error("streak_3 should be unlocked at a 5-day streak")
	}
	if badgeUnlocked(stats, "streak_7") {
		t.Error("streak_7 unlocked too early")
	}
}

// Logging nothing yet today must not look like a broken streak.
func TestStreakSurvivesUntilEndOfDay(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	now := time.Now()

	for i := 3; i >= 1; i-- {
		day := store.Day(now.AddDate(0, 0, -i))
		if _, err := st.AddFood(ctx, "u1", store.FoodEntry{Day: day, Name: "x", Kcal: 400}); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := Compute(ctx, st, "u1", prof, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CurrentStreak != 3 {
		t.Errorf("streak = %d, want 3 (yesterday still counts)", stats.CurrentStreak)
	}
}

func TestStreakBreaks(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	now := time.Now()

	// Activity three and four days ago, nothing since.
	for _, i := range []int{3, 4} {
		if _, err := st.AddFood(ctx, "u1", store.FoodEntry{
			Day: store.Day(now.AddDate(0, 0, -i)), Name: "x", Kcal: 400,
		}); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := Compute(ctx, st, "u1", prof, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CurrentStreak != 0 {
		t.Errorf("current streak = %d, want 0", stats.CurrentStreak)
	}
	if stats.LongestStreak != 2 {
		t.Errorf("longest streak = %d, want 2", stats.LongestStreak)
	}
}

func TestWeightProgressAndMilestones(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	now := time.Now()

	// 85 kg start, now 80 kg, target 65.
	if _, err := st.AddWeight(ctx, "u1", store.WeightEntry{WeightKg: 80}); err != nil {
		t.Fatal(err)
	}

	stats, err := Compute(ctx, st, "u1", prof, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.KgLost != 5 {
		t.Errorf("kg lost = %v, want 5", stats.KgLost)
	}
	if stats.KgToGoal != 15 {
		t.Errorf("kg to goal = %v, want 15", stats.KgToGoal)
	}
	if want := 5.0 / 20.0; stats.GoalProgress < want-0.01 || stats.GoalProgress > want+0.01 {
		t.Errorf("goal progress = %v, want %v", stats.GoalProgress, want)
	}
	if !badgeUnlocked(stats, "lost_1kg") || !badgeUnlocked(stats, "lost_5kg") {
		t.Error("1 kg and 5 kg badges should be unlocked")
	}
	if badgeUnlocked(stats, "lost_10kg") {
		t.Error("10 kg badge unlocked at 5 kg lost")
	}
	if stats.NextMilestone == nil {
		t.Fatal("no next milestone")
	}
	if stats.NextMilestone.WeightKg >= 80 {
		t.Errorf("next milestone %v should be below the current 80 kg", stats.NextMilestone.WeightKg)
	}
}

// Gaining weight while trying to lose must not read as progress.
func TestGoalProgressIgnoresWrongDirection(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	if _, err := st.AddWeight(ctx, "u1", store.WeightEntry{WeightKg: 88}); err != nil {
		t.Fatal(err)
	}
	stats, err := Compute(ctx, st, "u1", prof, plan, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if stats.GoalProgress != 0 {
		t.Errorf("goal progress = %v after gaining weight, want 0", stats.GoalProgress)
	}
	if stats.KgLost >= 0 {
		t.Errorf("kg lost = %v, want a negative number", stats.KgLost)
	}
}

func TestOnTargetWindow(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	now := time.Now()

	// Three days inside the ±15 % window, one well outside it.
	inside := plan.TargetKcal
	for i := 0; i < 3; i++ {
		if _, err := st.AddFood(ctx, "u1", store.FoodEntry{
			Day: store.Day(now.AddDate(0, 0, -i)), Name: "meal", Kcal: inside,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.AddFood(ctx, "u1", store.FoodEntry{
		Day: store.Day(now.AddDate(0, 0, -3)), Name: "feast", Kcal: inside * 2,
	}); err != nil {
		t.Fatal(err)
	}

	stats, err := Compute(ctx, st, "u1", prof, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.DaysOnTarget7 != 3 {
		t.Errorf("days on target = %d, want 3", stats.DaysOnTarget7)
	}
}

func TestHealthyBMIBadge(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	// 68 kg at 168 cm is a BMI of 24.1.
	if _, err := st.AddWeight(ctx, "u1", store.WeightEntry{WeightKg: 68}); err != nil {
		t.Fatal(err)
	}
	stats, err := Compute(ctx, st, "u1", prof, plan, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !badgeUnlocked(stats, "healthy_bmi") {
		t.Error("healthy_bmi should unlock inside the normal range")
	}
}

func TestBadgesSortUnlockedFirst(t *testing.T) {
	st, ctx, prof, plan := newFixture(t)
	stats, err := Compute(ctx, st, "u1", prof, plan, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	seenLocked := false
	for _, b := range stats.Badges {
		if !b.Unlocked {
			seenLocked = true
		} else if seenLocked {
			t.Error("an unlocked badge sorted after a locked one")
			break
		}
	}
	// Progress is always a sane fraction.
	for _, b := range stats.Badges {
		if b.Progress < 0 || b.Progress > 1 {
			t.Errorf("%s: progress %v out of range", b.Code, b.Progress)
		}
		if b.Unlocked && b.Progress != 1 {
			t.Errorf("%s: unlocked but progress %v", b.Code, b.Progress)
		}
	}
}

func TestBadgeCodesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range catalogue {
		if seen[d.Code] {
			t.Errorf("duplicate badge code %q", d.Code)
		}
		seen[d.Code] = true
		if d.Goal <= 0 {
			t.Errorf("%s: goal must be positive", d.Code)
		}
		if d.XP <= 0 {
			t.Errorf("%s: XP must be positive", d.Code)
		}
		if d.Icon == "" || d.Group == "" {
			t.Errorf("%s: missing icon or group", d.Code)
		}
	}
}

// badgeUnlocked reports whether a badge with the given code is unlocked.
func badgeUnlocked(s Stats, code string) bool {
	for _, b := range s.Badges {
		if b.Code == code {
			return b.Unlocked
		}
	}
	return false
}
