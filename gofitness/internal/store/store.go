// Package store owns the local SQLite database. Everything the app records —
// profiles, weigh-ins, food, workouts, meal plans, achievements — lives here and
// never leaves the machine Home Assistant runs on.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, no cgo needed for multi-arch builds
)

// Store is a handle on the SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (and creates, if missing) the database at path and applies all
// pending migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles one writer at a time; keeping the pool small avoids
	// spurious "database is locked" errors under concurrent requests.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle for the few places that need it (tests, health).
func (s *Store) DB() *sql.DB { return s.db }

// migrations are applied in order; each one runs exactly once. Never edit a
// migration that has shipped — append a new one instead.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL
	);`,

	`CREATE TABLE IF NOT EXISTS profiles (
		user_id          TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
		display_name     TEXT NOT NULL DEFAULT '',
		sex              TEXT NOT NULL DEFAULT 'female',
		birth_date       TEXT NOT NULL DEFAULT '',
		height_cm        REAL NOT NULL DEFAULT 0,
		start_weight_kg  REAL NOT NULL DEFAULT 0,
		target_weight_kg REAL NOT NULL DEFAULT 0,
		activity         TEXT NOT NULL DEFAULT 'light',
		goal             TEXT NOT NULL DEFAULT 'lose',
		breastfeeding    TEXT NOT NULL DEFAULT 'none',
		prefs_json       TEXT NOT NULL DEFAULT '{}',
		setup_done       INTEGER NOT NULL DEFAULT 0,
		created_at       TEXT NOT NULL,
		updated_at       TEXT NOT NULL
	);`,

	`CREATE TABLE IF NOT EXISTS weights (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		weight_kg    REAL NOT NULL,
		body_fat_pct REAL,
		recorded_at  TEXT NOT NULL,
		source       TEXT NOT NULL DEFAULT 'manual',
		note         TEXT NOT NULL DEFAULT ''
	);`,
	`CREATE INDEX IF NOT EXISTS idx_weights_user_time ON weights(user_id, recorded_at DESC);`,

	`CREATE TABLE IF NOT EXISTS food_logs (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		logged_at   TEXT NOT NULL,
		day         TEXT NOT NULL,
		meal_type   TEXT NOT NULL DEFAULT 'snack',
		name        TEXT NOT NULL,
		amount      TEXT NOT NULL DEFAULT '',
		kcal        REAL NOT NULL DEFAULT 0,
		protein_g   REAL NOT NULL DEFAULT 0,
		carbs_g     REAL NOT NULL DEFAULT 0,
		fat_g       REAL NOT NULL DEFAULT 0,
		source      TEXT NOT NULL DEFAULT 'manual',
		recipe_id   TEXT NOT NULL DEFAULT '',
		confidence  TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_food_user_day ON food_logs(user_id, day);`,

	`CREATE TABLE IF NOT EXISTS workouts (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		day        TEXT NOT NULL,
		kind       TEXT NOT NULL,
		minutes    REAL NOT NULL DEFAULT 0,
		kcal       REAL NOT NULL DEFAULT 0,
		steps      INTEGER NOT NULL DEFAULT 0,
		source     TEXT NOT NULL DEFAULT 'manual',
		note       TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_workouts_user_day ON workouts(user_id, day);`,

	`CREATE TABLE IF NOT EXISTS meal_plans (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		week_start  TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		UNIQUE(user_id, week_start)
	);`,

	`CREATE TABLE IF NOT EXISTS meal_plan_entries (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		plan_id   INTEGER NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
		day_index INTEGER NOT NULL,
		meal_type TEXT NOT NULL,
		recipe_id TEXT NOT NULL,
		servings  REAL NOT NULL DEFAULT 1,
		cooked    INTEGER NOT NULL DEFAULT 0
	);`,
	`CREATE INDEX IF NOT EXISTS idx_entries_plan ON meal_plan_entries(plan_id, day_index);`,

	`CREATE TABLE IF NOT EXISTS shopping_items (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		plan_id  INTEGER NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
		name     TEXT NOT NULL,
		amount   REAL NOT NULL DEFAULT 0,
		unit     TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL DEFAULT 'Sonstiges',
		checked  INTEGER NOT NULL DEFAULT 0
	);`,
	`CREATE INDEX IF NOT EXISTS idx_shopping_plan ON shopping_items(plan_id);`,

	`CREATE TABLE IF NOT EXISTS achievements (
		user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		code        TEXT NOT NULL,
		unlocked_at TEXT NOT NULL,
		PRIMARY KEY (user_id, code)
	);`,

	`CREATE TABLE IF NOT EXISTS tracker_links (
		user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		kind      TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		PRIMARY KEY (user_id, kind)
	);`,

	`CREATE TABLE IF NOT EXISTS app_settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`,
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);`); err != nil {
		return err
	}

	var current int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return err
	}

	for i := current; i < len(migrations); i++ {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			i+1, nowUTC()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Setting reads an app-wide setting, returning def when unset.
func (s *Store) Setting(ctx context.Context, key, def string) string {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

// SetSetting writes an app-wide setting.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// Day formats a time as the local calendar day used for grouping logs.
func Day(t time.Time) string { return t.Format("2006-01-02") }
