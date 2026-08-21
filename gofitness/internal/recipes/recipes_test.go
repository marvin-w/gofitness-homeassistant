package recipes

import (
	"math"
	"strings"
	"testing"
)

func TestLoadRecipes(t *testing.T) {
	b, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if b.Len() < 30 {
		t.Errorf("only %d recipes loaded, expected a usable library", b.Len())
	}
}

// Every recipe must be complete enough to shop for and cook.
func TestRecipesAreComplete(t *testing.T) {
	b := MustLoad()
	for _, r := range b.All() {
		if r.ID == "" || r.Title == "" || r.Description == "" {
			t.Errorf("%s: missing id, title or description", r.ID)
		}
		if len(r.Ingredients) == 0 {
			t.Errorf("%s: no ingredients", r.ID)
		}
		if len(r.Steps) == 0 {
			t.Errorf("%s: no steps", r.ID)
		}
		if len(r.MealTypes) == 0 {
			t.Errorf("%s: no meal types", r.ID)
		}
		if r.Servings <= 0 {
			t.Errorf("%s: servings must be positive", r.ID)
		}
		if r.PrepMinutes <= 0 {
			t.Errorf("%s: prep time must be positive", r.ID)
		}
		if r.Kcal <= 0 {
			t.Errorf("%s: kcal must be positive", r.ID)
		}
		if !strings.HasPrefix(r.URL, "https://") {
			t.Errorf("%s: url %q is not an https link", r.ID, r.URL)
		}
		for _, ing := range r.Ingredients {
			if ing.Name == "" {
				t.Errorf("%s: ingredient with no name", r.ID)
			}
			if ing.Amount <= 0 {
				t.Errorf("%s: ingredient %q has a non-positive amount", r.ID, ing.Name)
			}
			if ing.Category == "" {
				t.Errorf("%s: ingredient %q has no shopping category", r.ID, ing.Name)
			}
		}
		for _, mt := range r.MealTypes {
			switch mt {
			case "breakfast", "lunch", "dinner", "snack":
			default:
				t.Errorf("%s: unknown meal type %q", r.ID, mt)
			}
		}
		switch r.VeggieLevel {
		case "low", "medium", "high":
		default:
			t.Errorf("%s: unknown veggie level %q", r.ID, r.VeggieLevel)
		}
	}
}

// The declared calories must match the declared macros, or the meal planner
// will hit a calorie target while missing the protein target entirely.
func TestRecipeMacrosMatchCalories(t *testing.T) {
	for _, r := range MustLoad().All() {
		fromMacros := r.ProteinG*4 + r.CarbsG*4 + r.FatG*9
		if r.Kcal == 0 {
			continue
		}
		if rel := math.Abs(fromMacros-r.Kcal) / r.Kcal; rel > 0.12 {
			t.Errorf("%s: %.0f kcal declared but macros give %.0f (%.1f%% off)",
				r.ID, r.Kcal, fromMacros, rel*100)
		}
	}
}

// The household this was built for eats fish only when it is breaded.
func TestFishIsAlwaysBreaded(t *testing.T) {
	for _, r := range MustLoad().All() {
		if r.ContainsFish && !r.FishBreaded {
			t.Errorf("%s: contains unbreaded fish", r.ID)
		}
	}
}

func TestEveryMealSlotHasEnoughChoices(t *testing.T) {
	b := MustLoad()
	// The default household: little veg, breaded fish only.
	base := Filter{MaxVeggieRank: RankFor("low"), FishPolicy: "breaded_only"}
	for _, slot := range []string{"breakfast", "lunch", "dinner", "snack"} {
		f := base
		f.MealType = slot
		if n := len(b.Select(f)); n < 4 {
			t.Errorf("only %d recipes for %s under the default preferences", n, slot)
		}
	}
}

func TestBreastfeedingFilterLeavesAFullWeek(t *testing.T) {
	b := MustLoad()
	f := Filter{
		MaxVeggieRank:     RankFor("low"),
		FishPolicy:        "breaded_only",
		BreastfeedingSafe: true,
	}
	for _, slot := range []string{"breakfast", "lunch", "dinner", "snack"} {
		f.MealType = slot
		if n := len(b.Select(f)); n < 4 {
			t.Errorf("only %d breastfeeding-safe recipes for %s", n, slot)
		}
	}
}

func TestFishPolicyFilters(t *testing.T) {
	b := MustLoad()
	none := b.Select(Filter{FishPolicy: "none", MaxVeggieRank: 2})
	for _, r := range none {
		if r.ContainsFish {
			t.Errorf("%s survived the no-fish filter", r.ID)
		}
	}
	any := b.Select(Filter{FishPolicy: "any", MaxVeggieRank: 2})
	if len(any) <= len(none) {
		t.Error("allowing fish should not shrink the candidate pool")
	}
}

func TestVeggieLevelFilters(t *testing.T) {
	b := MustLoad()
	low := b.Select(Filter{MaxVeggieRank: RankFor("low"), FishPolicy: "any"})
	for _, r := range low {
		if r.VeggieRank() > 0 {
			t.Errorf("%s (veggie %q) survived the low-veg filter", r.ID, r.VeggieLevel)
		}
	}
	high := b.Select(Filter{MaxVeggieRank: RankFor("high"), FishPolicy: "any"})
	if len(high) < len(low) {
		t.Error("a higher veg tolerance must not shrink the pool")
	}
}

func TestExcludeIngredientFilters(t *testing.T) {
	b := MustLoad()
	out := b.Select(Filter{
		MaxVeggieRank:     2,
		FishPolicy:        "any",
		ExcludeIngredient: []string{"Hähnchenbrustfilet"},
	})
	for _, r := range out {
		for _, ing := range r.Ingredients {
			if strings.Contains(strings.ToLower(ing.Name), "hähnchenbrustfilet") {
				t.Errorf("%s survived the ingredient exclusion", r.ID)
			}
		}
	}
}

func TestScaleIngredients(t *testing.T) {
	r, ok := MustLoad().Get("haehnchen-reispfanne")
	if !ok {
		t.Fatal("reference recipe missing")
	}
	doubled := r.ScaleIngredients(float64(r.Servings) * 2)
	if len(doubled) != len(r.Ingredients) {
		t.Fatalf("scaling changed the ingredient count")
	}
	for i, ing := range doubled {
		want := r.Ingredients[i].Amount * 2
		// Amounts are rounded to shoppable steps, so allow a little slack.
		if math.Abs(ing.Amount-want) > math.Max(want*0.05, 0.5) {
			t.Errorf("%s: doubled amount %.1f, want ~%.1f", ing.Name, ing.Amount, want)
		}
	}

	same := r.ScaleIngredients(float64(r.Servings))
	for i, ing := range same {
		if math.Abs(ing.Amount-r.Ingredients[i].Amount) > 0.5 {
			t.Errorf("%s: unscaled amount changed from %.1f to %.1f",
				ing.Name, r.Ingredients[i].Amount, ing.Amount)
		}
	}
}

// --------------------------------------------------------------------- i18n

func TestEveryRecipeHasEnglishText(t *testing.T) {
	if missing := MustLoad().MissingTranslations(); len(missing) > 0 {
		t.Errorf("recipes missing or mismatched English text: %v", missing)
	}
}

func TestEveryIngredientHasEnglishName(t *testing.T) {
	if missing := MustLoad().UntranslatedIngredients(); len(missing) > 0 {
		t.Errorf("ingredients with no English name: %v", missing)
	}
}

func TestLocalizeSwapsText(t *testing.T) {
	b := MustLoad()
	r, ok := b.Get("fischstaebchen-kartoffelpueree")
	if !ok {
		t.Fatal("reference recipe missing")
	}

	de := Localize(r, "de")
	if de.Title != r.Title {
		t.Error("German localisation must be a no-op")
	}

	en := Localize(r, "en")
	if en.Title == r.Title {
		t.Error("English title was not translated")
	}
	if len(en.Steps) != len(r.Steps) {
		t.Errorf("English step count %d != German %d", len(en.Steps), len(r.Steps))
	}
	if !strings.Contains(en.URL, "allrecipes.com") {
		t.Errorf("English recipe link %q should point at an English portal", en.URL)
	}
	// Numbers must survive translation untouched.
	if en.Kcal != r.Kcal || en.ProteinG != r.ProteinG {
		t.Error("localisation must not change nutrition values")
	}
	// Ingredient names and units get translated too.
	for i, ing := range en.Ingredients {
		if ing.Amount != r.Ingredients[i].Amount {
			t.Errorf("ingredient amount changed during localisation")
		}
	}
	if en.Ingredients[0].Name == r.Ingredients[0].Name {
		t.Errorf("ingredient %q was not translated", en.Ingredients[0].Name)
	}
}

func TestLocalizeDoesNotMutateTheBook(t *testing.T) {
	b := MustLoad()
	before, _ := b.Get("haehnchen-reispfanne")
	titleBefore := before.Title
	ingBefore := before.Ingredients[0].Name

	_ = Localize(before, "en")

	after, _ := b.Get("haehnchen-reispfanne")
	if after.Title != titleBefore || after.Ingredients[0].Name != ingBefore {
		t.Error("Localize mutated the stored recipe")
	}
}

func TestNormalizeLang(t *testing.T) {
	for input, want := range map[string]string{
		"en": LangEN, "EN": LangEN, "en-GB": LangEN, "english": LangEN,
		"de": LangDE, "de-DE": LangDE, "": LangDE, "fr": LangDE, "  De ": LangDE,
	} {
		if got := NormalizeLang(input); got != want {
			t.Errorf("NormalizeLang(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTranslateMealType(t *testing.T) {
	if got := TranslateMealType("breakfast", "de"); got != "Frühstück" {
		t.Errorf("German breakfast = %q", got)
	}
	if got := TranslateMealType("breakfast", "en"); got != "Breakfast" {
		t.Errorf("English breakfast = %q", got)
	}
	if got := TranslateMealType("brunch", "en"); got != "brunch" {
		t.Errorf("unknown slot should pass through, got %q", got)
	}
}

func TestUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range MustLoad().All() {
		if seen[r.ID] {
			t.Errorf("duplicate recipe id %q", r.ID)
		}
		seen[r.ID] = true
	}
}
