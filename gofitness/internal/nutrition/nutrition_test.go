package nutrition

import (
	"math"
	"testing"
)

func TestBMRMifflinStJeor(t *testing.T) {
	// Worked examples from the published formula.
	tests := []struct {
		name string
		p    Profile
		want float64
	}{
		{
			"female 60kg 165cm 30y",
			Profile{Sex: SexFemale, WeightKg: 60, HeightCm: 165, Age: 30},
			10*60 + 6.25*165 - 5*30 - 161,
		},
		{
			"male 80kg 180cm 35y",
			Profile{Sex: SexMale, WeightKg: 80, HeightCm: 180, Age: 35},
			10*80 + 6.25*180 - 5*35 + 5,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BMR(tc.p); math.Abs(got-tc.want) > 0.01 {
				t.Errorf("BMR = %.2f, want %.2f", got, tc.want)
			}
		})
	}
}

func TestBMIAndCategory(t *testing.T) {
	tests := []struct {
		weight, height float64
		wantBMI        float64
		wantCat        string
	}{
		{60, 165, 22.04, "normal"},
		{50, 175, 16.33, "underweight"},
		{85, 175, 27.76, "overweight"},
		{110, 175, 35.92, "obese_2"},
	}
	for _, tc := range tests {
		got := BMI(tc.weight, tc.height)
		if math.Abs(got-tc.wantBMI) > 0.05 {
			t.Errorf("BMI(%v,%v) = %.2f, want %.2f", tc.weight, tc.height, got, tc.wantBMI)
		}
		if cat := BMICategory(got); cat != tc.wantCat {
			t.Errorf("BMICategory(%.2f) = %q, want %q", got, cat, tc.wantCat)
		}
	}
}

func TestBMIZeroHeight(t *testing.T) {
	if got := BMI(70, 0); got != 0 {
		t.Errorf("BMI with zero height = %v, want 0", got)
	}
	if cat := BMICategory(0); cat != "unknown" {
		t.Errorf("category for BMI 0 = %q, want unknown", cat)
	}
}

func TestHealthyWeightRange(t *testing.T) {
	low, high := HealthyWeightRange(170)
	if math.Abs(low-53.5) > 0.2 || math.Abs(high-72.0) > 0.2 {
		t.Errorf("range for 170cm = %.1f–%.1f, want ~53.5–72.0", low, high)
	}
	// Every weight in the range must classify as normal.
	for w := low; w <= high; w += 0.5 {
		if cat := BMICategory(BMI(w, 170)); cat != "normal" {
			t.Fatalf("weight %.1f inside healthy range classified as %q", w, cat)
		}
	}
}

// The safety floors are the whole point of this package, so they get the most
// attention: no configuration may produce a starvation target.
func TestTargetNeverBelowFloor(t *testing.T) {
	cases := []struct {
		name     string
		p        Profile
		minFloor float64
	}{
		{
			"small sedentary woman losing",
			Profile{Sex: SexFemale, Age: 45, HeightCm: 155, WeightKg: 58,
				Activity: ActivitySedentary, Goal: GoalLose},
			1500,
		},
		{
			"small sedentary man losing",
			Profile{Sex: SexMale, Age: 50, HeightCm: 168, WeightKg: 70,
				Activity: ActivitySedentary, Goal: GoalLose},
			1800,
		},
		{
			"exclusively breastfeeding losing",
			Profile{Sex: SexFemale, Age: 30, HeightCm: 160, WeightKg: 65,
				Activity: ActivitySedentary, Goal: GoalLose, Breastfeeding: BfExclusive},
			2000,
		},
		{
			"partially breastfeeding losing",
			Profile{Sex: SexFemale, Age: 30, HeightCm: 160, WeightKg: 65,
				Activity: ActivityLight, Goal: GoalLose, Breastfeeding: BfPartial},
			1800,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := Calculate(tc.p)
			if plan.TargetKcal < tc.minFloor {
				t.Errorf("target %.0f is below the floor of %.0f", plan.TargetKcal, tc.minFloor)
			}
			if plan.TargetKcal < plan.BMR {
				t.Errorf("target %.0f is below BMR %.0f", plan.TargetKcal, plan.BMR)
			}
		})
	}
}

func TestBreastfeedingDeficitIsCapped(t *testing.T) {
	// A large woman with a big TDEE would otherwise get a 750 kcal deficit.
	p := Profile{
		Sex: SexFemale, Age: 30, HeightCm: 178, WeightKg: 105,
		Activity: ActivityActive, Goal: GoalLose, Breastfeeding: BfExclusive,
	}
	plan := Calculate(p)
	if plan.DeficitKcal > 305 {
		t.Errorf("deficit %.0f exceeds the 300 kcal lactation cap", plan.DeficitKcal)
	}
	if plan.BreastfeedingAdd != 500 {
		t.Errorf("breastfeeding surcharge = %.0f, want 500", plan.BreastfeedingAdd)
	}
	if !hasNote(plan, "bf_deficit_capped") {
		t.Error("expected a bf_deficit_capped note")
	}
	if !hasNote(plan, "bf_active") {
		t.Error("expected a bf_active note")
	}
}

func TestBreastfeedingAddsEnergy(t *testing.T) {
	base := Profile{Sex: SexFemale, Age: 30, HeightCm: 165, WeightKg: 68,
		Activity: ActivityLight, Goal: GoalMaintain}
	nursing := base
	nursing.Breastfeeding = BfExclusive

	if diff := TDEE(nursing) - TDEE(base); math.Abs(diff-500) > 0.01 {
		t.Errorf("lactation surcharge in TDEE = %.1f, want 500", diff)
	}
	if Calculate(nursing).TargetKcal <= Calculate(base).TargetKcal {
		t.Error("breastfeeding target should exceed the non-breastfeeding target")
	}
}

func TestMuscleGoalFallsBackToRecompWhenHeavy(t *testing.T) {
	p := Profile{Sex: SexMale, Age: 35, HeightCm: 175, WeightKg: 95,
		Activity: ActivityModerate, Goal: GoalMuscle}
	plan := Calculate(p)
	if plan.TargetKcal > plan.TDEE {
		t.Errorf("target %.0f exceeds TDEE %.0f despite a BMI of %.1f",
			plan.TargetKcal, plan.TDEE, plan.BMI)
	}
	if !hasNote(plan, "recomp_suggested") {
		t.Error("expected a recomp_suggested note")
	}
}

func TestMuscleGoalAddsSurplusWhenLean(t *testing.T) {
	p := Profile{Sex: SexMale, Age: 28, HeightCm: 180, WeightKg: 72,
		Activity: ActivityModerate, Goal: GoalMuscle}
	plan := Calculate(p)
	if plan.TargetKcal <= plan.TDEE {
		t.Errorf("target %.0f should exceed TDEE %.0f for a lean muscle goal",
			plan.TargetKcal, plan.TDEE)
	}
	if surplus := plan.TargetKcal - plan.TDEE; surplus > 410 {
		t.Errorf("surplus %.0f exceeds the 400 kcal cap", surplus)
	}
}

func TestMacrosAddUpToTarget(t *testing.T) {
	profiles := []Profile{
		{Sex: SexFemale, Age: 32, HeightCm: 168, WeightKg: 78, Activity: ActivityLight, Goal: GoalLose},
		{Sex: SexMale, Age: 40, HeightCm: 183, WeightKg: 95, Activity: ActivityModerate, Goal: GoalLose},
		{Sex: SexFemale, Age: 29, HeightCm: 162, WeightKg: 70, Activity: ActivityLight,
			Goal: GoalLose, Breastfeeding: BfExclusive},
		{Sex: SexDivers, Age: 35, HeightCm: 172, WeightKg: 68, Activity: ActivityActive, Goal: GoalMaintain},
	}
	for _, p := range profiles {
		plan := Calculate(p)
		got := KcalFromMacros(plan.ProteinG, plan.CarbsG, plan.FatG)
		// Rounding each macro to whole grams costs a few kcal; more than 3 %
		// drift would mean the macro split itself is wrong.
		if rel := math.Abs(got-plan.TargetKcal) / plan.TargetKcal; rel > 0.03 {
			t.Errorf("macros give %.0f kcal but target is %.0f (%.1f%% off)",
				got, plan.TargetKcal, rel*100)
		}
		if plan.ProteinG <= 0 || plan.CarbsG <= 0 || plan.FatG <= 0 {
			t.Errorf("non-positive macro in plan: %+v", plan)
		}
	}
}

func TestBreastfeedingKeepsCarbsUp(t *testing.T) {
	p := Profile{Sex: SexFemale, Age: 31, HeightCm: 160, WeightKg: 62,
		Activity: ActivitySedentary, Goal: GoalLose, Breastfeeding: BfExclusive}
	plan := Calculate(p)
	if plan.CarbsG < 160 {
		t.Errorf("carbs %.0f g below the 160 g lactation minimum", plan.CarbsG)
	}
	if plan.WaterMl < p.WeightKg*35+700 {
		t.Errorf("water target %.0f ml missing the lactation allowance", plan.WaterMl)
	}
}

func TestRecommendedTargetWeight(t *testing.T) {
	// Above the range: aim for BMI 23.
	heavy := Profile{HeightCm: 170, WeightKg: 95}
	got := RecommendedTargetWeight(heavy)
	if bmi := BMI(got, 170); math.Abs(bmi-23) > 0.2 {
		t.Errorf("target %.1f kg gives BMI %.1f, want ~23", got, bmi)
	}
	// Already healthy: stay put.
	fine := Profile{HeightCm: 170, WeightKg: 65}
	if got := RecommendedTargetWeight(fine); math.Abs(got-65) > 0.1 {
		t.Errorf("healthy weight target = %.1f, want 65", got)
	}
	// Below the range: come up to the lower edge.
	light := Profile{HeightCm: 170, WeightKg: 48}
	low, _ := HealthyWeightRange(170)
	if got := RecommendedTargetWeight(light); math.Abs(got-low) > 0.1 {
		t.Errorf("underweight target = %.1f, want %.1f", got, low)
	}
}

func TestUnderweightLoseGoalWarns(t *testing.T) {
	p := Profile{Sex: SexFemale, Age: 25, HeightCm: 172, WeightKg: 50,
		Activity: ActivityLight, Goal: GoalLose}
	if !hasNote(Calculate(p), "underweight_warning") {
		t.Error("expected an underweight_warning note")
	}
}

func TestMilestonesProgressTowardsGoal(t *testing.T) {
	ms := Milestones(90, 90, 75, 175)
	if len(ms) < 2 {
		t.Fatalf("expected several milestones, got %d", len(ms))
	}
	last := ms[len(ms)-1]
	if !last.IsGoal || math.Abs(last.WeightKg-75) > 0.05 {
		t.Errorf("last milestone = %+v, want the 75 kg goal", last)
	}
	// Weights must descend monotonically and stay inside the journey.
	for i := 1; i < len(ms); i++ {
		if ms[i].WeightKg >= ms[i-1].WeightKg {
			t.Errorf("milestone %d (%.1f) is not below milestone %d (%.1f)",
				i, ms[i].WeightKg, i-1, ms[i-1].WeightKg)
		}
		if ms[i].WeightKg < 75-0.05 || ms[i].WeightKg > 90 {
			t.Errorf("milestone %.1f lies outside the 75–90 kg journey", ms[i].WeightKg)
		}
	}
	// None reached yet at the starting weight.
	for _, m := range ms {
		if m.Reached {
			t.Errorf("milestone %.1f marked reached at the start weight", m.WeightKg)
		}
	}
}

func TestMilestonesMarkReachedOnes(t *testing.T) {
	ms := Milestones(90, 80, 75, 175)
	for _, m := range ms {
		want := m.WeightKg >= 80
		if m.Reached != want {
			t.Errorf("milestone %.1f reached=%v, want %v (current 80)", m.WeightKg, m.Reached, want)
		}
	}
}

func TestMilestonesHoldingWeight(t *testing.T) {
	ms := Milestones(70, 70, 70, 175)
	if len(ms) != 1 || !ms[0].IsGoal || !ms[0].Reached {
		t.Errorf("holding weight should yield one reached goal, got %+v", ms)
	}
}

func TestMilestonesGainDirection(t *testing.T) {
	ms := Milestones(60, 60, 68, 175)
	for i := 1; i < len(ms); i++ {
		if ms[i].WeightKg <= ms[i-1].WeightKg {
			t.Errorf("gain milestones must ascend: %.1f then %.1f", ms[i-1].WeightKg, ms[i].WeightKg)
		}
	}
	if last := ms[len(ms)-1]; !last.IsGoal || math.Abs(last.WeightKg-68) > 0.05 {
		t.Errorf("last milestone = %+v, want the 68 kg goal", last)
	}
}

func TestActivityPALOrdering(t *testing.T) {
	levels := []Activity{ActivitySedentary, ActivityLight, ActivityModerate, ActivityActive, ActivityVeryHigh}
	for i := 1; i < len(levels); i++ {
		if levels[i].PAL() <= levels[i-1].PAL() {
			t.Errorf("PAL not increasing: %s=%v then %s=%v",
				levels[i-1], levels[i-1].PAL(), levels[i], levels[i].PAL())
		}
	}
	if unknown := Activity("nonsense").PAL(); unknown != 1.375 {
		t.Errorf("unknown activity PAL = %v, want the light default 1.375", unknown)
	}
}

func hasNote(p Plan, code string) bool {
	for _, n := range p.Notes {
		if n.Code == code {
			return true
		}
	}
	return false
}

func TestProjectionIsPessimistic(t *testing.T) {
	// Losing: start 90, now 84, goal 75, height 175. At -0.5 kg/week the honest
	// time to the goal is 18 weeks; the pessimistic estimate must exceed it.
	pr := Project(90, 84, 75, 175, -0.5)
	if !pr.Moving {
		t.Fatal("expected Moving=true with a real weekly change")
	}
	honest := int(math.Ceil((84 - 75) / 0.5))
	if pr.WeeksToGoal <= honest {
		t.Errorf("weeks-to-goal %d should exceed the honest %d (pessimism)", pr.WeeksToGoal, honest)
	}
	// The next milestone is nearer than the goal, so it must not take longer.
	if pr.WeeksToNextMilestone > pr.WeeksToGoal {
		t.Errorf("next milestone %d further than goal %d", pr.WeeksToNextMilestone, pr.WeeksToGoal)
	}
	if pr.NextMilestoneKg <= 75 || pr.NextMilestoneKg >= 84 {
		t.Errorf("next milestone %.1f should lie between current and goal", pr.NextMilestoneKg)
	}
	if pr.CurrentBMI < 27 || pr.CurrentBMI > 28 {
		t.Errorf("current BMI %.1f off (expected ~27.4)", pr.CurrentBMI)
	}
}

func TestProjectionAtMaintenanceDoesNotForecast(t *testing.T) {
	pr := Project(75, 75, 75, 175, 0)
	if pr.Moving {
		t.Error("no weekly change should mean Moving=false")
	}
	if pr.WeeksToGoal != 0 || pr.WeeksToNextMilestone != 0 {
		t.Errorf("expected zero weeks at maintenance, got goal=%d next=%d", pr.WeeksToGoal, pr.WeeksToNextMilestone)
	}
}
