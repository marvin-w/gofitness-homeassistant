package store

import (
	"context"
	"encoding/json"
)

// HouseholdID is the pseudo-user that owns everything shared by the whole
// household rather than by one person: the meal plan and its shopping list.
// Every Home Assistant user still has their own profile, weight and food log,
// but they all look at — and cook from — the same plan.
const HouseholdID = "__household__"

// ensureHousehold creates the shared pseudo-user so the meal_plans foreign key
// has something to point at. Called once at startup.
func (s *Store) ensureHousehold(ctx context.Context) error {
	return s.EnsureUser(ctx, HouseholdID, "Household")
}

const householdPrefsKey = "household_prefs"

// HouseholdPrefs returns the shared meal-planning settings. These are global:
// whatever one household member sets applies to everyone's view of the plan.
// The per-user interface language is NOT part of this — that stays on each
// profile. Missing settings fall back to the household defaults.
func (s *Store) HouseholdPrefs(ctx context.Context) (Prefs, error) {
	p := DefaultPrefs()
	if raw := s.Setting(ctx, householdPrefsKey, ""); raw != "" {
		_ = json.Unmarshal([]byte(raw), &p)
	}
	// Cook-once-eat-twice is always on: the household explicitly wants leftovers
	// reused, so it is no longer an optional toggle.
	p.CookOnceEatTwice = true
	return p, nil
}

// SaveHouseholdPrefs persists the shared meal-planning settings. Only the
// meal-plan fields are stored; Language is ignored here because it is per-user.
func (s *Store) SaveHouseholdPrefs(ctx context.Context, p Prefs) error {
	p.Language = ""
	p.CookOnceEatTwice = true
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.SetSetting(ctx, householdPrefsKey, string(b))
}
