package ai

import "testing"

func TestFoodTableLoads(t *testing.T) {
	if n := len(Foods()); n < 50 {
		t.Errorf("only %d foods in the local table", n)
	}
	for _, f := range Foods() {
		if f.DE == "" || f.EN == "" {
			t.Errorf("food missing a name: %+v", f)
		}
		if f.Kcal100 <= 0 {
			t.Errorf("%s has no calories", f.DE)
		}
		if f.PortionG <= 0 || f.PortionDE == "" || f.PortionEN == "" {
			t.Errorf("%s has no usable portion", f.DE)
		}
		// Calories must be consistent with the energy-bearing components,
		// within the rounding slack of a reference table. Alcohol carries
		// 7 kcal/g and would otherwise make beer and wine look wrong.
		macro := f.P100*4 + f.C100*4 + f.F100*9 + f.Alc100*7
		if diff := macro - f.Kcal100; diff > f.Kcal100*0.25 || diff < -f.Kcal100*0.25 {
			t.Errorf("%s: %.0f kcal/100g but macros give %.0f", f.DE, f.Kcal100, macro)
		}
	}
}

func TestSearchMatchesGermanAndEnglish(t *testing.T) {
	cases := []struct{ query, wantDE string }{
		{"eis", "Eis"},
		{"ice cream", "Eis"},
		{"broetchen", "Brötchen"},
		{"Brötchen", "Brötchen"},
		{"magerquark", "Magerquark"},
		{"peanut butter", "Erdnussmus"},
		{"haribo", "Gummibärchen"},
		{"pizza", "Pizza (Margherita)"},
	}
	for _, tc := range cases {
		got := SearchFoods(tc.query, "de", 1)
		if len(got) == 0 {
			t.Errorf("%q: no match", tc.query)
			continue
		}
		if got[0].DE != tc.wantDE {
			t.Errorf("%q matched %q, want %q", tc.query, got[0].DE, tc.wantDE)
		}
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	if got := SearchFoods("   ", "de", 5); len(got) != 0 {
		t.Errorf("blank query returned %d results", len(got))
	}
}

func TestEstimateLocalParsesQuantities(t *testing.T) {
	cases := []struct {
		desc        string
		wantMinKcal float64
		wantMaxKcal float64
	}{
		// One scoop is ~55 g at 200 kcal/100 g, so two scoops ~220 kcal.
		{"2 Kugeln Eis", 180, 260},
		{"1 Kugel Eis", 90, 130},
		{"150 g Nudeln", 200, 280},
		{"Apfel", 60, 100},
		{"3 Scheiben Vollkornbrot", 250, 320},
	}
	for _, tc := range cases {
		est, err := EstimateLocal(tc.desc, "de")
		if err != nil {
			t.Errorf("%q: %v", tc.desc, err)
			continue
		}
		if est.Kcal < tc.wantMinKcal || est.Kcal > tc.wantMaxKcal {
			t.Errorf("%q: %.0f kcal, want %.0f–%.0f", tc.desc, est.Kcal, tc.wantMinKcal, tc.wantMaxKcal)
		}
		if est.Source != "local_db" {
			t.Errorf("%q: source %q", tc.desc, est.Source)
		}
		if est.Confidence != "low" {
			t.Errorf("%q: a table lookup should be low confidence", tc.desc)
		}
		if len(est.Items) != 1 || est.Items[0].Amount == "" {
			t.Errorf("%q: items = %+v", tc.desc, est.Items)
		}
	}
}

func TestEstimateLocalScalesLinearly(t *testing.T) {
	one, err := EstimateLocal("1 Kugel Eis", "de")
	if err != nil {
		t.Fatal(err)
	}
	four, err := EstimateLocal("4 Kugeln Eis", "de")
	if err != nil {
		t.Fatal(err)
	}
	if ratio := four.Kcal / one.Kcal; ratio < 3.8 || ratio > 4.2 {
		t.Errorf("four scoops / one scoop = %.2f, want ~4", ratio)
	}
}

func TestEstimateLocalLanguage(t *testing.T) {
	en, err := EstimateLocal("2 scoops ice cream", "en")
	if err != nil {
		t.Fatal(err)
	}
	if en.Items[0].Name != "Ice cream" {
		t.Errorf("English name = %q", en.Items[0].Name)
	}
	de, err := EstimateLocal("2 Kugeln Eis", "de")
	if err != nil {
		t.Fatal(err)
	}
	if de.Items[0].Name != "Eis" {
		t.Errorf("German name = %q", de.Items[0].Name)
	}
	if en.Kcal != de.Kcal {
		t.Errorf("language changed the calories: %v vs %v", en.Kcal, de.Kcal)
	}
}

func TestEstimateLocalUnknown(t *testing.T) {
	if _, err := EstimateLocal("qwertzuiop", "de"); err == nil {
		t.Error("expected an error for an unknown food")
	}
	if _, err := EstimateLocal("", "de"); err == nil {
		t.Error("expected an error for an empty description")
	}
}

func TestClientDisabledWithoutKey(t *testing.T) {
	c := New("", "")
	if c.Enabled() {
		t.Error("client should be disabled without an API key")
	}
	if c.Model() != DefaultModel {
		t.Errorf("model = %q, want the default %q", c.Model(), DefaultModel)
	}
	if _, err := c.EstimateText(t.Context(), "Eis", "de"); err != ErrNoKey {
		t.Errorf("expected ErrNoKey, got %v", err)
	}
	if _, err := c.EstimateImage(t.Context(), []byte{1}, "image/png", "", "de"); err != ErrNoKey {
		t.Errorf("expected ErrNoKey, got %v", err)
	}
}

func TestClientEnabledWithKey(t *testing.T) {
	c := New("sk-ant-test", "claude-sonnet-5")
	if !c.Enabled() {
		t.Error("client should be enabled with a key")
	}
	if c.Model() != "claude-sonnet-5" {
		t.Errorf("model = %q", c.Model())
	}
}

func TestNormalize(t *testing.T) {
	for input, want := range map[string]string{
		"BRÖTCHEN":    "brötchen",
		"Weiß-Brot":   "weiß brot",
		"  Eis  ":     "eis",
		"Ice   Cream": "ice cream",
	} {
		if got := normalize(input); got != want {
			t.Errorf("normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

// A German word with umlauts is spelled two ways on a keyboard without them,
// and both must find the same food.
func TestFoldVariants(t *testing.T) {
	got := foldVariants("Brötchen")
	want := map[string]bool{"broetchen": true, "brotchen": true}
	if len(got) != 2 {
		t.Fatalf("foldVariants = %v, want two spellings", got)
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("unexpected variant %q", v)
		}
	}
	// A word with no umlauts yields exactly one variant.
	if v := foldVariants("Eis"); len(v) != 1 || v[0] != "eis" {
		t.Errorf(`foldVariants("Eis") = %v`, v)
	}
	// ß has a single accepted transliteration.
	if v := foldVariants("Weiß"); len(v) != 1 || v[0] != "weiss" {
		t.Errorf(`foldVariants("Weiß") = %v`, v)
	}
}

func TestSearchFindsBothUmlautSpellings(t *testing.T) {
	for _, q := range []string{"Brötchen", "broetchen", "brotchen", "BRÖTCHEN"} {
		got := SearchFoods(q, "de", 1)
		if len(got) == 0 || got[0].DE != "Brötchen" {
			t.Errorf("%q did not find Brötchen (got %v)", q, got)
		}
	}
	for _, q := range []string{"Hähnchenbrust", "haehnchenbrust", "hahnchenbrust"} {
		got := SearchFoods(q, "de", 1)
		if len(got) == 0 || got[0].DE != "Hähnchenbrust" {
			t.Errorf("%q did not find Hähnchenbrust (got %v)", q, got)
		}
	}
}
