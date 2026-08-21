// Package nutrition implements the energy, macro and BMI maths behind the app.
//
// Everything here is deliberately conservative: the goal is a healthy BMI on a
// sustainable basis, so every calculation is clamped by safety floors rather
// than producing the most aggressive number that would "work" on paper.
package nutrition

import "math"

// Sex selects the Mifflin-St Jeor constant. Diverse/other falls back to the
// average of the two formulas, which is the least-wrong option available.
type Sex string

const (
	SexFemale Sex = "female"
	SexMale   Sex = "male"
	SexDivers Sex = "divers"
)

// Activity is the everyday movement level, excluding logged workouts.
type Activity string

const (
	ActivitySedentary Activity = "sedentary"
	ActivityLight     Activity = "light"
	ActivityModerate  Activity = "moderate"
	ActivityActive    Activity = "active"
	ActivityVeryHigh  Activity = "very_active"
)

// PAL returns the physical activity level multiplier applied to the BMR.
func (a Activity) PAL() float64 {
	switch a {
	case ActivitySedentary:
		return 1.2
	case ActivityLight:
		return 1.375
	case ActivityModerate:
		return 1.55
	case ActivityActive:
		return 1.725
	case ActivityVeryHigh:
		return 1.9
	default:
		return 1.375
	}
}

// Goal is what the user is working towards.
type Goal string

const (
	GoalLose     Goal = "lose"        // fat loss
	GoalMaintain Goal = "maintain"    // hold weight
	GoalMuscle   Goal = "gain_muscle" // lean gain
	GoalRecomp   Goal = "recomp"      // hold weight, swap fat for muscle
)

// Breastfeeding captures the extra energy cost of lactation.
type Breastfeeding string

const (
	BfNone      Breastfeeding = "none"
	BfPartial   Breastfeeding = "partial"   // with complementary feeding
	BfExclusive Breastfeeding = "exclusive" // first ~4-6 months
)

// ExtraKcal follows the DGE recommendation for lactating women: roughly
// +500 kcal/day while exclusively breastfeeding, half that once the baby also
// eats solids.
func (b Breastfeeding) ExtraKcal() float64 {
	switch b {
	case BfExclusive:
		return 500
	case BfPartial:
		return 250
	default:
		return 0
	}
}

// Active reports whether lactation safety rules apply.
func (b Breastfeeding) Active() bool { return b == BfPartial || b == BfExclusive }

// Profile is the input to every calculation in this package.
type Profile struct {
	Sex           Sex
	Age           int     // years
	HeightCm      float64 // centimetres
	WeightKg      float64 // current weight
	Activity      Activity
	Goal          Goal
	Breastfeeding Breastfeeding
	TargetWeight  float64 // optional; 0 means "derive a healthy target"
}

// BMR returns the basal metabolic rate in kcal/day (Mifflin-St Jeor, 1990).
func BMR(p Profile) float64 {
	base := 10*p.WeightKg + 6.25*p.HeightCm - 5*float64(p.Age)
	switch p.Sex {
	case SexMale:
		return base + 5
	case SexFemale:
		return base - 161
	default:
		// Average of both constants for non-binary users.
		return base + (5-161)/2.0
	}
}

// TDEE is the total daily energy expenditure: BMR scaled by everyday activity,
// plus the lactation surcharge. This is "what you burn".
func TDEE(p Profile) float64 {
	return BMR(p)*p.Activity.PAL() + p.Breastfeeding.ExtraKcal()
}

// BMI returns the body mass index. Returns 0 for an unusable height.
func BMI(weightKg, heightCm float64) float64 {
	if heightCm <= 0 {
		return 0
	}
	m := heightCm / 100
	return weightKg / (m * m)
}

// BMICategory classifies a BMI using the WHO cut-offs. It returns a stable
// code, not display text — the interface translates it, so the same value works
// in both languages.
func BMICategory(bmi float64) string {
	switch {
	case bmi <= 0:
		return "unknown"
	case bmi < 18.5:
		return "underweight"
	case bmi < 25:
		return "normal"
	case bmi < 30:
		return "overweight"
	case bmi < 35:
		return "obese_1"
	case bmi < 40:
		return "obese_2"
	default:
		return "obese_3"
	}
}

// HealthyWeightRange returns the weight span for a BMI of 18.5 to 24.9.
func HealthyWeightRange(heightCm float64) (low, high float64) {
	if heightCm <= 0 {
		return 0, 0
	}
	m := heightCm / 100
	return round1(18.5 * m * m), round1(24.9 * m * m)
}

// RecommendedTargetWeight picks a sensible goal weight when the user has not
// set one: the nearest edge of the healthy BMI range, or the current weight if
// it is already healthy.
func RecommendedTargetWeight(p Profile) float64 {
	low, high := HealthyWeightRange(p.HeightCm)
	if low == 0 {
		return p.WeightKg
	}
	switch {
	case p.WeightKg > high:
		// Aim for BMI 23: comfortably inside the range without chasing the edge.
		m := p.HeightCm / 100
		return round1(23 * m * m)
	case p.WeightKg < low:
		return low
	default:
		return round1(p.WeightKg)
	}
}

// Note is an advisory attached to a plan. Code identifies the message so the
// interface can render it in the user's language; Params carries any numbers
// the message interpolates.
type Note struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
}

// Plan is the calculated daily nutrition target for a profile.
type Plan struct {
	BMR              float64 `json:"bmr"`
	TDEE             float64 `json:"tdee"`
	TargetKcal       float64 `json:"target_kcal"`
	DeficitKcal      float64 `json:"deficit_kcal"` // negative means surplus
	ProteinG         float64 `json:"protein_g"`
	FatG             float64 `json:"fat_g"`
	CarbsG           float64 `json:"carbs_g"`
	FiberG           float64 `json:"fiber_g"`
	WaterMl          float64 `json:"water_ml"`
	BMI              float64 `json:"bmi"`
	BMICategory      string  `json:"bmi_category"`
	HealthyLowKg     float64 `json:"healthy_low_kg"`
	HealthyHighKg    float64 `json:"healthy_high_kg"`
	TargetWeightKg   float64 `json:"target_weight_kg"`
	WeeklyChangeKg   float64 `json:"weekly_change_kg"` // negative means losing
	EstimatedWeeks   int     `json:"estimated_weeks"`
	BreastfeedingAdd float64 `json:"breastfeeding_add_kcal"`
	Notes            []Note  `json:"notes"`
}

// kcalPerKgFat is the widely used energy content of one kilogram of body fat.
const kcalPerKgFat = 7700

// Calculate turns a profile into a daily plan. It never returns a target below
// the safety floor for the user's situation.
func Calculate(p Profile) Plan {
	bmr := BMR(p)
	tdee := TDEE(p)
	bmi := BMI(p.WeightKg, p.HeightCm)
	low, high := HealthyWeightRange(p.HeightCm)

	target := p.TargetWeight
	if target <= 0 {
		target = RecommendedTargetWeight(p)
	}

	plan := Plan{
		BMR:              math.Round(bmr),
		TDEE:             math.Round(tdee),
		BMI:              round1(bmi),
		BMICategory:      BMICategory(bmi),
		HealthyLowKg:     low,
		HealthyHighKg:    high,
		TargetWeightKg:   round1(target),
		BreastfeedingAdd: p.Breastfeeding.ExtraKcal(),
	}

	goal := p.Goal
	// Chasing a surplus while already above a healthy BMI is not the fastest
	// way to a healthy BMI. Recomposition gets you there without wild swings.
	if goal == GoalMuscle && bmi >= 27 {
		goal = GoalRecomp
		plan.Notes = append(plan.Notes, Note{Code: "recomp_suggested"})
	}

	var adjust float64
	switch goal {
	case GoalLose:
		adjust = -0.20 * tdee
		if adjust < -750 {
			adjust = -750
		}
	case GoalMuscle:
		adjust = 0.10 * tdee
		if adjust > 400 {
			adjust = 400
		}
	case GoalRecomp, GoalMaintain:
		adjust = 0
	}

	// Lactation guard rails: a deficit larger than ~300 kcal/day risks milk
	// supply, so it is capped regardless of the goal.
	if p.Breastfeeding.Active() && adjust < -300 {
		adjust = -300
		plan.Notes = append(plan.Notes, Note{Code: "bf_deficit_capped", Params: map[string]any{"kcal": 300}})
	}

	targetKcal := tdee + adjust

	// Absolute floors. Eating under these long-term costs muscle and nutrients.
	floor := 1500.0
	if p.Sex == SexMale {
		floor = 1800
	}
	if p.Breastfeeding.Active() {
		floor = math.Max(floor, 1800)
		if p.Breastfeeding == BfExclusive {
			floor = math.Max(floor, 2000)
		}
	}
	floor = math.Max(floor, bmr) // never eat below your own basal rate

	if targetKcal < floor {
		targetKcal = floor
		plan.Notes = append(plan.Notes, Note{Code: "kcal_floor_raised", Params: map[string]any{"kcal": math.Round(floor)}})
	}

	plan.TargetKcal = math.Round(targetKcal/10) * 10
	plan.DeficitKcal = math.Round(tdee - plan.TargetKcal)

	// Protein is scaled to a reference weight so users far above their healthy
	// range do not get an unreachable protein target.
	ref := math.Min(p.WeightKg, high)
	if ref <= 0 {
		ref = p.WeightKg
	}
	perKg := 1.6
	switch goal {
	case GoalLose:
		perKg = 2.0 // preserves muscle in a deficit
	case GoalMuscle, GoalRecomp:
		perKg = 1.8
	}
	protein := perKg * ref
	if p.Breastfeeding.Active() {
		protein += 20 // extra requirement while nursing
	}

	fatKcal := 0.28 * plan.TargetKcal
	fat := fatKcal / 9
	if minFat := 0.8 * ref; fat < minFat {
		fat = minFat
	}

	carbs := (plan.TargetKcal - protein*4 - fat*9) / 4
	minCarbs := 100.0
	if p.Breastfeeding.Active() {
		minCarbs = 160 // supports milk production
	}
	if carbs < minCarbs {
		// Buy the carbs back from fat before touching protein.
		carbs = minCarbs
		fat = (plan.TargetKcal - protein*4 - carbs*4) / 9
		if fat < 0.6*ref {
			fat = 0.6 * ref
			protein = (plan.TargetKcal - carbs*4 - fat*9) / 4
		}
	}

	plan.ProteinG = math.Round(protein)
	plan.FatG = math.Round(fat)
	plan.CarbsG = math.Round(carbs)
	plan.FiberG = math.Round(plan.TargetKcal / 1000 * 14) // ~14 g per 1000 kcal
	plan.WaterMl = math.Round(p.WeightKg * 35)
	if p.Breastfeeding.Active() {
		plan.WaterMl += 700
	}

	// Expected rate of change, from the daily energy gap.
	plan.WeeklyChangeKg = round2(-plan.DeficitKcal * 7 / kcalPerKgFat)
	if diff := target - p.WeightKg; math.Abs(diff) > 0.4 && plan.WeeklyChangeKg != 0 {
		weeks := diff / plan.WeeklyChangeKg
		if weeks > 0 {
			plan.EstimatedWeeks = int(math.Ceil(weeks))
		}
	}

	if p.Breastfeeding.Active() {
		plan.Notes = append(plan.Notes, Note{
			Code:   "bf_active",
			Params: map[string]any{"kcal": p.Breastfeeding.ExtraKcal()},
		})
	}
	if bmi > 0 && bmi < 18.5 && goal == GoalLose {
		plan.Notes = append(plan.Notes, Note{Code: "underweight_warning"})
	}

	return plan
}

// Milestone is one step on the way to the goal weight.
type Milestone struct {
	Index       int     `json:"index"`
	WeightKg    float64 `json:"weight_kg"`
	Reached     bool    `json:"reached"`
	IsGoal      bool    `json:"is_goal"`
	IsBMIHealth bool    `json:"is_bmi_healthy"` // crossing into a healthy BMI
}

// Milestones breaks the journey from start weight to target into ~2 kg steps so
// there is always a small win within reach, not just the far-away goal.
func Milestones(startKg, currentKg, targetKg, heightCm float64) []Milestone {
	if startKg <= 0 || targetKg <= 0 {
		return nil
	}
	total := targetKg - startKg
	if math.Abs(total) < 0.5 {
		return []Milestone{{Index: 1, WeightKg: round1(targetKg), Reached: true, IsGoal: true}}
	}

	losing := total < 0
	step := 2.0
	if n := math.Abs(total) / step; n > 12 {
		step = math.Abs(total) / 12 // keep the list readable for big journeys
	}

	_, healthyHigh := HealthyWeightRange(heightCm)

	var out []Milestone
	for i := 1; ; i++ {
		var w float64
		if losing {
			w = startKg - float64(i)*step
			if w <= targetKg {
				break
			}
		} else {
			w = startKg + float64(i)*step
			if w >= targetKg {
				break
			}
		}
		m := Milestone{Index: i, WeightKg: round1(w)}
		if losing {
			m.Reached = currentKg <= w
			// Flag the step where a healthy BMI is first reached.
			m.IsBMIHealth = healthyHigh > 0 && w <= healthyHigh && w+step > healthyHigh
		} else {
			m.Reached = currentKg >= w
		}
		out = append(out, m)
		if i > 40 {
			break
		}
	}

	goal := Milestone{
		Index:    len(out) + 1,
		WeightKg: round1(targetKg),
		IsGoal:   true,
	}
	if losing {
		goal.Reached = currentKg <= targetKg
	} else {
		goal.Reached = currentKg >= targetKg
	}
	return append(out, goal)
}

// pessimismFactor pads time estimates. Real weight change is slower than the
// pure calorie maths predicts — water retention, plateaus and the odd birthday
// cake all get in the way — so the app deliberately over-estimates how long
// things take. Beating a cautious estimate feels good; missing an optimistic
// one creates the pressure this app is trying to avoid.
const pessimismFactor = 1.4

// Projection is a conservative, low-pressure forecast of the journey from the
// current weight. Week counts are padded (see pessimismFactor) so they
// under-promise on purpose.
type Projection struct {
	CurrentBMI  float64 `json:"current_bmi"`
	BMICategory string  `json:"bmi_category"`
	// NextMilestoneKg is the next ~2 kg checkpoint not yet reached; equal to the
	// goal once the last checkpoint is passed.
	NextMilestoneKg      float64 `json:"next_milestone_kg"`
	WeeksToNextMilestone int     `json:"weeks_to_next_milestone"`
	WeeksToGoal          int     `json:"weeks_to_goal"`
	// Moving is false at maintenance or when already at the goal, so the UI can
	// show "you're there" rather than a meaningless week count.
	Moving bool `json:"moving"`
}

// Project forecasts progress from the current weight toward the target, using
// the plan's expected weekly change. Estimates are intentionally pessimistic.
func Project(startKg, currentKg, targetKg, heightCm, weeklyChangeKg float64) Projection {
	pr := Projection{
		CurrentBMI:      round1(BMI(currentKg, heightCm)),
		BMICategory:     BMICategory(BMI(currentKg, heightCm)),
		NextMilestoneKg: round1(targetKg),
	}
	if targetKg <= 0 {
		pr.NextMilestoneKg = round1(currentKg)
		return pr
	}

	// The next checkpoint is the first milestone the user has not reached yet.
	for _, m := range Milestones(startKg, currentKg, targetKg, heightCm) {
		if !m.Reached {
			pr.NextMilestoneKg = m.WeightKg
			break
		}
	}

	rate := math.Abs(weeklyChangeKg) // kg per week, regardless of direction
	if rate < 0.01 {
		return pr // maintenance, or no deficit/surplus: no honest ETA
	}
	pr.Moving = true

	weeks := func(fromKg, toKg float64) int {
		gap := math.Abs(fromKg - toKg)
		if gap < 0.05 {
			return 0
		}
		return int(math.Ceil(gap / rate * pessimismFactor))
	}
	pr.WeeksToNextMilestone = weeks(currentKg, pr.NextMilestoneKg)
	pr.WeeksToGoal = weeks(currentKg, targetKg)
	return pr
}

// KcalFromMacros reconstructs energy from macros (4/4/9 kcal per gram).
func KcalFromMacros(proteinG, carbsG, fatG float64) float64 {
	return math.Round(proteinG*4 + carbsG*4 + fatG*9)
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }
