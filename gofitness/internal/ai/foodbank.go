package ai

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

//go:embed data/foods.json
var foodsJSON []byte

// Food is one entry in the local lookup table, stored per 100 g plus a typical
// household portion so "1 Scheibe Brot" resolves without the user weighing it.
type Food struct {
	DE      string  `json:"de"`
	EN      string  `json:"en"`
	Kcal100 float64 `json:"kcal100"`
	P100    float64 `json:"p100"`
	C100    float64 `json:"c100"`
	F100    float64 `json:"f100"`
	// Alc100 is grams of alcohol per 100 g. Alcohol carries 7 kcal/g but is not
	// a macronutrient, so it is tracked separately to keep the calorie figure
	// honest without inventing carbs that are not there.
	Alc100    float64  `json:"alc100"`
	PortionG  float64  `json:"portion_g"`
	PortionDE string   `json:"portion_de"`
	PortionEN string   `json:"portion_en"`
	Aliases   []string `json:"aliases"`
}

// Name returns the food's name in the requested language.
func (f Food) Name(lang string) string {
	if lang == "en" && f.EN != "" {
		return f.EN
	}
	return f.DE
}

// PortionLabel returns the household portion description.
func (f Food) PortionLabel(lang string) string {
	if lang == "en" && f.PortionEN != "" {
		return f.PortionEN
	}
	return f.PortionDE
}

var foods []Food

func loadFoods() []Food {
	if foods == nil {
		if err := json.Unmarshal(foodsJSON, &foods); err != nil {
			// The table is embedded at build time; a parse failure is a bug,
			// but degrading to an empty table beats crashing the add-on.
			foods = []Food{}
		}
	}
	return foods
}

// Foods exposes the local table, e.g. for a search box in the interface.
func Foods() []Food { return loadFoods() }

// SearchFoods returns entries matching a free-text query, best match first.
func SearchFoods(query, lang string, limit int) []Food {
	queries := foldVariants(query)
	if len(queries) == 0 || queries[0] == "" {
		return nil
	}
	type hit struct {
		f     Food
		score int
	}
	var hits []hit
	for _, f := range loadFoods() {
		best := 0
		for _, q := range queries {
			if s := matchScore(f, q); s > best {
				best = s
			}
		}
		if best > 0 {
			hits = append(hits, hit{f, best})
		}
	}
	// Higher score first; stable enough without a full sort package import.
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].score > hits[j-1].score; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Food, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.f)
	}
	return out
}

// matchScore rates how well a food matches a normalised query.
func matchScore(f Food, q string) int {
	best := 0
	consider := func(name string, bonus int) {
		for _, n := range foldVariants(name) {
			considerFold(n, q, bonus, &best)
		}
	}
	consider(f.DE, 5)
	consider(f.EN, 4)
	for _, a := range f.Aliases {
		consider(a, 0)
	}
	return best
}

// considerFold scores one spelling of a name against the query.
func considerFold(n, q string, bonus int, best *int) {
	if n == "" {
		return
	}
	switch {
	case n == q:
		if 100+bonus > *best {
			*best = 100 + bonus
		}
	case strings.HasPrefix(n, q) || strings.HasPrefix(q, n):
		if 70+bonus > *best {
			*best = 70 + bonus
		}
	case strings.Contains(q, n) || strings.Contains(n, q):
		if 40+bonus > *best {
			*best = 40 + bonus
		}
	}
}

// normalize lower-cases and collapses whitespace. Umlauts are handled by
// foldVariants, because German has two competing transliterations.
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", " ")
	return strings.Join(strings.Fields(s), " ")
}

// foldVariants returns the spellings a user might type for the same word:
// "Brötchen" is written both "broetchen" and "brotchen" on a keyboard without
// umlauts, and matching only one of them loses real searches.
func foldVariants(s string) []string {
	s = normalize(s)
	long := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss").Replace(s)
	short := strings.NewReplacer("ä", "a", "ö", "o", "ü", "u", "ß", "ss").Replace(s)
	if long == short {
		return []string{long}
	}
	return []string{long, short}
}

// quantityRe pulls a leading amount out of a description: "2 Scheiben Brot",
// "150 g Nudeln", "1,5 Portionen Reis".
var quantityRe = regexp.MustCompile(`^\s*(\d+(?:[.,]\d+)?)\s*(g|gramm|grams|kg|ml|l|st|stk|stück|stueck|pieces?|scheiben?|slices?|kugeln?|scoops?|portionen?|servings?|handvoll|handfuls?|el|tbsp|tl|tsp|glas|glasses?|becher|dose|cans?|tasse|cups?|riegel|bars?)?\s*(.*)$`)

// EstimateLocal answers from the local table. It handles a leading quantity and
// falls back to the food's typical portion when none is given.
//
// The result is deliberately marked low confidence: a lookup table cannot know
// how much butter went into the pan.
func EstimateLocal(description, lang string) (Estimate, error) {
	desc := strings.TrimSpace(description)
	if desc == "" {
		return Estimate{}, fmt.Errorf("no description given")
	}

	qty, unit, rest := splitQuantity(desc)
	name := rest
	if name == "" {
		name = desc
	}

	matches := SearchFoods(name, lang, 1)
	if len(matches) == 0 {
		// Try the whole string in case the quantity split ate a real word.
		matches = SearchFoods(desc, lang, 1)
	}
	if len(matches) == 0 {
		return Estimate{}, fmt.Errorf("kein Treffer in der lokalen Lebensmitteltabelle für %q", description)
	}
	f := matches[0]

	grams, label := resolveAmount(f, qty, unit, lang)
	factor := grams / 100

	item := Item{
		Name:     f.Name(lang),
		Amount:   label,
		Kcal:     round(f.Kcal100 * factor),
		ProteinG: round(f.P100 * factor),
		CarbsG:   round(f.C100 * factor),
		FatG:     round(f.F100 * factor),
	}

	note := "Aus der lokalen Lebensmitteltabelle geschätzt – Portionsgröße bitte prüfen."
	if lang == "en" {
		note = "Estimated from the local food table — please check the portion size."
	}

	return Estimate{
		Items:       []Item{item},
		Kcal:        item.Kcal,
		ProteinG:    item.ProteinG,
		CarbsG:      item.CarbsG,
		FatG:        item.FatG,
		Confidence:  "low",
		Assumptions: note,
		MealType:    "snack",
		Source:      "local_db",
	}, nil
}

// splitQuantity separates "2 Scheiben" from "Vollkornbrot".
func splitQuantity(s string) (qty float64, unit, rest string) {
	m := quantityRe.FindStringSubmatch(s)
	if m == nil {
		return 0, "", s
	}
	v, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
	if err != nil {
		return 0, "", s
	}
	return v, strings.ToLower(m[2]), strings.TrimSpace(m[3])
}

// resolveAmount turns a parsed quantity into grams plus a human label.
func resolveAmount(f Food, qty float64, unit, lang string) (grams float64, label string) {
	portion := f.PortionG
	if portion <= 0 {
		portion = 100
	}
	switch unit {
	case "g", "gramm", "grams", "ml":
		// Millilitres are treated as grams; for the drinks in this table the
		// density is close enough to 1 that the error is negligible.
		if qty > 0 {
			return qty, trim(qty) + " g"
		}
	case "kg":
		if qty > 0 {
			return qty * 1000, trim(qty) + " kg"
		}
	case "l":
		if qty > 0 {
			return qty * 1000, trim(qty) + " l"
		}
	case "":
		if qty > 0 {
			// A bare number means "that many normal portions".
			return qty * portion, trim(qty) + " × " + f.PortionLabel(lang)
		}
	default:
		if qty > 0 {
			return qty * portion, trim(qty) + " × " + f.PortionLabel(lang)
		}
	}
	return portion, f.PortionLabel(lang)
}

func trim(v float64) string {
	if v == math.Trunc(v) {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}
