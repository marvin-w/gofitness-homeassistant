package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// WeightEntry is a single weigh-in.
type WeightEntry struct {
	ID         int64    `json:"id"`
	WeightKg   float64  `json:"weight_kg"`
	BodyFatPct *float64 `json:"body_fat_pct,omitempty"`
	RecordedAt string   `json:"recorded_at"`
	Source     string   `json:"source"`
	Note       string   `json:"note"`
}

// AddWeight records a weigh-in and returns its id.
func (s *Store) AddWeight(ctx context.Context, userID string, e WeightEntry) (int64, error) {
	if e.RecordedAt == "" {
		e.RecordedAt = nowUTC()
	}
	if e.Source == "" {
		e.Source = "manual"
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO weights (user_id, weight_kg, body_fat_pct, recorded_at, source, note)
		VALUES (?,?,?,?,?,?)`,
		userID, e.WeightKg, e.BodyFatPct, e.RecordedAt, e.Source, e.Note)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeleteWeight removes a weigh-in belonging to userID.
func (s *Store) DeleteWeight(ctx context.Context, userID string, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM weights WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// Weights returns the most recent weigh-ins, newest first, capped at limit.
func (s *Store) Weights(ctx context.Context, userID string, limit int) ([]WeightEntry, error) {
	if limit <= 0 {
		limit = 365
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, weight_kg, body_fat_pct, recorded_at, source, note
		  FROM weights WHERE user_id = ?
		 ORDER BY recorded_at DESC, id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []WeightEntry{}
	for rows.Next() {
		var e WeightEntry
		var bf sql.NullFloat64
		if err := rows.Scan(&e.ID, &e.WeightKg, &bf, &e.RecordedAt, &e.Source, &e.Note); err != nil {
			return nil, err
		}
		if bf.Valid {
			v := bf.Float64
			e.BodyFatPct = &v
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LatestWeight returns the newest weigh-in, or ErrNotFound.
func (s *Store) LatestWeight(ctx context.Context, userID string) (WeightEntry, error) {
	list, err := s.Weights(ctx, userID, 1)
	if err != nil {
		return WeightEntry{}, err
	}
	if len(list) == 0 {
		return WeightEntry{}, ErrNotFound
	}
	return list[0], nil
}

// FoodEntry is one logged item of food.
type FoodEntry struct {
	ID         int64   `json:"id"`
	LoggedAt   string  `json:"logged_at"`
	Day        string  `json:"day"`
	MealType   string  `json:"meal_type"`
	Name       string  `json:"name"`
	Amount     string  `json:"amount"`
	Kcal       float64 `json:"kcal"`
	ProteinG   float64 `json:"protein_g"`
	CarbsG     float64 `json:"carbs_g"`
	FatG       float64 `json:"fat_g"`
	Source     string  `json:"source"`
	RecipeID   string  `json:"recipe_id"`
	Confidence string  `json:"confidence"`
}

// AddFood records an eaten item.
func (s *Store) AddFood(ctx context.Context, userID string, e FoodEntry) (int64, error) {
	if e.LoggedAt == "" {
		e.LoggedAt = nowUTC()
	}
	if e.Day == "" {
		e.Day = Day(time.Now())
	}
	if e.MealType == "" {
		e.MealType = "snack"
	}
	if e.Source == "" {
		e.Source = "manual"
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO food_logs (user_id, logged_at, day, meal_type, name, amount,
			kcal, protein_g, carbs_g, fat_g, source, recipe_id, confidence, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		userID, e.LoggedAt, e.Day, e.MealType, e.Name, e.Amount,
		e.Kcal, e.ProteinG, e.CarbsG, e.FatG, e.Source, e.RecipeID, e.Confidence, nowUTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeleteFood removes a food entry belonging to userID.
func (s *Store) DeleteFood(ctx context.Context, userID string, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM food_logs WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// FoodByDay returns everything logged on a calendar day, in the order eaten.
func (s *Store) FoodByDay(ctx context.Context, userID, day string) ([]FoodEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, logged_at, day, meal_type, name, amount, kcal, protein_g,
		       carbs_g, fat_g, source, recipe_id, confidence
		  FROM food_logs WHERE user_id = ? AND day = ?
		 ORDER BY logged_at, id`, userID, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []FoodEntry{}
	for rows.Next() {
		var e FoodEntry
		if err := rows.Scan(&e.ID, &e.LoggedAt, &e.Day, &e.MealType, &e.Name, &e.Amount,
			&e.Kcal, &e.ProteinG, &e.CarbsG, &e.FatG, &e.Source, &e.RecipeID, &e.Confidence); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DayTotals is the sum of everything eaten and burned on one day.
type DayTotals struct {
	Day         string  `json:"day"`
	Kcal        float64 `json:"kcal"`
	ProteinG    float64 `json:"protein_g"`
	CarbsG      float64 `json:"carbs_g"`
	FatG        float64 `json:"fat_g"`
	Entries     int     `json:"entries"`
	WorkoutKcal float64 `json:"workout_kcal"`
	WorkoutMin  float64 `json:"workout_minutes"`
	Steps       int     `json:"steps"`
}

// TotalsForDay aggregates food and workouts for one day.
func (s *Store) TotalsForDay(ctx context.Context, userID, day string) (DayTotals, error) {
	t := DayTotals{Day: day}
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(kcal),0), COALESCE(SUM(protein_g),0),
		       COALESCE(SUM(carbs_g),0), COALESCE(SUM(fat_g),0), COUNT(*)
		  FROM food_logs WHERE user_id = ? AND day = ?`, userID, day).
		Scan(&t.Kcal, &t.ProteinG, &t.CarbsG, &t.FatG, &t.Entries)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return t, err
	}
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(kcal),0), COALESCE(SUM(minutes),0), COALESCE(SUM(steps),0)
		  FROM workouts WHERE user_id = ? AND day = ?`, userID, day).
		Scan(&t.WorkoutKcal, &t.WorkoutMin, &t.Steps)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return t, err
	}
	return t, nil
}

// TotalsRange returns per-day totals from `from` to `to` inclusive, oldest
// first. Days with no data are omitted.
func (s *Store) TotalsRange(ctx context.Context, userID, from, to string) ([]DayTotals, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT day, COALESCE(SUM(kcal),0), COALESCE(SUM(protein_g),0),
		       COALESCE(SUM(carbs_g),0), COALESCE(SUM(fat_g),0), COUNT(*)
		  FROM food_logs WHERE user_id = ? AND day BETWEEN ? AND ?
		 GROUP BY day ORDER BY day`, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []DayTotals{}
	for rows.Next() {
		var t DayTotals
		if err := rows.Scan(&t.Day, &t.Kcal, &t.ProteinG, &t.CarbsG, &t.FatG, &t.Entries); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Workout is a logged training session or a day of tracker activity.
type Workout struct {
	ID      int64   `json:"id"`
	Day     string  `json:"day"`
	Kind    string  `json:"kind"`
	Minutes float64 `json:"minutes"`
	Kcal    float64 `json:"kcal"`
	Steps   int     `json:"steps"`
	Source  string  `json:"source"`
	Note    string  `json:"note"`
}

// AddWorkout records a training session.
func (s *Store) AddWorkout(ctx context.Context, userID string, w Workout) (int64, error) {
	if w.Day == "" {
		w.Day = Day(time.Now())
	}
	if w.Source == "" {
		w.Source = "manual"
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO workouts (user_id, day, kind, minutes, kcal, steps, source, note, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		userID, w.Day, w.Kind, w.Minutes, w.Kcal, w.Steps, w.Source, w.Note, nowUTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpsertTrackerWorkout stores a day of tracker data, replacing any previous
// import for the same day and source so repeated syncs do not double-count.
func (s *Store) UpsertTrackerWorkout(ctx context.Context, userID string, w Workout) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM workouts WHERE user_id = ? AND day = ? AND source = ?`,
		userID, w.Day, w.Source); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workouts (user_id, day, kind, minutes, kcal, steps, source, note, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		userID, w.Day, w.Kind, w.Minutes, w.Kcal, w.Steps, w.Source, w.Note, nowUTC()); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteWorkout removes a workout belonging to userID.
func (s *Store) DeleteWorkout(ctx context.Context, userID string, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM workouts WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// WorkoutsByDay lists a day's training.
func (s *Store) WorkoutsByDay(ctx context.Context, userID, day string) ([]Workout, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, day, kind, minutes, kcal, steps, source, note
		  FROM workouts WHERE user_id = ? AND day = ? ORDER BY id`, userID, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Workout{}
	for rows.Next() {
		var w Workout
		if err := rows.Scan(&w.ID, &w.Day, &w.Kind, &w.Minutes, &w.Kcal, &w.Steps, &w.Source, &w.Note); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ActiveDays returns the distinct days between from and to (inclusive) on which
// the user logged anything at all. Used for streaks.
func (s *Store) ActiveDays(ctx context.Context, userID, from, to string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, q := range []string{
		`SELECT DISTINCT day FROM food_logs WHERE user_id = ? AND day BETWEEN ? AND ?`,
		`SELECT DISTINCT day FROM workouts WHERE user_id = ? AND day BETWEEN ? AND ?`,
		`SELECT DISTINCT substr(recorded_at, 1, 10) FROM weights WHERE user_id = ? AND substr(recorded_at, 1, 10) BETWEEN ? AND ?`,
	} {
		rows, err := s.db.QueryContext(ctx, q, userID, from, to)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err != nil {
				rows.Close()
				return nil, err
			}
			out[d] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// WeighInDays returns the distinct days on which a weight was recorded.
func (s *Store) WeighInDays(ctx context.Context, userID, from, to string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT substr(recorded_at, 1, 10) FROM weights
		  WHERE user_id = ? AND substr(recorded_at, 1, 10) BETWEEN ? AND ?`,
		userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d] = true
	}
	return out, rows.Err()
}

// CountFoodEntries returns how many items the user has ever logged.
func (s *Store) CountFoodEntries(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM food_logs WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}

// CountWorkouts returns how many sessions the user has ever logged by hand.
func (s *Store) CountWorkouts(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workouts WHERE user_id = ? AND source = 'manual'`, userID).Scan(&n)
	return n, err
}
