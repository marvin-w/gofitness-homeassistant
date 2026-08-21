// Package hass integrates with the Home Assistant Supervisor: it identifies the
// logged-in user from the ingress headers, reads fitness-tracker sensors, and
// publishes the app's own numbers back as sensor entities.
package hass

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Ingress headers Home Assistant sets on every proxied request. They are the
// reason the app needs no login of its own: if the request reached us through
// ingress, Home Assistant already authenticated the user.
const (
	HeaderUserID      = "X-Remote-User-Id"
	HeaderUserName    = "X-Remote-User-Name"
	HeaderDisplayName = "X-Remote-User-Display-Name"
	HeaderIngressPath = "X-Ingress-Path"
)

// LocalUserID is the identity used when the app is reached directly on its port
// rather than through ingress (e.g. during development). Everyone hitting the
// port shares this profile, which is why direct access should stay on the LAN.
const LocalUserID = "local"

// User is the identified caller.
type User struct {
	ID          string
	Name        string
	ViaIngress  bool
	IngressPath string
}

// UserFromRequest reads the Home Assistant identity out of the request. When
// the headers are absent the caller is treated as the shared local user.
func UserFromRequest(r *http.Request) User {
	id := strings.TrimSpace(r.Header.Get(HeaderUserID))
	name := strings.TrimSpace(r.Header.Get(HeaderDisplayName))
	if name == "" {
		name = strings.TrimSpace(r.Header.Get(HeaderUserName))
	}
	path := strings.TrimRight(strings.TrimSpace(r.Header.Get(HeaderIngressPath)), "/")

	if id == "" {
		return User{ID: LocalUserID, Name: name, IngressPath: path}
	}
	if name == "" {
		name = id
	}
	return User{ID: id, Name: name, ViaIngress: true, IngressPath: path}
}

// Client talks to the Home Assistant core API through the Supervisor proxy.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// New builds a Supervisor client from the environment. Home Assistant injects
// SUPERVISOR_TOKEN into every add-on container; without it the client is
// disabled and every call reports that gracefully.
func New() *Client {
	base := os.Getenv("GOFITNESS_HA_URL")
	if base == "" {
		base = "http://supervisor/core"
	}
	token := os.Getenv("SUPERVISOR_TOKEN")
	if token == "" {
		token = os.Getenv("HASSIO_TOKEN")
	}
	return &Client{
		base:  strings.TrimRight(base, "/"),
		token: token,
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled reports whether a Supervisor token is available.
func (c *Client) Enabled() bool { return c.token != "" }

// State is a Home Assistant entity state.
type State struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged string         `json:"last_changed"`
}

// FriendlyName returns the entity's display name, falling back to its id.
func (s State) FriendlyName() string {
	if v, ok := s.Attributes["friendly_name"].(string); ok && v != "" {
		return v
	}
	return s.EntityID
}

// Unit returns the unit of measurement, if the entity declares one.
func (s State) Unit() string {
	if v, ok := s.Attributes["unit_of_measurement"].(string); ok {
		return v
	}
	return ""
}

// Float parses the state as a number. Returns ok=false for "unknown",
// "unavailable" and anything else non-numeric.
func (s State) Float() (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s.State), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if !c.Enabled() {
		return fmt.Errorf("home assistant api not available (no supervisor token)")
	}
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("home assistant api %s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// GetState reads one entity.
func (c *Client) GetState(ctx context.Context, entityID string) (State, error) {
	var s State
	err := c.do(ctx, http.MethodGet, "/api/states/"+url.PathEscape(entityID), nil, &s)
	return s, err
}

// States lists every entity Home Assistant knows about.
func (c *Client) States(ctx context.Context) ([]State, error) {
	var out []State
	err := c.do(ctx, http.MethodGet, "/api/states", nil, &out)
	return out, err
}

// trackerKinds are the data points the app can import from a wearable.
var trackerKinds = []string{"steps", "active_energy", "weight", "heart_rate", "sleep"}

// TrackerKinds lists the supported tracker data kinds.
func TrackerKinds() []string { return append([]string(nil), trackerKinds...) }

// candidateRe matches entity ids that plausibly carry each kind of data, so the
// settings screen can suggest sensors instead of making the user type ids.
var candidateRe = map[string]*regexp.Regexp{
	"steps":         regexp.MustCompile(`(?i)(step|schritt)`),
	"active_energy": regexp.MustCompile(`(?i)(active_energy|active_calor|kalorien|calories_burn|energy_burn)`),
	"weight":        regexp.MustCompile(`(?i)(weight|gewicht|waage|scale)`),
	"heart_rate":    regexp.MustCompile(`(?i)(heart_rate|puls|herzfrequenz|bpm)`),
	"sleep":         regexp.MustCompile(`(?i)(sleep|schlaf)`),
}

// Suggestion is a proposed sensor for a tracker data kind.
type Suggestion struct {
	Kind     string `json:"kind"`
	EntityID string `json:"entity_id"`
	Name     string `json:"name"`
	State    string `json:"state"`
	Unit     string `json:"unit"`
}

// SuggestTrackers scans Home Assistant for sensors that look like fitness data.
// It is a convenience, not a guarantee — the user confirms the mapping.
func (c *Client) SuggestTrackers(ctx context.Context) ([]Suggestion, error) {
	states, err := c.States(ctx)
	if err != nil {
		return nil, err
	}
	var out []Suggestion
	for _, s := range states {
		if !strings.HasPrefix(s.EntityID, "sensor.") {
			continue
		}
		if _, ok := s.Float(); !ok {
			continue // only numeric sensors are useful here
		}
		for kind, re := range candidateRe {
			if re.MatchString(s.EntityID) || re.MatchString(s.FriendlyName()) {
				out = append(out, Suggestion{
					Kind:     kind,
					EntityID: s.EntityID,
					Name:     s.FriendlyName(),
					State:    s.State,
					Unit:     s.Unit(),
				})
				break
			}
		}
	}
	return out, nil
}

// SetSensor publishes a value back into Home Assistant so the app's data can be
// used in dashboards and automations.
func (c *Client) SetSensor(ctx context.Context, entityID string, state any, attrs map[string]any) error {
	if attrs == nil {
		attrs = map[string]any{}
	}
	body := map[string]any{
		"state":      fmt.Sprint(state),
		"attributes": attrs,
	}
	return c.do(ctx, http.MethodPost, "/api/states/"+url.PathEscape(entityID), body, nil)
}

// asciiFold maps accented Latin letters onto their plain ASCII form. Without
// it, a name like "Renée" would lose letters and could collide with a different
// user's slug, which would make two people share one set of sensors.
var asciiFold = map[rune]string{
	'ä': "ae", 'ö': "oe", 'ü': "ue", 'ß': "ss",
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'å': "a", 'æ': "ae",
	'ç': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
	'ñ': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ø': "o",
	'ù': "u", 'ú': "u", 'û': "u",
	'ý': "y", 'ÿ': "y",
	'š': "s", 'ž': "z", 'ł': "l", 'č': "c", 'ř': "r", 'ě': "e", 'ą': "a", 'ę': "e",
}

// SlugifyUser turns a Home Assistant user id or name into an entity-id-safe
// slug, so each person gets their own sensors.
func SlugifyUser(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if folded, ok := asciiFold[r]; ok {
				b.WriteString(folded)
				lastUnderscore = false
				continue
			}
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "user"
	}
	if len(out) > 40 {
		out = strings.Trim(out[:40], "_")
	}
	return out
}
