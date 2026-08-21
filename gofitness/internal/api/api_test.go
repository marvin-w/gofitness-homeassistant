package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/ai"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/config"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/hass"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/recipes"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

// newTestServer builds a server backed by a throwaway database, with the AI
// client and Home Assistant connection disabled — exactly the configuration an
// add-on runs in before the user supplies an API key.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	ctx := t.Context()

	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := config.Default()
	cfg.PublishSensors = false

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(cfg, st, recipes.MustLoad(), ai.New("", ""), hass.New(), log)
}

// do issues a request as a given Home Assistant user.
func do(t *testing.T, s *Server, method, path, userID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader = bytes.NewReader(nil)
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set(hass.HeaderUserID, userID)
		req.Header.Set(hass.HeaderDisplayName, "User "+userID)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return out
}

func validProfile() map[string]any {
	return map[string]any{
		"display_name":     "Anna",
		"sex":              "female",
		"birth_date":       "1992-04-17",
		"height_cm":        168,
		"weight_kg":        78,
		"target_weight_kg": 0,
		"activity":         "light",
		"goal":             "lose",
		"breastfeeding":    "exclusive",
		"prefs": map[string]any{
			"fish_policy": "breaded_only", "max_fish_per_week": 1,
			"veggie_level": "low", "household_size": 2, "meals_per_day": 4,
			"max_cook_minutes": 45, "language": "de", "cook_once_eat_twice": true,
		},
	}
}

func setup(t *testing.T, s *Server, userID string) {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/profile", userID, validProfile())
	if rec.Code != http.StatusOK {
		t.Fatalf("setup failed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/healthz", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := decode(t, rec)
	if body["ok"] != true {
		t.Errorf("health = %v", body)
	}
	if n, _ := body["recipes"].(float64); n < 30 {
		t.Errorf("health reports %v recipes", body["recipes"])
	}
}

func TestBootstrapBeforeSetup(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/bootstrap", "alice", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	if body["setup_done"] != false {
		t.Error("a fresh user should not be marked set up")
	}
	if _, hasPlan := body["plan"]; hasPlan {
		t.Error("no plan should be returned before setup")
	}
	user := body["user"].(map[string]any)
	if user["id"] != "alice" || user["via_ingress"] != true {
		t.Errorf("user = %v", user)
	}
}

func TestSetupAndBootstrap(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	rec := do(t, s, http.MethodGet, "/api/bootstrap", "alice", nil)
	body := decode(t, rec)
	if body["setup_done"] != true {
		t.Fatal("setup flag not persisted")
	}

	plan := body["plan"].(map[string]any)
	target := plan["target_kcal"].(float64)
	// Exclusively breastfeeding: the floor is 2000 kcal.
	if target < 2000 {
		t.Errorf("target %v is below the lactation floor", target)
	}
	if plan["breastfeeding_add_kcal"].(float64) != 500 {
		t.Errorf("lactation surcharge = %v", plan["breastfeeding_add_kcal"])
	}

	// Setup doubles as the first weigh-in.
	if body["current_weight"].(float64) != 78 {
		t.Errorf("current weight = %v, want 78", body["current_weight"])
	}
	if _, ok := body["last_weigh_in"]; !ok {
		t.Error("setup should have recorded a weigh-in")
	}

	// And it unlocks the first badges.
	g := body["gamify"].(map[string]any)
	if g["unlocked_count"].(float64) < 2 {
		t.Errorf("expected setup and weigh-in badges, got %v", g["unlocked_count"])
	}
}

func TestSetupValidation(t *testing.T) {
	s := newTestServer(t)
	cases := []struct {
		name  string
		patch map[string]any
	}{
		{"height in metres", map[string]any{"height_cm": 1.68}},
		{"impossible height", map[string]any{"height_cm": 400}},
		{"weight too low", map[string]any{"weight_kg": 12}},
		{"weight too high", map[string]any{"weight_kg": 900}},
		{"unknown sex", map[string]any{"sex": "yes"}},
		{"unknown goal", map[string]any{"goal": "shred"}},
		{"unknown activity", map[string]any{"activity": "extreme"}},
		{"unknown breastfeeding", map[string]any{"breastfeeding": "maybe"}},
		{"malformed birth date", map[string]any{"birth_date": "17.04.1992"}},
		{"birth date in the future", map[string]any{"birth_date": "2099-01-01"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile()
			for k, v := range tc.patch {
				p[k] = v
			}
			rec := do(t, s, http.MethodPost, "/api/profile", "bob", p)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestProfilePreviewDoesNotPersist(t *testing.T) {
	s := newTestServer(t)

	rec := do(t, s, http.MethodPost, "/api/profile/preview", "carol", validProfile())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	if _, ok := body["plan"]; !ok {
		t.Error("preview returned no plan")
	}
	if ms, ok := body["milestones"].([]any); !ok || len(ms) == 0 {
		t.Error("preview returned no milestones")
	}

	// Nothing may have been written.
	boot := decode(t, do(t, s, http.MethodGet, "/api/bootstrap", "carol", nil))
	if boot["setup_done"] != false {
		t.Error("preview persisted the profile")
	}
}

func TestUsersAreIsolated(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")
	setup(t, s, "bob")

	if rec := do(t, s, http.MethodPost, "/api/weights", "alice",
		map[string]any{"weight_kg": 77.2}); rec.Code != http.StatusOK {
		t.Fatalf("alice weigh-in: %d %s", rec.Code, rec.Body)
	}

	alice := decode(t, do(t, s, http.MethodGet, "/api/weights", "alice", nil))
	bob := decode(t, do(t, s, http.MethodGet, "/api/weights", "bob", nil))

	if len(alice["weights"].([]any)) != 2 { // setup weigh-in plus this one
		t.Errorf("alice has %d weigh-ins, want 2", len(alice["weights"].([]any)))
	}
	if len(bob["weights"].([]any)) != 1 {
		t.Errorf("bob sees %d weigh-ins, want only his own setup entry", len(bob["weights"].([]any)))
	}
}

// Without ingress headers everyone shares the "local" profile. That is expected
// for direct port access, and worth pinning down so it cannot change silently.
func TestNoIngressHeadersUsesLocalUser(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/bootstrap", "", nil)
	body := decode(t, rec)
	user := body["user"].(map[string]any)
	if user["id"] != hass.LocalUserID || user["via_ingress"] != false {
		t.Errorf("user = %v, want the local fallback", user)
	}
}

func TestWeightValidation(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	for _, body := range []map[string]any{
		{"weight_kg": 5},
		{"weight_kg": 500},
		{"weight_kg": 70, "body_fat_pct": 95.0},
		{"weight_kg": 70, "recorded_at": "yesterday"},
	} {
		rec := do(t, s, http.MethodPost, "/api/weights", "alice", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%v: status %d, want 400", body, rec.Code)
		}
	}
}

func TestFoodLifecycle(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	rec := do(t, s, http.MethodPost, "/api/food", "alice", map[string]any{
		"name": "Fischstäbchen", "meal_type": "dinner", "amount": "4 Stück",
		"kcal": 246, "protein_g": 14.4, "carbs_g": 20.4, "fat_g": 11.4,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("add food: %d %s", rec.Code, rec.Body)
	}
	added := decode(t, rec)
	id := int64(added["id"].(float64))
	if totals := added["totals"].(map[string]any); totals["kcal"].(float64) != 246 {
		t.Errorf("totals after add = %v", totals)
	}

	listed := decode(t, do(t, s, http.MethodGet, "/api/food", "alice", nil))
	if len(listed["food"].([]any)) != 1 {
		t.Errorf("expected 1 entry, got %v", listed["food"])
	}

	day := listed["day"].(string)
	del := do(t, s, http.MethodDelete, "/api/food/"+itoa(id)+"?day="+day, "alice", nil)
	if del.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", del.Code, del.Body)
	}
	if totals := decode(t, del)["totals"].(map[string]any); totals["kcal"].(float64) != 0 {
		t.Errorf("totals after delete = %v", totals)
	}
}

func TestFoodValidation(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	for _, body := range []map[string]any{
		{"name": "   ", "kcal": 100},
		{"name": "x", "kcal": -5},
		{"name": "x", "kcal": 99999},
	} {
		rec := do(t, s, http.MethodPost, "/api/food", "alice", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%v: status %d, want 400", body, rec.Code)
		}
	}
}

// With no API key configured, a text estimate must still work from the local
// food table rather than failing.
func TestEstimateFallsBackToLocalTable(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	rec := do(t, s, http.MethodPost, "/api/food/estimate", "alice",
		map[string]any{"text": "2 Kugeln Eis"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	est := decode(t, rec)["estimate"].(map[string]any)
	if est["source"] != "local_db" {
		t.Errorf("source = %v, want local_db", est["source"])
	}
	if est["kcal"].(float64) <= 0 {
		t.Errorf("kcal = %v", est["kcal"])
	}
	if est["confidence"] != "low" {
		t.Errorf("a table lookup should report low confidence, got %v", est["confidence"])
	}
	// Estimating must not log anything by itself.
	listed := decode(t, do(t, s, http.MethodGet, "/api/food", "alice", nil))
	if len(listed["food"].([]any)) != 0 {
		t.Error("estimating silently logged the food")
	}
}

func TestEstimateUnknownFood(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")
	rec := do(t, s, http.MethodPost, "/api/food/estimate", "alice",
		map[string]any{"text": "zzzz nonexistent foodstuff"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}

func TestFoodSearch(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	rec := do(t, s, http.MethodGet, "/api/food/search?q=brot", "alice", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	results := decode(t, rec)["results"].([]any)
	if len(results) == 0 {
		t.Fatal("no results for 'brot'")
	}
	first := results[0].(map[string]any)
	if first["kcal"].(float64) <= 0 || first["portion"] == "" {
		t.Errorf("result = %v", first)
	}
}

func TestWorkoutKcalIsEstimated(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	rec := do(t, s, http.MethodPost, "/api/workouts", "alice",
		map[string]any{"kind": "run", "minutes": 30})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	// 9 MET x 78 kg x 0.5 h ~= 351 kcal.
	kcal := decode(t, rec)["kcal"].(float64)
	if kcal < 250 || kcal > 450 {
		t.Errorf("estimated %v kcal for a 30 min run at 78 kg", kcal)
	}
}

func TestWorkoutValidation(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	for _, body := range []map[string]any{
		{"kind": "", "minutes": 30},
		{"kind": "run", "minutes": 5000},
		{"kind": "run", "minutes": 30, "kcal": -10},
	} {
		if rec := do(t, s, http.MethodPost, "/api/workouts", "alice", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%v: status %d, want 400", body, rec.Code)
		}
	}
}

func TestPlanGenerationAndShoppingList(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	// An unsaved proposal first.
	rec := do(t, s, http.MethodGet, "/api/plan", "alice", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get plan: %d %s", rec.Code, rec.Body)
	}
	proposal := decode(t, rec)
	if proposal["saved"] != false {
		t.Error("first plan should be an unsaved proposal")
	}

	// Then save one.
	rec = do(t, s, http.MethodPost, "/api/plan/generate", "alice", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body)
	}
	saved := decode(t, rec)
	if saved["saved"] != true {
		t.Error("generated plan not marked saved")
	}

	plan := saved["plan"].(map[string]any)
	days := plan["days"].([]any)
	if len(days) != 7 {
		t.Fatalf("plan has %d days", len(days))
	}

	shopping := saved["shopping"].([]any)
	if len(shopping) < 10 {
		t.Errorf("shopping list has only %d lines", len(shopping))
	}

	// Every planned meal must carry an id so it can be ticked off.
	var entryID int64
	for _, d := range days {
		for _, e := range d.(map[string]any)["entries"].([]any) {
			entry := e.(map[string]any)
			if entry["id"].(float64) == 0 {
				t.Fatal("saved plan entry without an id")
			}
			if entryID == 0 {
				entryID = int64(entry["id"].(float64))
			}
		}
	}

	// Logging a planned meal records it as eaten and marks it cooked.
	week := saved["week_start"].(string)
	rec = do(t, s, http.MethodPost, "/api/plan/entries/"+itoa(entryID)+"/log?week="+week,
		"alice", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("log planned meal: %d %s", rec.Code, rec.Body)
	}
	logged := decode(t, rec)
	if logged["totals"].(map[string]any)["kcal"].(float64) <= 0 {
		t.Error("logging a planned meal recorded no calories")
	}

	// The shopping list can be ticked off.
	itemID := int64(shopping[0].(map[string]any)["id"].(float64))
	if rec := do(t, s, http.MethodPost, "/api/shopping/"+itoa(itemID)+"/check", "alice",
		map[string]any{"checked": true}); rec.Code != http.StatusOK {
		t.Fatalf("check item: %d %s", rec.Code, rec.Body)
	}

	reload := decode(t, do(t, s, http.MethodGet, "/api/plan?week="+week, "alice", nil))
	checked := false
	for _, it := range reload["shopping"].([]any) {
		if it.(map[string]any)["checked"] == true {
			checked = true
		}
	}
	if !checked {
		t.Error("ticked shopping item did not persist")
	}
}

func TestPlanRequiresSetup(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/plan", "nobody", nil)
	if rec.Code != http.StatusPreconditionRequired {
		t.Errorf("status %d, want 428", rec.Code)
	}
}

func TestCrossUserPlanAccessIsRejected(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")
	setup(t, s, "bob")

	saved := decode(t, do(t, s, http.MethodPost, "/api/plan/generate", "alice", map[string]any{}))
	itemID := int64(saved["shopping"].([]any)[0].(map[string]any)["id"].(float64))

	rec := do(t, s, http.MethodPost, "/api/shopping/"+itoa(itemID)+"/check", "bob",
		map[string]any{"checked": true})
	if rec.Code != http.StatusNotFound {
		t.Errorf("bob ticking alice's item returned %d, want 404", rec.Code)
	}
}

func TestRecipeLocalisation(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	de := decode(t, do(t, s, http.MethodGet, "/api/recipes/haehnchen-reispfanne?lang=de", "alice", nil))
	en := decode(t, do(t, s, http.MethodGet, "/api/recipes/haehnchen-reispfanne?lang=en", "alice", nil))

	deTitle := de["recipe"].(map[string]any)["title"].(string)
	enTitle := en["recipe"].(map[string]any)["title"].(string)
	if deTitle == enTitle {
		t.Errorf("title not localised: %q", deTitle)
	}
	if deKcal, enKcal := de["recipe"].(map[string]any)["kcal"], en["recipe"].(map[string]any)["kcal"]; deKcal != enKcal {
		t.Errorf("localisation changed the calories: %v vs %v", deKcal, enKcal)
	}
	if len(en["scaled_ingredients"].([]any)) == 0 {
		t.Error("no scaled ingredients returned")
	}
}

func TestRecipeNotFound(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")
	rec := do(t, s, http.MethodGet, "/api/recipes/does-not-exist", "alice", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}

func TestRecipeListRespectsPreferences(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice") // low veg, breaded fish only, breastfeeding

	body := decode(t, do(t, s, http.MethodGet, "/api/recipes", "alice", nil))
	list := body["recipes"].([]any)
	if len(list) == 0 {
		t.Fatal("no recipes matched the default preferences")
	}
	for _, r := range list {
		rec := r.(map[string]any)
		if rec["veggie_level"] != "low" {
			t.Errorf("%v has veggie level %v despite a low preference", rec["id"], rec["veggie_level"])
		}
		if rec["contains_fish"] == true && rec["fish_breaded"] != true {
			t.Errorf("%v is unbreaded fish", rec["id"])
		}
		if rec["breastfeeding_safe"] != true {
			t.Errorf("%v is not breastfeeding-safe", rec["id"])
		}
	}

	// The browser can opt out of the filters.
	all := decode(t, do(t, s, http.MethodGet, "/api/recipes?all=1", "alice", nil))
	if len(all["recipes"].([]any)) <= len(list) {
		t.Error("the unfiltered list should be larger")
	}
}

func TestStatsSeries(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	for _, kg := range []float64{77.5, 77.0, 76.4} {
		if rec := do(t, s, http.MethodPost, "/api/weights", "alice",
			map[string]any{"weight_kg": kg}); rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
	}

	body := decode(t, do(t, s, http.MethodGet, "/api/stats?days=30", "alice", nil))
	weights := body["weights"].([]any)
	if len(weights) != 4 {
		t.Fatalf("got %d weight points, want 4", len(weights))
	}
	// Chronological order matters for the chart.
	first := weights[0].(map[string]any)["weight"].(float64)
	last := weights[len(weights)-1].(map[string]any)["weight"].(float64)
	if first != 78 || last != 76.4 {
		t.Errorf("series runs %v → %v, want 78 → 76.4", first, last)
	}
	if len(body["weight_trend"].([]any)) != 4 {
		t.Errorf("trend has %d points", len(body["weight_trend"].([]any)))
	}
	rangeVals := body["healthy_range"].([]any)
	if rangeVals[0].(float64) >= rangeVals[1].(float64) {
		t.Errorf("healthy range = %v", rangeVals)
	}
}

func TestGamifyProgresses(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	before := decode(t, do(t, s, http.MethodGet, "/api/gamify", "alice", nil))["gamify"].(map[string]any)

	for i := 0; i < 3; i++ {
		do(t, s, http.MethodPost, "/api/food", "alice",
			map[string]any{"name": "Snack", "kcal": 200})
	}

	after := decode(t, do(t, s, http.MethodGet, "/api/gamify", "alice", nil))["gamify"].(map[string]any)
	if after["xp"].(float64) <= before["xp"].(float64) {
		t.Errorf("XP did not increase: %v → %v", before["xp"], after["xp"])
	}
	if after["current_streak"].(float64) < 1 {
		t.Errorf("streak = %v, want at least 1", after["current_streak"])
	}
	badges := after["badges"].([]any)
	if len(badges) == 0 {
		t.Fatal("no badges returned")
	}
	// Unlocked badges sort first.
	if badges[0].(map[string]any)["unlocked"] != true {
		t.Error("unlocked badges should sort to the front")
	}
}

func TestTrackerSyncWithoutHomeAssistant(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	rec := do(t, s, http.MethodPost, "/api/trackers/sync", "alice", nil)
	if rec.Code != http.StatusPreconditionFailed {
		t.Errorf("status %d, want 412 outside the supervisor", rec.Code)
	}

	// Listing still works and reports the connection as unavailable.
	body := decode(t, do(t, s, http.MethodGet, "/api/trackers", "alice", nil))
	if body["available"] != false {
		t.Errorf("available = %v", body["available"])
	}
	if len(body["kinds"].([]any)) == 0 {
		t.Error("no tracker kinds offered")
	}
}

func TestTrackerLinkValidation(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	if rec := do(t, s, http.MethodPost, "/api/trackers", "alice",
		map[string]any{"kind": "mood", "entity_id": "sensor.x"}); rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400 for an unknown kind", rec.Code)
	}
	if rec := do(t, s, http.MethodPost, "/api/trackers", "alice",
		map[string]any{"kind": "steps", "entity_id": "sensor.x"}); rec.Code != http.StatusOK {
		t.Errorf("status %d for a valid link", rec.Code)
	}
	body := decode(t, do(t, s, http.MethodGet, "/api/trackers", "alice", nil))
	if body["links"].(map[string]any)["steps"] != "sensor.x" {
		t.Errorf("links = %v", body["links"])
	}
}

func TestMalformedJSONIsRejected(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/weights", bytes.NewReader([]byte("{oops")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(hass.HeaderUserID, "alice")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", rec.Code)
	}
}

func TestIndexInjectsIngressBase(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(hass.HeaderIngressPath, "/api/hassio_ingress/abc123")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte(`<base href="/api/hassio_ingress/abc123/">`)) {
		t.Errorf("ingress base not injected:\n%s", firstLines(body, 12))
	}
	if bytes.Contains([]byte(body), []byte("__BASE_HREF__")) {
		t.Error("base placeholder left in the page")
	}
}

func TestIndexWithoutIngress(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`<base href="./">`)) {
		t.Error("expected a relative base href when not behind ingress")
	}
}

func TestStaticAssetsAreServed(t *testing.T) {
	s := newTestServer(t)
	for _, path := range []string{"/css/app.css", "/js/app.js", "/js/i18n.js", "/js/api.js", "/js/ui.js"} {
		rec := do(t, s, http.MethodGet, path, "", nil)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", path, rec.Code)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("%s: empty", path)
		}
	}
}

// An unknown path must fall through to the app shell so client-side navigation
// keeps working after a reload.
func TestUnknownPathServesTheApp(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/some/deep/link", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("<div id=\"app\">")) {
		t.Error("expected the app shell")
	}
}

func TestLanguageSelection(t *testing.T) {
	s := newTestServer(t)
	setup(t, s, "alice")

	// Query parameter wins.
	en := decode(t, do(t, s, http.MethodGet, "/api/bootstrap?lang=en", "alice", nil))
	if en["lang"] != "en" {
		t.Errorf("lang = %v, want en", en["lang"])
	}
	// The stored profile preference (German) applies by default.
	de := decode(t, do(t, s, http.MethodGet, "/api/bootstrap", "alice", nil))
	if de["lang"] != "de" {
		t.Errorf("lang = %v, want de", de["lang"])
	}
}

func TestWeekStartSnapsToMonday(t *testing.T) {
	// 2026-08-21 is a Friday; the week must start on Monday the 17th.
	friday := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	if got := weekStart(friday).Format("2006-01-02"); got != "2026-08-17" {
		t.Errorf("weekStart(Friday) = %s, want 2026-08-17", got)
	}
	monday := time.Date(2026, 8, 17, 0, 30, 0, 0, time.UTC)
	if got := weekStart(monday).Format("2006-01-02"); got != "2026-08-17" {
		t.Errorf("weekStart(Monday) = %s, want the same day", got)
	}
	sunday := time.Date(2026, 8, 23, 23, 0, 0, 0, time.UTC)
	if got := weekStart(sunday).Format("2006-01-02"); got != "2026-08-17" {
		t.Errorf("weekStart(Sunday) = %s, want 2026-08-17", got)
	}
}

func TestDetectImageType(t *testing.T) {
	cases := []struct {
		name     string
		data     []byte
		declared string
		want     string
	}{
		{"jpeg magic", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "", "image/jpeg"},
		{"png magic", append([]byte{0x89}, []byte("PNG\r\n\x1a\n")...), "", "image/png"},
		{"gif magic", []byte("GIF89a...."), "", "image/gif"},
		{"declared beats nothing", []byte{0, 1, 2, 3}, "image/png", "image/png"},
		{"magic beats a wrong header", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/png", "image/jpeg"},
		{"unsupported", []byte{0, 1, 2, 3}, "application/pdf", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectImageType(tc.data, tc.declared); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEstimateWorkoutKcal(t *testing.T) {
	// 9 MET x 70 kg x 1 h = 630 kcal.
	if got := estimateWorkoutKcal("run", 60, 70); got != 630 {
		t.Errorf("run = %v, want 630", got)
	}
	// An unknown kind falls back to the "other" MET value rather than zero.
	if got := estimateWorkoutKcal("quidditch", 60, 70); got <= 0 {
		t.Errorf("unknown kind = %v, want a positive fallback", got)
	}
	// A missing weight must not produce zero either.
	if got := estimateWorkoutKcal("walk", 60, 0); got <= 0 {
		t.Errorf("missing weight = %v", got)
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func firstLines(s string, n int) string {
	count, i := 0, 0
	for ; i < len(s); i++ {
		if s[i] == '\n' {
			count++
			if count == n {
				break
			}
		}
	}
	return s[:i]
}
