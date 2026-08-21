package recipes

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

//go:embed data/recipes.en.json
var recipesEnJSON []byte

//go:embed data/i18n.json
var i18nJSON []byte

// LangDE and LangEN are the supported interface languages. German is the
// primary language: it is what the recipe database is authored in, and any
// unknown language code falls back to it.
const (
	LangDE = "de"
	LangEN = "en"
)

// NormalizeLang maps anything the client sends onto a supported language.
func NormalizeLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "en") {
		return LangEN
	}
	return LangDE
}

// translation is the English text for one recipe.
type translation struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Search      string   `json:"search"`
	Steps       []string `json:"steps"`
}

// vocab holds the shared word lists used across recipes.
type vocab struct {
	Ingredients map[string]string `json:"ingredients"`
	Units       map[string]string `json:"units"`
	Categories  map[string]string `json:"categories"`
	Tags        map[string]string `json:"tags"`
	MealTypes   map[string]string `json:"meal_types"`
}

var (
	enText  map[string]translation
	enVocab vocab
)

// loadI18n parses the translation tables. Called once from Load.
func loadI18n() error {
	if err := json.Unmarshal(recipesEnJSON, &enText); err != nil {
		return fmt.Errorf("parse english recipes: %w", err)
	}
	if err := json.Unmarshal(i18nJSON, &enVocab); err != nil {
		return fmt.Errorf("parse i18n vocabulary: %w", err)
	}
	return nil
}

// Localize returns the recipe with its text swapped to the requested language.
// German recipes are returned untouched; for English, any field without a
// translation keeps its German original rather than disappearing.
func Localize(r Recipe, lang string) Recipe {
	if NormalizeLang(lang) != LangEN {
		return r
	}

	if t, ok := enText[r.ID]; ok {
		if t.Title != "" {
			r.Title = t.Title
		}
		if t.Description != "" {
			r.Description = t.Description
		}
		if len(t.Steps) > 0 {
			r.Steps = append([]string(nil), t.Steps...)
		}
		if t.Search != "" {
			r.Search = t.Search
			// English users get an English-language recipe portal; a Chefkoch
			// search for English words would return nothing useful.
			r.URL = "https://www.allrecipes.com/search?q=" + url.QueryEscape(t.Search)
		}
	}

	ings := make([]Ingredient, len(r.Ingredients))
	for i, ing := range r.Ingredients {
		ing.Name = TranslateIngredient(ing.Name, lang)
		ing.Unit = TranslateUnit(ing.Unit, lang)
		ing.Category = TranslateCategory(ing.Category, lang)
		ings[i] = ing
	}
	r.Ingredients = ings

	tags := make([]string, len(r.Tags))
	for i, t := range r.Tags {
		tags[i] = TranslateTag(t, lang)
	}
	r.Tags = tags

	return r
}

// LocalizeAll localizes a slice of recipes.
func LocalizeAll(list []Recipe, lang string) []Recipe {
	if NormalizeLang(lang) != LangEN {
		return list
	}
	out := make([]Recipe, len(list))
	for i, r := range list {
		out[i] = Localize(r, lang)
	}
	return out
}

func lookup(table map[string]string, key, lang string) string {
	if NormalizeLang(lang) != LangEN {
		return key
	}
	if v, ok := table[key]; ok && v != "" {
		return v
	}
	return key
}

// TranslateIngredient maps a German ingredient name into the target language.
func TranslateIngredient(name, lang string) string { return lookup(enVocab.Ingredients, name, lang) }

// TranslateUnit maps a German unit into the target language.
func TranslateUnit(unit, lang string) string { return lookup(enVocab.Units, unit, lang) }

// TranslateCategory maps a shopping-list aisle into the target language.
func TranslateCategory(cat, lang string) string { return lookup(enVocab.Categories, cat, lang) }

// TranslateTag maps a recipe tag into the target language.
func TranslateTag(tag, lang string) string { return lookup(enVocab.Tags, tag, lang) }

// TranslateMealType gives the display name of a meal slot.
func TranslateMealType(slot, lang string) string {
	if NormalizeLang(lang) == LangEN {
		if v, ok := enVocab.MealTypes[slot]; ok {
			return v
		}
		return slot
	}
	switch slot {
	case "breakfast":
		return "Frühstück"
	case "lunch":
		return "Mittagessen"
	case "dinner":
		return "Abendessen"
	case "snack":
		return "Snack"
	}
	return slot
}

// MissingTranslations lists recipe ids that have no English text yet. It exists
// so a test can keep the two data files from drifting apart.
func (b *Book) MissingTranslations() []string {
	var out []string
	for _, r := range b.all {
		t, ok := enText[r.ID]
		if !ok || t.Title == "" || t.Description == "" || len(t.Steps) != len(r.Steps) {
			out = append(out, r.ID)
		}
	}
	return out
}

// UntranslatedIngredients lists ingredient names with no English equivalent.
func (b *Book) UntranslatedIngredients() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range b.all {
		for _, ing := range r.Ingredients {
			if seen[ing.Name] {
				continue
			}
			seen[ing.Name] = true
			if v, ok := enVocab.Ingredients[ing.Name]; !ok || v == "" {
				out = append(out, ing.Name)
			}
		}
	}
	return out
}

// LocalizeIngredients translates a scaled ingredient list.
func LocalizeIngredients(list []Ingredient, lang string) []Ingredient {
	if NormalizeLang(lang) != LangEN {
		return list
	}
	out := make([]Ingredient, len(list))
	for i, ing := range list {
		ing.Name = TranslateIngredient(ing.Name, lang)
		ing.Unit = TranslateUnit(ing.Unit, lang)
		ing.Category = TranslateCategory(ing.Category, lang)
		out[i] = ing
	}
	return out
}
