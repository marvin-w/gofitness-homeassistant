// Package mealplan turns a calorie target plus a household's preferences into a
// concrete week of meals and the shopping list to buy for it.
package mealplan

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/recipes"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

// weekdaysDE and weekdaysEN name the days, index 0 = Monday.
var (
	weekdaysDE = []string{"Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag", "Sonntag"}
	weekdaysEN = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
)

// Weekdays returns the day names in the requested language.
func Weekdays(lang string) []string {
	if recipes.NormalizeLang(lang) == recipes.LangEN {
		return weekdaysEN
	}
	return weekdaysDE
}

// slotShare is how the daily calorie budget is divided between meals.
var slotShare4 = map[string]float64{"breakfast": 0.25, "lunch": 0.35, "dinner": 0.30, "snack": 0.10}
var slotShare3 = map[string]float64{"breakfast": 0.30, "lunch": 0.40, "dinner": 0.30}

// Entry is one planned meal.
type Entry struct {
	DayIndex int     `json:"day_index"`
	Day      string  `json:"day"`
	MealType string  `json:"meal_type"`
	RecipeID string  `json:"recipe_id"`
	Title    string  `json:"title"`
	URL      string  `json:"url"`
	Portions float64 `json:"portions"`
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
	// PortionG is the approximate finished weight of one portion of this dish,
	// so the plan can say how much to actually cook.
	PortionG float64 `json:"portion_g"`
	// Leftover marks the second appearance of a dish cooked in bulk earlier.
	Leftover bool `json:"leftover"`
	Cooked   bool `json:"cooked"`
}

// Day groups the entries of one calendar day.
type Day struct {
	Index    int     `json:"index"`
	Name     string  `json:"name"`
	Date     string  `json:"date"`
	Entries  []Entry `json:"entries"`
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
}

// Note is a translatable advisory about the generated plan. It mirrors
// nutrition.Note so the interface renders both with the same code.
type Note struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
}

// Plan is a full week.
type Plan struct {
	WeekStart  string  `json:"week_start"`
	TargetKcal float64 `json:"target_kcal"`
	Days       []Day   `json:"days"`
	AvgKcal    float64 `json:"avg_kcal"`
	Notes      []Note  `json:"notes"`
}

// Options controls generation.
type Options struct {
	WeekStart  string // YYYY-MM-DD, a Monday
	Dates      []string
	TargetKcal float64
	Prefs      store.Prefs
	// BreastfeedingSafe restricts the pool to lactation-safe dishes.
	BreastfeedingSafe bool
	// Seed varies the outcome; pass a different value to reshuffle.
	Seed uint64
	// CookOnceEatTwice repeats bulk-friendly dinners as the next day's lunch.
	CookOnceEatTwice bool
	// MaxPortions caps how many recipe servings one slot may cook. For a shared
	// household plan sized to everyone's combined target this scales with the
	// number of people; 0 falls back to 2.
	MaxPortions float64
	// Lang selects the language for titles, notes and day names.
	Lang string
}

// Generate builds a week of meals. It never fails outright: if the filters are
// so tight that a slot cannot be filled, that slot is left out and a note
// explains why, rather than silently substituting something the user vetoed.
func Generate(book *recipes.Book, opts Options) Plan {
	prefs := opts.Prefs
	if prefs.MealsPerDay == 0 {
		prefs = store.DefaultPrefs()
	}
	shares := slotShare4
	slots := []string{"breakfast", "lunch", "dinner", "snack"}
	if prefs.MealsPerDay <= 3 {
		shares = slotShare3
		slots = []string{"breakfast", "lunch", "dinner"}
	}

	baseFilter := recipes.Filter{
		MaxVeggieRank:     recipes.RankFor(prefs.VeggieLevel),
		FishPolicy:        prefs.FishPolicy,
		BreastfeedingSafe: opts.BreastfeedingSafe,
		ExcludeTags:       prefs.ExcludeTags,
		ExcludeIngredient: prefs.ExcludeIngredients,
	}

	maxPortions := opts.MaxPortions
	if maxPortions < 1 {
		maxPortions = 2
	}

	lang := recipes.NormalizeLang(opts.Lang)
	names := Weekdays(lang)
	plan := Plan{WeekStart: opts.WeekStart, TargetKcal: opts.TargetKcal}

	// Track how often each dish is already used so the week stays varied, and
	// how often fish appeared so the weekly cap is honoured.
	used := map[string]int{}
	fishCount := 0
	// leftovers maps a day index to a dinner cooked the evening before.
	leftovers := map[int]Entry{}

	rnd := newRand(opts.Seed)

	for d := 0; d < 7; d++ {
		day := Day{Index: d, Name: names[d]}
		if d < len(opts.Dates) {
			day.Date = opts.Dates[d]
		}

		for _, slot := range slots {
			// Yesterday's bulk dinner becomes today's lunch.
			if slot == "lunch" && opts.CookOnceEatTwice {
				if lo, ok := leftovers[d]; ok {
					lo.DayIndex = d
					lo.Day = day.Name
					lo.MealType = "lunch"
					lo.Leftover = true
					day.Entries = append(day.Entries, lo)
					continue
				}
			}

			f := baseFilter
			f.MealType = slot
			// Weeknight time budget applies to the cooked main meals only.
			if slot == "lunch" || slot == "dinner" {
				f.MaxCookMinutes = prefs.MaxCookMinutes
			}

			candidates := book.Select(f)
			// If the time limit empties the pool, relax it before giving up —
			// a longer recipe beats no dinner at all.
			if len(candidates) == 0 && f.MaxCookMinutes > 0 {
				f.MaxCookMinutes = 0
				candidates = book.Select(f)
			}
			if len(candidates) == 0 {
				plan.Notes = append(plan.Notes, Note{
					Code:   "slot_unfilled",
					Params: map[string]any{"meal": slot, "day": d},
				})
				continue
			}

			slotTarget := opts.TargetKcal * shares[slot]
			pick := choose(candidates, slotTarget, used, fishCount, prefs.MaxFishPerWeek, maxPortions, rnd)
			if pick == nil {
				continue
			}

			loc := recipes.Localize(*pick, lang)
			portions := clampPortions(slotTarget/math.Max(pick.Kcal, 1), maxPortions)
			e := Entry{
				DayIndex: d,
				Day:      day.Name,
				MealType: slot,
				RecipeID: pick.ID,
				Title:    loc.Title,
				URL:      loc.URL,
				Portions: portions,
				Kcal:     round(pick.Kcal * portions),
				ProteinG: round(pick.ProteinG * portions),
				CarbsG:   round(pick.CarbsG * portions),
				FatG:     round(pick.FatG * portions),
				PortionG: pick.PortionG,
			}
			day.Entries = append(day.Entries, e)

			used[pick.ID]++
			if pick.ContainsFish {
				fishCount++
			}
			// Queue tomorrow's lunch when the dish keeps well.
			if opts.CookOnceEatTwice && slot == "dinner" && pick.MealPrep && d < 6 {
				leftovers[d+1] = e
			}
		}

		for _, e := range day.Entries {
			day.Kcal += e.Kcal
			day.ProteinG += e.ProteinG
			day.CarbsG += e.CarbsG
			day.FatG += e.FatG
		}
		plan.AvgKcal += day.Kcal
		plan.Days = append(plan.Days, day)
	}

	if len(plan.Days) > 0 {
		plan.AvgKcal = round(plan.AvgKcal / float64(len(plan.Days)))
	}
	if fishCount == 0 && prefs.FishPolicy != "none" {
		plan.Notes = append(plan.Notes, Note{Code: "fish_free_week"})
	}
	return plan
}

// SlotName gives the display label for a meal slot.
func SlotName(slot, lang string) string { return recipes.TranslateMealType(slot, lang) }

// choose picks the recipe whose calories land closest to the slot target,
// penalising dishes already used this week so the plan stays varied.
func choose(candidates []recipes.Recipe, target float64, used map[string]int, fishCount, maxFish int, maxPortions float64, rnd *rand) *recipes.Recipe {
	type scored struct {
		r     recipes.Recipe
		score float64
	}
	var list []scored
	for _, r := range candidates {
		if r.ContainsFish && fishCount >= maxFish {
			continue
		}
		// Distance from the slot target, normalised.
		portions := clampPortions(target/math.Max(r.Kcal, 1), maxPortions)
		diff := math.Abs(r.Kcal*portions-target) / math.Max(target, 1)
		score := diff
		// Each previous appearance makes a dish much less attractive.
		score += float64(used[r.ID]) * 0.9
		// A small stable jitter breaks ties differently for each seed.
		score += rnd.jitter(r.ID) * 0.25
		list = append(list, scored{r, score})
	}
	if len(list) == 0 {
		// Everything was filtered out by the fish cap; fall back to the
		// non-fish candidates so the slot still gets filled.
		for _, r := range candidates {
			if !r.ContainsFish {
				list = append(list, scored{r, float64(used[r.ID])})
			}
		}
		if len(list) == 0 {
			return nil
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].score == list[j].score {
			return list[i].r.ID < list[j].r.ID
		}
		return list[i].score < list[j].score
	})
	best := list[0].r
	return &best
}

// clampPortions keeps portion sizes realistic: never less than half a serving,
// never more than the household could plausibly eat in one sitting.
func clampPortions(p, max float64) float64 {
	if max < 1 {
		max = 2
	}
	if p < 0.5 {
		p = 0.5
	}
	if p > max {
		p = max
	}
	// Round to quarter portions so the number means something in a kitchen.
	return math.Round(p*4) / 4
}

func round(v float64) float64 { return math.Round(v) }

// rand is a tiny deterministic jitter source. A real PRNG is overkill here and
// a hash keeps regeneration reproducible for the same seed.
type rand struct{ seed uint64 }

func newRand(seed uint64) *rand { return &rand{seed: seed} }

// jitter maps an id to a stable value in [0,1) for this seed.
func (r *rand) jitter(id string) float64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d:%s", r.seed, id)
	return float64(h.Sum64()%1000) / 1000
}

// ShoppingItem is one aggregated line on the shopping list.
type ShoppingItem struct {
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Unit     string  `json:"unit"`
	Category string  `json:"category"`
	Pantry   bool    `json:"pantry"`
	Checked  bool    `json:"checked"`
}

// categoryOrder is the order aisles are walked in a typical German supermarket.
var categoryOrder = []string{
	"Obst & Gemüse", "Fleisch & Wurst", "Kühlregal", "Tiefkühl",
	"Trockenwaren", "Konserven", "Backen & Gewürze", "Getränke", "Sonstiges",
}

// ShoppingList aggregates every ingredient in the plan, merging identical
// name/unit pairs into a single line.
//
// Each entry's Portions already covers the whole household — the plan is sized
// to everyone's combined calorie target — so the ingredients are scaled by the
// portion count directly. This is still a shopping approximation, not a
// per-person nutrition calculation.
func ShoppingList(book *recipes.Book, plan Plan, lang string) []ShoppingItem {
	type key struct{ name, unit string }
	agg := map[key]*ShoppingItem{}

	for _, day := range plan.Days {
		for _, e := range day.Entries {
			if e.Leftover {
				continue // already bought for when the dish was cooked
			}
			r, ok := book.Get(e.RecipeID)
			if !ok {
				continue
			}
			for _, ing := range r.ScaleIngredients(e.Portions) {
				// Aggregate on the German name so the same ingredient merges
				// regardless of the display language, then translate once.
				k := key{strings.ToLower(ing.Name), ing.Unit}
				cat := defaultCategory(ing.Category)
				if cur, ok := agg[k]; ok {
					cur.Amount += ing.Amount
					continue
				}
				agg[k] = &ShoppingItem{
					Name:     recipes.TranslateIngredient(ing.Name, lang),
					Amount:   ing.Amount,
					Unit:     recipes.TranslateUnit(ing.Unit, lang),
					Category: cat,
					Pantry:   ing.Pantry,
				}
			}
		}
	}

	out := make([]ShoppingItem, 0, len(agg))
	for _, v := range agg {
		v.Amount = tidy(v.Amount)
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		ci, cj := catRank(out[i].Category), catRank(out[j].Category)
		if ci != cj {
			return ci < cj
		}
		if out[i].Pantry != out[j].Pantry {
			return !out[i].Pantry // things to buy first, staples last
		}
		return out[i].Name < out[j].Name
	})
	// Aisles are sorted by the canonical German order, then labelled.
	for i := range out {
		out[i].Category = recipes.TranslateCategory(out[i].Category, lang)
	}
	return out
}

func defaultCategory(c string) string {
	if strings.TrimSpace(c) == "" {
		return "Sonstiges"
	}
	return c
}

func catRank(c string) int {
	for i, x := range categoryOrder {
		if x == c {
			return i
		}
	}
	return len(categoryOrder)
}

// tidy rounds a shopping amount to something you can put in a basket, without
// ever rounding a real requirement away to zero.
func tidy(v float64) float64 {
	if v <= 0 {
		return 0
	}
	var out float64
	switch {
	case v >= 100:
		out = math.Round(v/10) * 10
	case v >= 10:
		out = math.Round(v)
	case v >= 1:
		out = math.Round(v*2) / 2
	default:
		out = math.Round(v*10) / 10
	}
	if out <= 0 {
		return 0.1
	}
	return out
}
