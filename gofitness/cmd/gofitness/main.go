// Command gofitness runs the GoFitness Home Assistant add-on: a local-first
// fitness and meal-planning app for the whole household.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/ai"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/api"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/config"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/hass"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/recipes"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	log := newLogger(cfg.LogLevel)

	log.Info("starting gofitness",
		"version", version,
		"port", cfg.Port,
		"data_dir", cfg.DataDir,
		"language", cfg.DefaultLang)

	book, err := recipes.Load()
	if err != nil {
		return fmt.Errorf("load recipes: %w", err)
	}
	if missing := book.MissingTranslations(); len(missing) > 0 {
		// Not fatal: an untranslated recipe falls back to German rather than
		// disappearing, but it is worth flagging in the log.
		log.Warn("recipes without english text", "ids", missing)
	}
	log.Info("recipes loaded", "count", book.Len())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	db, err := store.Open(ctx, cfg.DBPath())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	log.Info("database ready", "path", cfg.DBPath())

	aiClient := ai.New(cfg.AnthropicAPIKey, cfg.AIModel)
	if aiClient.Enabled() {
		log.Info("ai calorie estimation enabled", "model", aiClient.Model())
	} else {
		log.Info("ai calorie estimation disabled, using local food table only")
	}

	ha := hass.New()
	if ha.Enabled() {
		log.Info("home assistant api available")
	} else {
		log.Info("home assistant api unavailable (running outside the supervisor?)")
	}

	srv := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler:           api.New(cfg, db, book, aiClient, ha, log),
		ReadHeaderTimeout: 10 * time.Second,
		// Photo uploads and AI round trips are slow by nature, so the write
		// timeout has to be generous enough for a vision request to finish.
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 3 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("stopped cleanly")
	return nil
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
