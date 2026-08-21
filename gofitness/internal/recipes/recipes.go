// Package recipes holds the curated recipe database that meal planning draws
// from. Recipes are baked into the binary, so the add-on works with no internet
// connection and no external API.
package recipes

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
)

//go:embed data/recipes.json
var recipesJSON []byte

// Ingredient is one line on the shopping list.
type Ingredient struct {
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Unit     string  `json:"unit"`
	Category string  `json:"category"`
	// Pantry marks staples (salt, oil, spices) that are usually already at
	// home; they are grouped separately on the shopping list.
	Pantry bool `json:"pantry,omitempty"`
}

// Recipe is a single dish, with everything needed to shop for and cook it.
type Recipe struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// Search is the phrase used to build a working recipe-portal link.
	Search string `json:"search"`
	// URL is filled in at load time from Search.
	URL string `json:"url"`

	Servings    int     `json:"servings"`
	PrepMinutes int     `json:"prep_minutes"`
	Kcal        float64 `json:"kcal"` // per serving
	ProteinG    float64 `json:"protein_g"`
	CarbsG      float64 `json:"carbs_g"`
	FatG        float64 `json:"fat_g"`
	// PortionG is the approximate weight of one finished portion, in grams. It
	// answers "what is one portion" concretely, since a serving count alone is
	// ambiguous. Derived from the ingredient weights in tools/nutrition.
	PortionG float64 `json:"portion_g"`

	MealTypes []string `json:"meal_types"` // breakfast, lunch, dinner, snack
	Tags      []string `json:"tags"`

	// VeggieLevel is how prominent vegetables are: "low", "medium", "high".
	VeggieLevel string `json:"veggie_level"`
	// ContainsFish and FishBreaded together encode the "only breaded fish" rule.
	ContainsFish bool `json:"contains_fish"`
	FishBreaded  bool `json:"fish_breaded"`
	// BreastfeedingSafe means fully cooked, no raw fish/egg, no alcohol, no
	// unpasteurised cheese and no high-caffeine content.
	BreastfeedingSafe bool `json:"breastfeeding_safe"`
	KidFriendly       bool `json:"kid_friendly"`
	// MealPrep marks dishes that keep well for a few days in the fridge.
	MealPrep bool `json:"meal_prep"`
	// Freezable marks dishes that survive the freezer.
	Freezable bool `json:"freezable"`

	Ingredients []Ingredient `json:"ingredients"`
	Steps       []string     `json:"steps"`
}

// HasMealType reports whether the recipe fits a slot.
func (r Recipe) HasMealType(t string) bool {
	for _, m := range r.MealTypes {
		if m == t {
			return true
		}
	}
	return false
}

// HasTag reports whether the recipe carries a tag.
func (r Recipe) HasTag(t string) bool {
	for _, x := range r.Tags {
		if strings.EqualFold(x, t) {
			return true
		}
	}
	return false
}

// veggieRank orders the veggie levels so they can be compared.
var veggieRank = map[string]int{"low": 0, "medium": 1, "high": 2}

// VeggieRank returns 0 (low) to 2 (high); unknown values count as medium.
func (r Recipe) VeggieRank() int {
	if v, ok := veggieRank[r.VeggieLevel]; ok {
		return v
	}
	return 1
}

// RankFor converts a preference level into the highest acceptable rank.
func RankFor(level string) int {
	if v, ok := veggieRank[level]; ok {
		return v
	}
	return 1
}

// Book is the loaded recipe collection.
type Book struct {
	all   []Recipe
	byID  map[string]Recipe
	order []string
}

var loaded *Book

// Load parses the embedded recipe database. The result is cached.
func Load() (*Book, error) {
	if loaded != nil {
		return loaded, nil
	}
	var list []Recipe
	if err := json.Unmarshal(recipesJSON, &list); err != nil {
		return nil, fmt.Errorf("parse recipes: %w", err)
	}
	if err := loadI18n(); err != nil {
		return nil, err
	}
	b := &Book{byID: make(map[string]Recipe, len(list))}
	for i := range list {
		r := &list[i]
		if r.URL == "" {
			r.URL = searchURL(r.Search, r.Title)
		}
		if r.Servings <= 0 {
			r.Servings = 4
		}
		if _, dup := b.byID[r.ID]; dup {
			return nil, fmt.Errorf("duplicate recipe id %q", r.ID)
		}
		b.byID[r.ID] = *r
		b.order = append(b.order, r.ID)
	}
	b.all = list
	loaded = b
	return b, nil
}

// MustLoad is Load but panics on a malformed database, which can only happen if
// the embedded JSON was broken at build time.
func MustLoad() *Book {
	b, err := Load()
	if err != nil {
		panic(err)
	}
	return b
}

// searchURL builds a Chefkoch search link. A search link is used rather than a
// direct recipe id so the link keeps working even as the site reorganises; the
// full recipe is in the app anyway.
func searchURL(search, title string) string {
	q := search
	if q == "" {
		q = title
	}
	return "https://www.chefkoch.de/rs/s0/" + url.PathEscape(q) + "/Rezepte.html"
}

// All returns every recipe.
func (b *Book) All() []Recipe {
	out := make([]Recipe, len(b.all))
	copy(out, b.all)
	return out
}

// Get looks up a recipe by id.
func (b *Book) Get(id string) (Recipe, bool) {
	r, ok := b.byID[id]
	return r, ok
}

// Len returns the number of recipes.
func (b *Book) Len() int { return len(b.all) }

// Filter describes which recipes are acceptable for a household.
type Filter struct {
	MealType          string
	MaxVeggieRank     int
	FishPolicy        string // "breaded_only", "any", "none"
	BreastfeedingSafe bool
	MaxCookMinutes    int
	MealPrepOnly      bool
	ExcludeTags       []string
	ExcludeIngredient []string
	MinKcal           float64
	MaxKcal           float64
	Query             string
}

// Matches reports whether a recipe satisfies the filter.
func (b *Book) Matches(r Recipe, f Filter) bool {
	if f.MealType != "" && !r.HasMealType(f.MealType) {
		return false
	}
	if f.MaxVeggieRank > 0 && r.VeggieRank() > f.MaxVeggieRank {
		return false
	}
	if f.MaxVeggieRank == 0 && r.VeggieRank() > 0 {
		// Rank 0 means "keep vegetables in the background".
		return false
	}
	if r.ContainsFish {
		switch f.FishPolicy {
		case "none":
			return false
		case "breaded_only":
			if !r.FishBreaded {
				return false
			}
		}
	}
	if f.BreastfeedingSafe && !r.BreastfeedingSafe {
		return false
	}
	if f.MaxCookMinutes > 0 && r.PrepMinutes > f.MaxCookMinutes {
		return false
	}
	if f.MealPrepOnly && !r.MealPrep {
		return false
	}
	for _, t := range f.ExcludeTags {
		if t != "" && r.HasTag(t) {
			return false
		}
	}
	for _, bad := range f.ExcludeIngredient {
		bad = strings.ToLower(strings.TrimSpace(bad))
		if bad == "" {
			continue
		}
		if strings.Contains(strings.ToLower(r.Title), bad) {
			return false
		}
		for _, ing := range r.Ingredients {
			if strings.Contains(strings.ToLower(ing.Name), bad) {
				return false
			}
		}
	}
	if f.MinKcal > 0 && r.Kcal < f.MinKcal {
		return false
	}
	if f.MaxKcal > 0 && r.Kcal > f.MaxKcal {
		return false
	}
	if f.Query != "" {
		q := strings.ToLower(f.Query)
		if !strings.Contains(strings.ToLower(r.Title), q) &&
			!strings.Contains(strings.ToLower(r.Description), q) {
			return false
		}
	}
	return true
}

// Select returns every recipe matching the filter, ordered by title.
func (b *Book) Select(f Filter) []Recipe {
	var out []Recipe
	for _, r := range b.all {
		if b.Matches(r, f) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

// ScaleIngredients returns the ingredient list scaled to the given number of
// servings, rounded to something you can actually buy.
func (r Recipe) ScaleIngredients(servings float64) []Ingredient {
	if r.Servings <= 0 || servings <= 0 {
		return r.Ingredients
	}
	factor := servings / float64(r.Servings)
	out := make([]Ingredient, 0, len(r.Ingredients))
	for _, ing := range r.Ingredients {
		ing.Amount = roundAmount(ing.Amount * factor)
		out = append(out, ing)
	}
	return out
}

// roundAmount keeps shopping amounts tidy: whole grams for large quantities,
// halves for small ones. A positive amount never rounds down to zero — a recipe
// that needs a pinch of salt must still say so on the list.
func roundAmount(v float64) float64 {
	if v <= 0 {
		return 0
	}
	var out float64
	switch {
	case v >= 100:
		out = math.Round(v/5) * 5
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
