package store

import (
	"context"
	"database/sql"
	"errors"
)

// PlanEntry is a persisted meal-plan slot.
type PlanEntry struct {
	ID       int64   `json:"id"`
	DayIndex int     `json:"day_index"`
	MealType string  `json:"meal_type"`
	RecipeID string  `json:"recipe_id"`
	Servings float64 `json:"servings"`
	Cooked   bool    `json:"cooked"`
}

// ShoppingItem is a persisted shopping-list line.
type ShoppingItem struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Unit     string  `json:"unit"`
	Category string  `json:"category"`
	Checked  bool    `json:"checked"`
}

// SavePlan replaces the plan for a week, together with its shopping list, in a
// single transaction so a half-written plan can never be observed.
func (s *Store) SavePlan(ctx context.Context, userID, weekStart string, entries []PlanEntry, items []ShoppingItem) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM meal_plans WHERE user_id = ? AND week_start = ?`, userID, weekStart); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO meal_plans (user_id, week_start, created_at) VALUES (?,?,?)`,
		userID, weekStart, nowUTC())
	if err != nil {
		return 0, err
	}
	planID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, e := range entries {
		cooked := 0
		if e.Cooked {
			cooked = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO meal_plan_entries (plan_id, day_index, meal_type, recipe_id, servings, cooked)
			 VALUES (?,?,?,?,?,?)`,
			planID, e.DayIndex, e.MealType, e.RecipeID, e.Servings, cooked); err != nil {
			return 0, err
		}
	}
	for _, it := range items {
		checked := 0
		if it.Checked {
			checked = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO shopping_items (plan_id, name, amount, unit, category, checked)
			 VALUES (?,?,?,?,?,?)`,
			planID, it.Name, it.Amount, it.Unit, it.Category, checked); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return planID, nil
}

// PlanID looks up the stored plan id for a week.
func (s *Store) PlanID(ctx context.Context, userID, weekStart string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM meal_plans WHERE user_id = ? AND week_start = ?`, userID, weekStart).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// PlanEntries loads the slots of a stored plan.
func (s *Store) PlanEntries(ctx context.Context, planID int64) ([]PlanEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, day_index, meal_type, recipe_id, servings, cooked
		   FROM meal_plan_entries WHERE plan_id = ? ORDER BY day_index, id`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []PlanEntry{}
	for rows.Next() {
		var e PlanEntry
		var cooked int
		if err := rows.Scan(&e.ID, &e.DayIndex, &e.MealType, &e.RecipeID, &e.Servings, &cooked); err != nil {
			return nil, err
		}
		e.Cooked = cooked == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

// ShoppingItems loads the shopping list of a stored plan.
func (s *Store) ShoppingItems(ctx context.Context, planID int64) ([]ShoppingItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, amount, unit, category, checked
		   FROM shopping_items WHERE plan_id = ? ORDER BY id`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ShoppingItem{}
	for rows.Next() {
		var it ShoppingItem
		var checked int
		if err := rows.Scan(&it.ID, &it.Name, &it.Amount, &it.Unit, &it.Category, &checked); err != nil {
			return nil, err
		}
		it.Checked = checked == 1
		out = append(out, it)
	}
	return out, rows.Err()
}

// SetShoppingChecked ticks or unticks a shopping-list line, verifying it
// belongs to the requesting user.
func (s *Store) SetShoppingChecked(ctx context.Context, userID string, itemID int64, checked bool) error {
	v := 0
	if checked {
		v = 1
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE shopping_items SET checked = ?
		 WHERE id = ? AND plan_id IN (SELECT id FROM meal_plans WHERE user_id = ?)`,
		v, itemID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetEntryCooked marks a planned meal as cooked, verifying ownership.
func (s *Store) SetEntryCooked(ctx context.Context, userID string, entryID int64, cooked bool) error {
	v := 0
	if cooked {
		v = 1
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE meal_plan_entries SET cooked = ?
		 WHERE id = ? AND plan_id IN (SELECT id FROM meal_plans WHERE user_id = ?)`,
		v, entryID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountCookedMeals returns how many planned meals the household has ticked off.
// The plan is shared, so this counts across the whole household.
func (s *Store) CountCookedMeals(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM meal_plan_entries
		 WHERE cooked = 1 AND plan_id IN (SELECT id FROM meal_plans WHERE user_id = ?)`,
		HouseholdID).Scan(&n)
	return n, err
}

// CountPlans returns how many weeks the household has planned.
func (s *Store) CountPlans(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM meal_plans WHERE user_id = ?`, HouseholdID).Scan(&n)
	return n, err
}

// UnlockAchievement records an achievement, ignoring repeats. It reports
// whether this call was the one that unlocked it.
func (s *Store) UnlockAchievement(ctx context.Context, userID, code string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO achievements (user_id, code, unlocked_at) VALUES (?,?,?)`,
		userID, code, nowUTC())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// UnlockedAchievements returns code -> unlock timestamp.
func (s *Store) UnlockedAchievements(ctx context.Context, userID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT code, unlocked_at FROM achievements WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var c, t string
		if err := rows.Scan(&c, &t); err != nil {
			return nil, err
		}
		out[c] = t
	}
	return out, rows.Err()
}
