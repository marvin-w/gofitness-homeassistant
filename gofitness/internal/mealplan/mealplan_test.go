package mealplan

import (
	"strings"
	"testing"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/recipes"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

func defaultOpts() Options {
	return Options{
		WeekStart:  "2026-08-17",
		TargetKcal: 1900,
		Prefs:      store.DefaultPrefs(),
		Seed:       1,
	}
}

func TestGenerateFillsEveryDay(t *testing.T) {
	book := recipes.MustLoad()
	plan := Generate(book, defaultOpts())

	if len(plan.Days) != 7 {
		t.Fatalf("plan has %d days, want 7", len(plan.Days))
	}
	for _, d := range plan.Days {
		if len(d.Entries) != 4 {
			t.Errorf("%s has %d meals, want 4", d.Name, len(d.Entries))
		}
		if d.Kcal == 0 {
			t.Errorf("%s has no calories", d.Name)
		}
	}
}

// The plan is only useful if it actually lands near the calorie target.
func TestGenerateHitsTheCalorieTarget(t *testing.T) {
	book := recipes.MustLoad()
	for _, target := range []float64{1500, 1900, 2400, 2900} {
		opts := defaultOpts()
		opts.TargetKcal = target
		plan := Generate(book, opts)

		for _, d := range plan.Days {
			drift := (d.Kcal - target) / target
			if drift < -0.25 || drift > 0.25 {
				t.Errorf("target %.0f: %s came out at %.0f kcal (%.0f%% off)",
					target, d.Name, d.Kcal, drift*100)
			}
		}
		if plan.AvgKcal < target*0.85 || plan.AvgKcal > target*1.15 {
			t.Errorf("target %.0f: weekly average %.0f is too far off", target, plan.AvgKcal)
		}
	}
}

func TestThreeMealsPerDay(t *testing.T) {
	opts := defaultOpts()
	opts.Prefs.MealsPerDay = 3
	plan := Generate(recipes.MustLoad(), opts)
	for _, d := range plan.Days {
		if len(d.Entries) != 3 {
			t.Errorf("%s has %d meals, want 3", d.Name, len(d.Entries))
		}
	}
}

func TestFishCapIsRespected(t *testing.T) {
	opts := defaultOpts()
	opts.Prefs.MaxFishPerWeek = 1
	book := recipes.MustLoad()

	// Several seeds, because the cap must hold regardless of the shuffle.
	for seed := uint64(0); seed < 12; seed++ {
		opts.Seed = seed
		plan := Generate(book, opts)

		fish := 0
		for _, d := range plan.Days {
			for _, e := range d.Entries {
				if e.Leftover {
					continue
				}
				r, ok := book.Get(e.RecipeID)
				if ok && r.ContainsFish {
					fish++
				}
			}
		}
		if fish > opts.Prefs.MaxFishPerWeek {
			t.Errorf("seed %d: %d fish meals, cap is %d", seed, fish, opts.Prefs.MaxFishPerWeek)
		}
	}
}

func TestOnlyBreadedFishIsPlanned(t *testing.T) {
	book := recipes.MustLoad()
	opts := defaultOpts()
	opts.Prefs.FishPolicy = "breaded_only"

	for seed := uint64(0); seed < 8; seed++ {
		opts.Seed = seed
		for _, d := range Generate(book, opts).Days {
			for _, e := range d.Entries {
				r, ok := book.Get(e.RecipeID)
				if ok && r.ContainsFish && !r.FishBreaded {
					t.Fatalf("seed %d planned unbreaded fish: %s", seed, r.ID)
				}
			}
		}
	}
}

func TestVeggiePreferenceIsRespected(t *testing.T) {
	book := recipes.MustLoad()
	opts := defaultOpts()
	opts.Prefs.VeggieLevel = "low"

	for _, d := range Generate(book, opts).Days {
		for _, e := range d.Entries {
			r, ok := book.Get(e.RecipeID)
			if ok && r.VeggieRank() > 0 {
				t.Errorf("%s is veggie level %q despite a low preference", r.ID, r.VeggieLevel)
			}
		}
	}
}

func TestBreastfeedingSafeOnly(t *testing.T) {
	book := recipes.MustLoad()
	opts := defaultOpts()
	opts.BreastfeedingSafe = true

	for _, d := range Generate(book, opts).Days {
		for _, e := range d.Entries {
			r, ok := book.Get(e.RecipeID)
			if ok && !r.BreastfeedingSafe {
				t.Errorf("%s is not breastfeeding-safe", r.ID)
			}
		}
	}
}

func TestPlanIsVaried(t *testing.T) {
	plan := Generate(recipes.MustLoad(), defaultOpts())

	counts := map[string]int{}
	for _, d := range plan.Days {
		for _, e := range d.Entries {
			if e.Leftover {
				continue // leftovers are intentional repeats
			}
			counts[e.RecipeID]++
		}
	}
	for id, n := range counts {
		if n > 3 {
			t.Errorf("%s appears %d times in one week", id, n)
		}
	}
	if len(counts) < 10 {
		t.Errorf("only %d distinct dishes in a week", len(counts))
	}
}

func TestSameSeedIsReproducible(t *testing.T) {
	book := recipes.MustLoad()
	a := Generate(book, defaultOpts())
	b := Generate(book, defaultOpts())

	for i := range a.Days {
		for j := range a.Days[i].Entries {
			if a.Days[i].Entries[j].RecipeID != b.Days[i].Entries[j].RecipeID {
				t.Fatal("the same seed produced a different plan")
			}
		}
	}
}

func TestDifferentSeedReshuffles(t *testing.T) {
	book := recipes.MustLoad()
	a := Generate(book, defaultOpts())
	optsB := defaultOpts()
	optsB.Seed = 999
	b := Generate(book, optsB)

	same := 0
	total := 0
	for i := range a.Days {
		for j := range a.Days[i].Entries {
			total++
			if a.Days[i].Entries[j].RecipeID == b.Days[i].Entries[j].RecipeID {
				same++
			}
		}
	}
	if same == total {
		t.Error("a different seed produced an identical plan")
	}
}

func TestCookOnceEatTwice(t *testing.T) {
	opts := defaultOpts()
	opts.CookOnceEatTwice = true
	plan := Generate(recipes.MustLoad(), opts)

	leftovers := 0
	for _, d := range plan.Days {
		for _, e := range d.Entries {
			if !e.Leftover {
				continue
			}
			leftovers++
			if e.MealType != "lunch" {
				t.Errorf("leftover planned as %s, expected lunch", e.MealType)
			}
			// The dish must have been cooked the evening before.
			prev := plan.Days[d.Index-1]
			found := false
			for _, pe := range prev.Entries {
				if pe.MealType == "dinner" && pe.RecipeID == e.RecipeID {
					found = true
				}
			}
			if !found {
				t.Errorf("leftover %s on day %d has no matching dinner the day before",
					e.RecipeID, d.Index)
			}
		}
	}
	if leftovers == 0 {
		t.Error("cook-once-eat-twice produced no leftovers at all")
	}
}

func TestShoppingListAggregates(t *testing.T) {
	book := recipes.MustLoad()
	plan := Generate(book, defaultOpts())
	list := ShoppingList(book, plan, 2, "de")

	if len(list) == 0 {
		t.Fatal("empty shopping list")
	}

	seen := map[string]bool{}
	for _, item := range list {
		key := strings.ToLower(item.Name) + "|" + item.Unit
		if seen[key] {
			t.Errorf("duplicate line for %s (%s)", item.Name, item.Unit)
		}
		seen[key] = true

		if item.Amount <= 0 {
			t.Errorf("%s has a non-positive amount", item.Name)
		}
		if item.Category == "" {
			t.Errorf("%s has no category", item.Name)
		}
	}
}

func TestShoppingListScalesWithHousehold(t *testing.T) {
	book := recipes.MustLoad()
	plan := Generate(book, defaultOpts())

	one := totalAmount(ShoppingList(book, plan, 1, "de"))
	four := totalAmount(ShoppingList(book, plan, 4, "de"))

	if four <= one*2 {
		t.Errorf("four people (%.0f) should need far more than one (%.0f)", four, one)
	}
}

func totalAmount(items []ShoppingItem) float64 {
	var sum float64
	for _, i := range items {
		if i.Unit == "g" || i.Unit == "ml" {
			sum += i.Amount
		}
	}
	return sum
}

func TestShoppingListSkipsLeftovers(t *testing.T) {
	book := recipes.MustLoad()

	withLeftovers := defaultOpts()
	withLeftovers.CookOnceEatTwice = true
	planA := Generate(book, withLeftovers)

	// Leftovers must not be bought twice: the same dish eaten a second time
	// adds nothing to the list.
	list := ShoppingList(book, planA, 2, "de")
	if len(list) == 0 {
		t.Fatal("empty shopping list")
	}

	countLeftovers := 0
	for _, d := range planA.Days {
		for _, e := range d.Entries {
			if e.Leftover {
				countLeftovers++
			}
		}
	}
	if countLeftovers == 0 {
		t.Skip("no leftovers generated for this seed")
	}

	// Removing the leftover flag would inflate the list; verify it does.
	inflated := planA
	for i := range inflated.Days {
		for j := range inflated.Days[i].Entries {
			inflated.Days[i].Entries[j].Leftover = false
		}
	}
	if totalAmount(ShoppingList(book, inflated, 2, "de")) <= totalAmount(list) {
		t.Error("leftovers are being added to the shopping list")
	}
}

func TestShoppingListLocalises(t *testing.T) {
	book := recipes.MustLoad()
	plan := Generate(book, defaultOpts())

	de := ShoppingList(book, plan, 2, "de")
	en := ShoppingList(book, plan, 2, "en")

	if len(de) != len(en) {
		t.Fatalf("language changed the list length: %d vs %d", len(de), len(en))
	}
	// The aggregation key is the German name, so switching language must not
	// merge or split any line.
	deNames := map[string]bool{}
	for _, i := range de {
		deNames[i.Name] = true
	}
	differing := 0
	for _, i := range en {
		if !deNames[i.Name] {
			differing++
		}
	}
	if differing == 0 {
		t.Error("English shopping list was not translated at all")
	}
	for _, i := range en {
		if i.Category == "Obst & Gemüse" {
			t.Error("English list still has a German category label")
		}
	}
}

func TestWeekdaysLocalised(t *testing.T) {
	if Weekdays("de")[0] != "Montag" {
		t.Errorf("German day 0 = %q", Weekdays("de")[0])
	}
	if Weekdays("en")[0] != "Monday" {
		t.Errorf("English day 0 = %q", Weekdays("en")[0])
	}
	if len(Weekdays("de")) != 7 || len(Weekdays("en")) != 7 {
		t.Error("a week must have seven days")
	}
}

func TestPlanTitlesFollowLanguage(t *testing.T) {
	book := recipes.MustLoad()
	opts := defaultOpts()
	opts.Lang = "en"
	en := Generate(book, opts)

	opts.Lang = "de"
	de := Generate(book, opts)

	if en.Days[0].Name != "Monday" || de.Days[0].Name != "Montag" {
		t.Errorf("day names not localised: %q / %q", en.Days[0].Name, de.Days[0].Name)
	}
	// Same seed and preferences, so the dishes must match — only the text differs.
	if en.Days[0].Entries[0].RecipeID != de.Days[0].Entries[0].RecipeID {
		t.Error("language changed which recipes were chosen")
	}
	if en.Days[0].Entries[0].Title == de.Days[0].Entries[0].Title {
		t.Error("recipe title was not localised")
	}
}

func TestPortionsStayReasonable(t *testing.T) {
	for _, target := range []float64{1400, 2000, 3000} {
		opts := defaultOpts()
		opts.TargetKcal = target
		for _, d := range Generate(recipes.MustLoad(), opts).Days {
			for _, e := range d.Entries {
				if e.Portions < 0.5 || e.Portions > 2 {
					t.Errorf("target %.0f: %s planned at %.2f portions", target, e.RecipeID, e.Portions)
				}
			}
		}
	}
}

func TestNoteCodesAreStable(t *testing.T) {
	opts := defaultOpts()
	opts.Prefs.FishPolicy = "none"
	plan := Generate(recipes.MustLoad(), opts)
	for _, n := range plan.Notes {
		if n.Code == "" {
			t.Error("note without a code — the interface cannot translate it")
		}
		if strings.ContainsAny(n.Code, " äöüß") {
			t.Errorf("note code %q looks like display text, not a code", n.Code)
		}
	}
}
