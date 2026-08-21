package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrNotFound is returned when a lookup has no row.
var ErrNotFound = errors.New("not found")

// Prefs holds the food preferences that shape meal planning. They are stored as
// JSON so new options can be added without a migration.
type Prefs struct {
	// FishPolicy is one of "breaded_only", "any" or "none". "breaded_only"
	// keeps fish on the menu as Fischstäbchen/Backfisch only.
	FishPolicy string `json:"fish_policy"`
	// MaxFishPerWeek caps how often fish shows up in a generated week.
	MaxFishPerWeek int `json:"max_fish_per_week"`
	// VeggieLevel is "low", "medium" or "high" — how prominent vegetables may be.
	VeggieLevel string `json:"veggie_level"`
	// HouseholdSize scales the shopping list.
	HouseholdSize int `json:"household_size"`
	// MealsPerDay is 3 (main meals) or 4 (with a snack).
	MealsPerDay int `json:"meals_per_day"`
	// MaxCookMinutes filters out recipes that take too long on a weeknight.
	MaxCookMinutes int `json:"max_cook_minutes"`
	// ExcludeTags lets the user veto whole categories, e.g. "scharf".
	ExcludeTags []string `json:"exclude_tags"`
	// ExcludeIngredients is a free-text deny list matched against ingredients.
	ExcludeIngredients []string `json:"exclude_ingredients"`
	// Language is the interface language, "de" or "en". Stored per profile so
	// two people sharing one Home Assistant can each use their own.
	Language string `json:"language"`
	// CookOnceEatTwice repeats batch-friendly dinners as the next day's lunch.
	CookOnceEatTwice bool `json:"cook_once_eat_twice"`
}

// DefaultPrefs mirrors the household this add-on was written for: little fish
// and only when breaded, vegetables kept in the background, family portions.
func DefaultPrefs() Prefs {
	return Prefs{
		FishPolicy:       "breaded_only",
		MaxFishPerWeek:   1,
		VeggieLevel:      "low",
		HouseholdSize:    2,
		MealsPerDay:      4,
		MaxCookMinutes:   45,
		Language:         "de",
		CookOnceEatTwice: true,
	}
}

// Profile is one person's setup.
type Profile struct {
	UserID         string  `json:"user_id"`
	DisplayName    string  `json:"display_name"`
	Sex            string  `json:"sex"`
	BirthDate      string  `json:"birth_date"` // YYYY-MM-DD
	HeightCm       float64 `json:"height_cm"`
	StartWeightKg  float64 `json:"start_weight_kg"`
	TargetWeightKg float64 `json:"target_weight_kg"`
	Activity       string  `json:"activity"`
	Goal           string  `json:"goal"`
	Breastfeeding  string  `json:"breastfeeding"`
	Prefs          Prefs   `json:"prefs"`
	SetupDone      bool    `json:"setup_done"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// Age derives the current age in years from the birth date. Returns 30 as a
// neutral fallback when the date is missing or unparseable, so the energy
// calculation still produces something usable.
func (p Profile) Age() int {
	t, err := time.Parse("2006-01-02", p.BirthDate)
	if err != nil {
		return 30
	}
	now := time.Now()
	age := now.Year() - t.Year()
	if now.YearDay() < t.YearDay() {
		age--
	}
	if age < 10 || age > 120 {
		return 30
	}
	return age
}

// EnsureUser creates the user row if this is their first visit.
func (s *Store) EnsureUser(ctx context.Context, id, name string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, name, created_at) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = CASE WHEN excluded.name != '' THEN excluded.name ELSE users.name END`,
		id, name, nowUTC())
	return err
}

// GetProfile loads a profile. Returns ErrNotFound when setup has never run.
func (s *Store) GetProfile(ctx context.Context, userID string) (Profile, error) {
	var p Profile
	var prefsJSON string
	var setup int
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, display_name, sex, birth_date, height_cm, start_weight_kg,
		       target_weight_kg, activity, goal, breastfeeding, prefs_json,
		       setup_done, created_at, updated_at
		  FROM profiles WHERE user_id = ?`, userID).
		Scan(&p.UserID, &p.DisplayName, &p.Sex, &p.BirthDate, &p.HeightCm, &p.StartWeightKg,
			&p.TargetWeightKg, &p.Activity, &p.Goal, &p.Breastfeeding, &prefsJSON,
			&setup, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	p.SetupDone = setup == 1
	p.Prefs = DefaultPrefs()
	if prefsJSON != "" {
		_ = json.Unmarshal([]byte(prefsJSON), &p.Prefs)
	}
	return p, nil
}

// SaveProfile inserts or updates a profile.
func (s *Store) SaveProfile(ctx context.Context, p Profile) error {
	prefsJSON, err := json.Marshal(p.Prefs)
	if err != nil {
		return err
	}
	setup := 0
	if p.SetupDone {
		setup = 1
	}
	now := nowUTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO profiles (user_id, display_name, sex, birth_date, height_cm,
			start_weight_kg, target_weight_kg, activity, goal, breastfeeding,
			prefs_json, setup_done, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(user_id) DO UPDATE SET
			display_name = excluded.display_name,
			sex = excluded.sex,
			birth_date = excluded.birth_date,
			height_cm = excluded.height_cm,
			start_weight_kg = excluded.start_weight_kg,
			target_weight_kg = excluded.target_weight_kg,
			activity = excluded.activity,
			goal = excluded.goal,
			breastfeeding = excluded.breastfeeding,
			prefs_json = excluded.prefs_json,
			setup_done = excluded.setup_done,
			updated_at = excluded.updated_at`,
		p.UserID, p.DisplayName, p.Sex, p.BirthDate, p.HeightCm,
		p.StartWeightKg, p.TargetWeightKg, p.Activity, p.Goal, p.Breastfeeding,
		string(prefsJSON), setup, now, now)
	return err
}

// TrackerLinks returns the Home Assistant entity IDs this user has wired up,
// keyed by kind ("steps", "active_energy", "weight", "heart_rate").
func (s *Store) TrackerLinks(ctx context.Context, userID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT kind, entity_id FROM tracker_links WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, e string
		if err := rows.Scan(&k, &e); err != nil {
			return nil, err
		}
		out[k] = e
	}
	return out, rows.Err()
}

// SetTrackerLink wires a fitness-tracker entity to a data kind. An empty
// entityID removes the link.
func (s *Store) SetTrackerLink(ctx context.Context, userID, kind, entityID string) error {
	if strings.TrimSpace(entityID) == "" {
		_, err := s.db.ExecContext(ctx,
			`DELETE FROM tracker_links WHERE user_id = ? AND kind = ?`, userID, kind)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tracker_links (user_id, kind, entity_id) VALUES (?,?,?)
		 ON CONFLICT(user_id, kind) DO UPDATE SET entity_id = excluded.entity_id`,
		userID, kind, entityID)
	return err
}

// ListUsers returns every known real user id and name, excluding the shared
// household pseudo-user.
func (s *Store) ListUsers(ctx context.Context) ([]struct{ ID, Name string }, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name FROM users WHERE id != ? ORDER BY created_at`, HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []struct{ ID, Name string }
	for rows.Next() {
		var u struct{ ID, Name string }
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// atoiDefault parses n, falling back to def.
func atoiDefault(s string, def int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
