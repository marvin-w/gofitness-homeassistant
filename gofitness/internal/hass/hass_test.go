package hass

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserFromIngressHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderUserID, "abc-123")
	req.Header.Set(HeaderUserName, "marvin")
	req.Header.Set(HeaderDisplayName, "Marvin W.")
	req.Header.Set(HeaderIngressPath, "/api/hassio_ingress/tok/")

	u := UserFromRequest(req)
	if u.ID != "abc-123" || !u.ViaIngress {
		t.Errorf("user = %+v", u)
	}
	if u.Name != "Marvin W." {
		t.Errorf("display name should win: %q", u.Name)
	}
	if u.IngressPath != "/api/hassio_ingress/tok" {
		t.Errorf("ingress path = %q, want it without the trailing slash", u.IngressPath)
	}
}

func TestUserFallsBackToUserName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderUserID, "abc")
	req.Header.Set(HeaderUserName, "marvin")

	if u := UserFromRequest(req); u.Name != "marvin" {
		t.Errorf("name = %q, want the username fallback", u.Name)
	}
}

func TestUserIDFallsBackToItself(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderUserID, "abc")

	if u := UserFromRequest(req); u.Name != "abc" {
		t.Errorf("name = %q, want the id as a last resort", u.Name)
	}
}

func TestLocalUserWithoutHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	u := UserFromRequest(req)
	if u.ID != LocalUserID || u.ViaIngress {
		t.Errorf("user = %+v, want the local fallback", u)
	}
}

func TestStateFloat(t *testing.T) {
	cases := []struct {
		state string
		want  float64
		ok    bool
	}{
		{"1234", 1234, true},
		{"78.5", 78.5, true},
		{" 42 ", 42, true},
		{"unknown", 0, false},
		{"unavailable", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, ok := State{State: tc.state}.Float()
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("Float(%q) = %v,%v want %v,%v", tc.state, got, ok, tc.want, tc.ok)
		}
	}
}

func TestStateAttributes(t *testing.T) {
	s := State{
		EntityID: "sensor.x",
		Attributes: map[string]any{
			"friendly_name":       "Steps",
			"unit_of_measurement": "steps",
		},
	}
	if s.FriendlyName() != "Steps" || s.Unit() != "steps" {
		t.Errorf("attributes not read: %q %q", s.FriendlyName(), s.Unit())
	}
	bare := State{EntityID: "sensor.y"}
	if bare.FriendlyName() != "sensor.y" || bare.Unit() != "" {
		t.Errorf("missing attributes should fall back cleanly: %q %q",
			bare.FriendlyName(), bare.Unit())
	}
}

func TestSlugifyUser(t *testing.T) {
	cases := map[string]string{
		"Marvin":           "marvin",
		"Marvin W.":        "marvin_w",
		"Anna-Lena Müller": "anna_lena_mueller",
		"  spaced   out  ": "spaced_out",
		"Ünïcödé":          "uenicoede",
		"":                 "user",
		"!!!":              "user",
		"Straße":           "strasse",
	}
	for in, want := range cases {
		if got := SlugifyUser(in); got != want {
			t.Errorf("SlugifyUser(%q) = %q, want %q", in, got, want)
		}
	}

	// Entity ids cannot be unbounded.
	long := SlugifyUser("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if len(long) > 40 {
		t.Errorf("slug is %d chars, want at most 40", len(long))
	}
	// A slug must never start or end with an underscore.
	for _, s := range []string{SlugifyUser("  x  "), SlugifyUser("--x--"), long} {
		if len(s) > 0 && (s[0] == '_' || s[len(s)-1] == '_') {
			t.Errorf("slug %q has a stray underscore", s)
		}
	}
}

func TestClientDisabledWithoutToken(t *testing.T) {
	t.Setenv("SUPERVISOR_TOKEN", "")
	t.Setenv("HASSIO_TOKEN", "")
	c := New()
	if c.Enabled() {
		t.Error("client should be disabled with no token")
	}
	if _, err := c.GetState(t.Context(), "sensor.x"); err == nil {
		t.Error("expected an error without a token")
	}
}

func TestSuggestTrackersMatchesLikelySensors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"entity_id":"sensor.watch_steps","state":"8412","attributes":{"friendly_name":"Watch Steps"}},
			{"entity_id":"sensor.scale_gewicht","state":"78.4","attributes":{"friendly_name":"Waage"}},
			{"entity_id":"sensor.watch_active_calories","state":"430","attributes":{}},
			{"entity_id":"sensor.living_room_temperature","state":"21.5","attributes":{}},
			{"entity_id":"sensor.watch_steps_text","state":"unknown","attributes":{}},
			{"entity_id":"light.kitchen","state":"on","attributes":{}}
		]`))
	}))
	defer srv.Close()

	t.Setenv("SUPERVISOR_TOKEN", "test-token")
	t.Setenv("GOFITNESS_HA_URL", srv.URL)
	c := New()

	got, err := c.SuggestTrackers(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	byKind := map[string]string{}
	for _, s := range got {
		byKind[s.Kind] = s.EntityID
	}
	if byKind["steps"] != "sensor.watch_steps" {
		t.Errorf("steps suggestion = %q", byKind["steps"])
	}
	if byKind["weight"] != "sensor.scale_gewicht" {
		t.Errorf("weight suggestion = %q", byKind["weight"])
	}
	if byKind["active_energy"] != "sensor.watch_active_calories" {
		t.Errorf("energy suggestion = %q", byKind["active_energy"])
	}
	for _, s := range got {
		if s.EntityID == "light.kitchen" {
			t.Error("non-sensor entity suggested")
		}
		if s.EntityID == "sensor.living_room_temperature" {
			t.Error("unrelated sensor suggested")
		}
		if s.EntityID == "sensor.watch_steps_text" {
			t.Error("non-numeric sensor suggested")
		}
	}
}

func TestTrackerKindsAreStable(t *testing.T) {
	kinds := TrackerKinds()
	if len(kinds) == 0 {
		t.Fatal("no tracker kinds")
	}
	// The returned slice must be a copy: mutating it must not corrupt the list.
	kinds[0] = "mutated"
	if TrackerKinds()[0] == "mutated" {
		t.Error("TrackerKinds exposes its backing array")
	}
}
