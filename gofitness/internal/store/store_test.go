package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureUser(ctx, "u1", "Test"); err != nil {
		t.Fatalf("EnsureUser: %v", err)
	}
	return st, ctx
}

func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "migrate.db")

	st, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	st.Close()

	// Re-opening must not re-apply anything or fail.
	st2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer st2.Close()

	var version int
	if err := st2.DB().QueryRowContext(ctx,
		`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != len(migrations) {
		t.Errorf("schema at version %d, want %d", version, len(migrations))
	}
}

func TestProfileRoundTrip(t *testing.T) {
	st, ctx := newTestStore(t)

	if _, err := st.GetProfile(ctx, "u1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for a fresh user, got %v", err)
	}

	in := Profile{
		UserID:         "u1",
		DisplayName:    "Anna",
		Sex:            "female",
		BirthDate:      "1992-04-17",
		HeightCm:       168,
		StartWeightKg:  78.5,
		TargetWeightKg: 65,
		Activity:       "light",
		Goal:           "lose",
		Breastfeeding:  "exclusive",
		Prefs: Prefs{
			FishPolicy: "breaded_only", MaxFishPerWeek: 1, VeggieLevel: "low",
			HouseholdSize: 2, MealsPerDay: 4, MaxCookMinutes: 45,
			Language: "de", CookOnceEatTwice: true,
			ExcludeIngredients: []string{"Rosinen"},
		},
		SetupDone: true,
	}
	if err := st.SaveProfile(ctx, in); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	got, err := st.GetProfile(ctx, "u1")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.DisplayName != in.DisplayName || got.HeightCm != in.HeightCm ||
		got.Breastfeeding != in.Breastfeeding || !got.SetupDone {
		t.Errorf("profile round trip mismatch:\n got %+v\nwant %+v", got, in)
	}
	if got.Prefs.FishPolicy != "breaded_only" || got.Prefs.HouseholdSize != 2 ||
		len(got.Prefs.ExcludeIngredients) != 1 {
		t.Errorf("prefs not round-tripped: %+v", got.Prefs)
	}

	// Saving again must update, not duplicate.
	in.DisplayName = "Anna B."
	if err := st.SaveProfile(ctx, in); err != nil {
		t.Fatalf("second SaveProfile: %v", err)
	}
	got, _ = st.GetProfile(ctx, "u1")
	if got.DisplayName != "Anna B." {
		t.Errorf("update did not apply: %q", got.DisplayName)
	}
}

func TestProfileAge(t *testing.T) {
	now := time.Now()
	born := now.AddDate(-30, 0, -1) // definitely 30 already
	p := Profile{BirthDate: born.Format("2006-01-02")}
	if age := p.Age(); age != 30 {
		t.Errorf("Age = %d, want 30", age)
	}

	// Missing or absurd dates fall back to a neutral 30.
	for _, bad := range []string{"", "not-a-date", "1700-01-01"} {
		if age := (Profile{BirthDate: bad}).Age(); age != 30 {
			t.Errorf("Age for %q = %d, want the fallback 30", bad, age)
		}
	}
}

func TestWeightsAreScopedToUser(t *testing.T) {
	st, ctx := newTestStore(t)
	if err := st.EnsureUser(ctx, "u2", "Other"); err != nil {
		t.Fatal(err)
	}

	if _, err := st.AddWeight(ctx, "u1", WeightEntry{WeightKg: 80}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddWeight(ctx, "u2", WeightEntry{WeightKg: 60}); err != nil {
		t.Fatal(err)
	}

	list, err := st.Weights(ctx, "u1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].WeightKg != 80 {
		t.Errorf("u1 sees %+v, expected only its own 80 kg entry", list)
	}

	// One user may not delete another's data.
	if err := st.DeleteWeight(ctx, "u1", 2); err != nil {
		t.Fatalf("DeleteWeight: %v", err)
	}
	other, _ := st.Weights(ctx, "u2", 10)
	if len(other) != 1 {
		t.Error("a cross-user delete removed another user's weigh-in")
	}
}

func TestWeightsNewestFirst(t *testing.T) {
	st, ctx := newTestStore(t)
	base := time.Now().UTC()
	for i, kg := range []float64{80, 79, 78} {
		_, err := st.AddWeight(ctx, "u1", WeightEntry{
			WeightKg:   kg,
			RecordedAt: base.AddDate(0, 0, -i*3).Format(time.RFC3339),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	list, _ := st.Weights(ctx, "u1", 10)
	if len(list) != 3 || list[0].WeightKg != 80 {
		t.Errorf("expected newest (80 kg, today) first, got %+v", list)
	}

	latest, err := st.LatestWeight(ctx, "u1")
	if err != nil || latest.WeightKg != 80 {
		t.Errorf("LatestWeight = %+v, %v", latest, err)
	}
}

func TestLatestWeightEmpty(t *testing.T) {
	st, ctx := newTestStore(t)
	if _, err := st.LatestWeight(ctx, "u1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBodyFatOptional(t *testing.T) {
	st, ctx := newTestStore(t)
	bf := 24.5
	if _, err := st.AddWeight(ctx, "u1", WeightEntry{WeightKg: 70, BodyFatPct: &bf}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddWeight(ctx, "u1", WeightEntry{WeightKg: 71}); err != nil {
		t.Fatal(err)
	}
	list, _ := st.Weights(ctx, "u1", 10)
	var withFat, withoutFat int
	for _, e := range list {
		if e.BodyFatPct != nil {
			withFat++
			if *e.BodyFatPct != 24.5 {
				t.Errorf("body fat = %v", *e.BodyFatPct)
			}
		} else {
			withoutFat++
		}
	}
	if withFat != 1 || withoutFat != 1 {
		t.Errorf("expected one entry with and one without body fat, got %d/%d", withFat, withoutFat)
	}
}

func TestFoodTotals(t *testing.T) {
	st, ctx := newTestStore(t)
	day := Day(time.Now())

	entries := []FoodEntry{
		{Day: day, Name: "Brot", Kcal: 200, ProteinG: 7, CarbsG: 38, FatG: 2, MealType: "breakfast"},
		{Day: day, Name: "Eis", Kcal: 250, ProteinG: 4, CarbsG: 30, FatG: 12, MealType: "snack"},
	}
	for _, e := range entries {
		if _, err := st.AddFood(ctx, "u1", e); err != nil {
			t.Fatal(err)
		}
	}

	totals, err := st.TotalsForDay(ctx, "u1", day)
	if err != nil {
		t.Fatal(err)
	}
	if totals.Kcal != 450 || totals.ProteinG != 11 || totals.Entries != 2 {
		t.Errorf("totals = %+v, want 450 kcal / 11 g protein / 2 entries", totals)
	}

	// A different day must be empty.
	other, _ := st.TotalsForDay(ctx, "u1", "2000-01-01")
	if other.Kcal != 0 || other.Entries != 0 {
		t.Errorf("unrelated day has data: %+v", other)
	}
}

func TestWorkoutsCountTowardsTheDay(t *testing.T) {
	st, ctx := newTestStore(t)
	day := Day(time.Now())

	if _, err := st.AddWorkout(ctx, "u1", Workout{
		Day: day, Kind: "run", Minutes: 30, Kcal: 300,
	}); err != nil {
		t.Fatal(err)
	}

	totals, _ := st.TotalsForDay(ctx, "u1", day)
	if totals.WorkoutKcal != 300 || totals.WorkoutMin != 30 {
		t.Errorf("workout totals = %+v", totals)
	}
}

// Repeated tracker syncs must overwrite, not accumulate.
func TestUpsertTrackerWorkoutReplaces(t *testing.T) {
	st, ctx := newTestStore(t)
	day := Day(time.Now())

	for _, steps := range []int{4000, 9000, 12000} {
		if err := st.UpsertTrackerWorkout(ctx, "u1", Workout{
			Day: day, Kind: "tracker", Steps: steps, Kcal: float64(steps) / 20, Source: "tracker",
		}); err != nil {
			t.Fatal(err)
		}
	}

	totals, _ := st.TotalsForDay(ctx, "u1", day)
	if totals.Steps != 12000 {
		t.Errorf("steps = %d, want the last synced value 12000", totals.Steps)
	}
	if totals.WorkoutKcal != 600 {
		t.Errorf("tracker kcal = %v, want 600 (not accumulated)", totals.WorkoutKcal)
	}

	// A manual workout on the same day must survive the sync.
	if _, err := st.AddWorkout(ctx, "u1", Workout{Day: day, Kind: "run", Kcal: 250}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertTrackerWorkout(ctx, "u1", Workout{
		Day: day, Kind: "tracker", Steps: 13000, Kcal: 650, Source: "tracker",
	}); err != nil {
		t.Fatal(err)
	}
	totals, _ = st.TotalsForDay(ctx, "u1", day)
	if totals.WorkoutKcal != 900 {
		t.Errorf("kcal = %v, want 650 tracker + 250 manual", totals.WorkoutKcal)
	}
}

func TestActiveDaysUnionsAllSources(t *testing.T) {
	st, ctx := newTestStore(t)
	now := time.Now()
	d0 := Day(now)
	d1 := Day(now.AddDate(0, 0, -1))
	d2 := Day(now.AddDate(0, 0, -2))

	if _, err := st.AddFood(ctx, "u1", FoodEntry{Day: d0, Name: "x", Kcal: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddWorkout(ctx, "u1", Workout{Day: d1, Kind: "run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddWeight(ctx, "u1", WeightEntry{
		WeightKg: 70, RecordedAt: now.AddDate(0, 0, -2).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}

	days, err := st.ActiveDays(ctx, "u1", Day(now.AddDate(0, 0, -7)), d0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{d0, d1, d2} {
		if !days[d] {
			t.Errorf("%s should count as an active day", d)
		}
	}
	if len(days) != 3 {
		t.Errorf("got %d active days, want 3: %v", len(days), days)
	}
}

func TestPlanRoundTripAndReplace(t *testing.T) {
	st, ctx := newTestStore(t)
	week := "2026-08-17"

	entries := []PlanEntry{
		{DayIndex: 0, MealType: "breakfast", RecipeID: "porridge-apfel-zimt", Servings: 1},
		{DayIndex: 0, MealType: "dinner", RecipeID: "frikadellen-kartoffelpueree", Servings: 1.25},
	}
	items := []ShoppingItem{
		{Name: "Haferflocken", Amount: 100, Unit: "g", Category: "Trockenwaren"},
	}

	planID, err := st.SavePlan(ctx, "u1", week, entries, items)
	if err != nil {
		t.Fatalf("SavePlan: %v", err)
	}

	got, err := st.PlanEntries(ctx, planID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].RecipeID != "porridge-apfel-zimt" || got[1].Servings != 1.25 {
		t.Errorf("plan entries = %+v", got)
	}

	shopping, err := st.ShoppingItems(ctx, planID)
	if err != nil {
		t.Fatal(err)
	}
	if len(shopping) != 1 || shopping[0].Name != "Haferflocken" {
		t.Errorf("shopping items = %+v", shopping)
	}

	// Re-planning the same week replaces everything, leaving no orphans.
	newID, err := st.SavePlan(ctx, "u1", week, entries[:1], items)
	if err != nil {
		t.Fatal(err)
	}
	if newID == planID {
		t.Error("re-planning should create a new plan row")
	}
	if again, _ := st.PlanEntries(ctx, newID); len(again) != 1 {
		t.Errorf("expected 1 entry after replace, got %d", len(again))
	}
	if orphans, _ := st.PlanEntries(ctx, planID); len(orphans) != 0 {
		t.Errorf("%d orphaned entries left from the old plan", len(orphans))
	}
}

func TestShoppingAndCookedOwnership(t *testing.T) {
	st, ctx := newTestStore(t)
	if err := st.EnsureUser(ctx, "u2", "Other"); err != nil {
		t.Fatal(err)
	}

	planID, err := st.SavePlan(ctx, "u1", "2026-08-17",
		[]PlanEntry{{DayIndex: 0, MealType: "dinner", RecipeID: "kaesespaetzle-leicht", Servings: 1}},
		[]ShoppingItem{{Name: "Spätzle", Amount: 800, Unit: "g"}})
	if err != nil {
		t.Fatal(err)
	}

	entries, _ := st.PlanEntries(ctx, planID)
	items, _ := st.ShoppingItems(ctx, planID)

	// The owner may change them.
	if err := st.SetEntryCooked(ctx, "u1", entries[0].ID, true); err != nil {
		t.Errorf("owner could not mark cooked: %v", err)
	}
	if err := st.SetShoppingChecked(ctx, "u1", items[0].ID, true); err != nil {
		t.Errorf("owner could not tick the item: %v", err)
	}

	// Another user may not.
	if err := st.SetEntryCooked(ctx, "u2", entries[0].ID, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-user cooked update returned %v, want ErrNotFound", err)
	}
	if err := st.SetShoppingChecked(ctx, "u2", items[0].ID, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-user shopping update returned %v, want ErrNotFound", err)
	}

	// The owner's changes stand.
	entries, _ = st.PlanEntries(ctx, planID)
	items, _ = st.ShoppingItems(ctx, planID)
	if !entries[0].Cooked || !items[0].Checked {
		t.Error("owner's updates were reverted")
	}
}

func TestAchievementsUnlockOnce(t *testing.T) {
	st, ctx := newTestStore(t)

	first, err := st.UnlockAchievement(ctx, "u1", "streak_7")
	if err != nil {
		t.Fatal(err)
	}
	if !first {
		t.Error("first unlock should report true")
	}

	second, err := st.UnlockAchievement(ctx, "u1", "streak_7")
	if err != nil {
		t.Fatal(err)
	}
	if second {
		t.Error("re-unlocking should report false")
	}

	all, _ := st.UnlockedAchievements(ctx, "u1")
	if len(all) != 1 || all["streak_7"] == "" {
		t.Errorf("unlocked = %v", all)
	}
}

func TestTrackerLinks(t *testing.T) {
	st, ctx := newTestStore(t)

	if err := st.SetTrackerLink(ctx, "u1", "steps", "sensor.watch_steps"); err != nil {
		t.Fatal(err)
	}
	links, _ := st.TrackerLinks(ctx, "u1")
	if links["steps"] != "sensor.watch_steps" {
		t.Errorf("links = %v", links)
	}

	// Re-linking replaces.
	if err := st.SetTrackerLink(ctx, "u1", "steps", "sensor.phone_steps"); err != nil {
		t.Fatal(err)
	}
	links, _ = st.TrackerLinks(ctx, "u1")
	if links["steps"] != "sensor.phone_steps" || len(links) != 1 {
		t.Errorf("links after replace = %v", links)
	}

	// An empty entity id unlinks.
	if err := st.SetTrackerLink(ctx, "u1", "steps", "  "); err != nil {
		t.Fatal(err)
	}
	links, _ = st.TrackerLinks(ctx, "u1")
	if len(links) != 0 {
		t.Errorf("expected no links after unlinking, got %v", links)
	}
}

func TestSettings(t *testing.T) {
	st, ctx := newTestStore(t)
	if got := st.Setting(ctx, "missing", "fallback"); got != "fallback" {
		t.Errorf("missing setting = %q", got)
	}
	if err := st.SetSetting(ctx, "k", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, "k", "v2"); err != nil {
		t.Fatal(err)
	}
	if got := st.Setting(ctx, "k", ""); got != "v2" {
		t.Errorf("setting = %q, want v2", got)
	}
}
