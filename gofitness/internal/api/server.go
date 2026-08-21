// Package api exposes the HTTP interface: a small JSON API plus the embedded
// single-page frontend.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/ai"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/config"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/hass"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/nutrition"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/recipes"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

// Server wires the dependencies together and serves HTTP.
type Server struct {
	cfg   config.Config
	store *store.Store
	book  *recipes.Book
	ai    *ai.Client
	ha    *hass.Client
	log   *slog.Logger
	mux   *http.ServeMux
}

// New builds the server and registers every route.
func New(cfg config.Config, st *store.Store, book *recipes.Book, aiClient *ai.Client, ha *hass.Client, log *slog.Logger) *Server {
	s := &Server{
		cfg:   cfg,
		store: st,
		book:  book,
		ai:    aiClient,
		ha:    ha,
		log:   log,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	m := s.mux

	m.HandleFunc("GET /healthz", s.handleHealth)

	m.HandleFunc("GET /api/bootstrap", s.wrap(s.handleBootstrap))
	m.HandleFunc("POST /api/profile", s.wrap(s.handleSaveProfile))
	m.HandleFunc("POST /api/profile/preview", s.wrap(s.handlePreviewProfile))

	m.HandleFunc("GET /api/weights", s.wrap(s.handleListWeights))
	m.HandleFunc("POST /api/weights", s.wrap(s.handleAddWeight))
	m.HandleFunc("DELETE /api/weights/{id}", s.wrap(s.handleDeleteWeight))

	m.HandleFunc("GET /api/food", s.wrap(s.handleListFood))
	m.HandleFunc("POST /api/food", s.wrap(s.handleAddFood))
	m.HandleFunc("DELETE /api/food/{id}", s.wrap(s.handleDeleteFood))
	m.HandleFunc("POST /api/food/estimate", s.wrap(s.handleEstimateFood))
	m.HandleFunc("GET /api/food/search", s.wrap(s.handleSearchFood))

	m.HandleFunc("GET /api/workouts", s.wrap(s.handleListWorkouts))
	m.HandleFunc("POST /api/workouts", s.wrap(s.handleAddWorkout))
	m.HandleFunc("DELETE /api/workouts/{id}", s.wrap(s.handleDeleteWorkout))

	m.HandleFunc("GET /api/recipes", s.wrap(s.handleListRecipes))
	m.HandleFunc("GET /api/recipes/{id}", s.wrap(s.handleGetRecipe))

	m.HandleFunc("GET /api/plan", s.wrap(s.handleGetPlan))
	m.HandleFunc("POST /api/plan/generate", s.wrap(s.handleGeneratePlan))
	m.HandleFunc("POST /api/plan/entries/{id}/cooked", s.wrap(s.handleEntryCooked))
	m.HandleFunc("POST /api/plan/entries/{id}/log", s.wrap(s.handleLogPlannedMeal))
	m.HandleFunc("POST /api/shopping/{id}/check", s.wrap(s.handleCheckShopping))

	m.HandleFunc("GET /api/stats", s.wrap(s.handleStats))
	m.HandleFunc("GET /api/gamify", s.wrap(s.handleGamify))

	m.HandleFunc("GET /api/trackers", s.wrap(s.handleGetTrackers))
	m.HandleFunc("POST /api/trackers", s.wrap(s.handleSetTracker))
	m.HandleFunc("POST /api/trackers/sync", s.wrap(s.handleSyncTrackers))

	s.registerStatic(m)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(rec, r)
	if strings.HasPrefix(r.URL.Path, "/api/") {
		s.log.Debug("request",
			"method", r.Method, "path", r.URL.Path,
			"status", rec.status, "ms", time.Since(start).Milliseconds())
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// reqCtx carries the per-request identity and language.
type reqCtx struct {
	User store.Profile
	HA   hass.User
	Lang string
	Now  time.Time
}

// handlerFunc is an API handler that can fail.
type handlerFunc func(w http.ResponseWriter, r *http.Request, rc *reqCtx) error

// apiError is an error with an HTTP status attached.
type apiError struct {
	Status int
	Msg    string
	Err    error
}

func (e apiError) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func badRequest(msg string) error { return apiError{Status: http.StatusBadRequest, Msg: msg} }
func notFound(msg string) error   { return apiError{Status: http.StatusNotFound, Msg: msg} }

// wrap handles identity resolution, JSON error rendering and panics.
func (s *Server) wrap(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		haUser := hass.UserFromRequest(r)

		if err := s.store.EnsureUser(ctx, haUser.ID, haUser.Name); err != nil {
			s.fail(w, r, err)
			return
		}

		prof, err := s.store.GetProfile(ctx, haUser.ID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			s.fail(w, r, err)
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			// A user who has never run setup still gets a usable context; the
			// frontend shows the setup wizard rather than an error.
			prof = store.Profile{
				UserID:      haUser.ID,
				DisplayName: haUser.Name,
				Prefs:       store.DefaultPrefs(),
			}
		}

		// The meal-planning preferences are shared by the whole household, so the
		// global settings are overlaid on top of this profile. Only the interface
		// language stays personal.
		if hp, err := s.store.HouseholdPrefs(ctx); err == nil {
			lang := prof.Prefs.Language
			prof.Prefs = hp
			prof.Prefs.Language = lang
		}

		rc := &reqCtx{
			User: prof,
			HA:   haUser,
			Lang: s.langFor(r, prof),
			Now:  time.Now(),
		}

		if err := h(w, r, rc); err != nil {
			s.fail(w, r, err)
		}
	}
}

// langFor resolves the response language: explicit query parameter first, then
// the stored profile preference, then the add-on default.
func (s *Server) langFor(r *http.Request, prof store.Profile) string {
	if v := r.URL.Query().Get("lang"); v != "" {
		return recipes.NormalizeLang(v)
	}
	if v := r.Header.Get("X-GoFitness-Lang"); v != "" {
		return recipes.NormalizeLang(v)
	}
	if prof.Prefs.Language != "" {
		return recipes.NormalizeLang(prof.Prefs.Language)
	}
	return recipes.NormalizeLang(s.cfg.DefaultLang)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ae apiError
	status := http.StatusInternalServerError
	msg := "internal error"
	if errors.As(err, &ae) {
		status = ae.Status
		msg = ae.Msg
	} else if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
		msg = "not found"
	}
	if status >= 500 {
		s.log.Error("request failed", "path", r.URL.Path, "err", err)
	} else {
		s.log.Debug("request rejected", "path", r.URL.Path, "status", status, "err", err)
	}
	writeJSON(w, status, map[string]any{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status line is already sent; there is nothing useful left to do.
		return
	}
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return badRequest("invalid JSON body")
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DB().PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"recipes": s.book.Len(),
		"ai":      s.ai.Enabled(),
		"hass":    s.ha.Enabled(),
	})
}

// pathInt reads an integer path parameter.
func pathInt(r *http.Request, name string) (int64, error) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		return 0, badRequest("invalid " + name)
	}
	return v, nil
}

// queryDay reads a YYYY-MM-DD query parameter, defaulting to today.
func queryDay(r *http.Request, name string, now time.Time) string {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		return store.Day(now)
	}
	if _, err := time.Parse("2006-01-02", v); err != nil {
		return store.Day(now)
	}
	return v
}

// nutritionProfile converts a stored profile plus the latest weigh-in into the
// input the nutrition package expects.
func (s *Server) nutritionProfile(ctx context.Context, prof store.Profile) nutrition.Profile {
	weight := prof.StartWeightKg
	if w, err := s.store.LatestWeight(ctx, prof.UserID); err == nil {
		weight = w.WeightKg
	}
	return nutrition.Profile{
		Sex:           nutrition.Sex(prof.Sex),
		Age:           prof.Age(),
		HeightCm:      prof.HeightCm,
		WeightKg:      weight,
		Activity:      nutrition.Activity(prof.Activity),
		Goal:          nutrition.Goal(prof.Goal),
		Breastfeeding: nutrition.Breastfeeding(prof.Breastfeeding),
		TargetWeight:  prof.TargetWeightKg,
	}
}
