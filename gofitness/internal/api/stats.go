package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/gamify"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/hass"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/nutrition"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

// handleStats returns the series behind the charts: weight over time with a
// smoothed trend, and daily calories against the target.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	if err := s.requireSetup(rc); err != nil {
		return err
	}
	ctx := r.Context()

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 || days > 730 {
		days = 90
	}
	from := store.Day(rc.Now.AddDate(0, 0, -days+1))
	to := store.Day(rc.Now)

	weights, err := s.store.Weights(ctx, rc.User.UserID, 1000)
	if err != nil {
		return err
	}
	// Weights come back newest first; charts want oldest first.
	series := make([]map[string]any, 0, len(weights))
	for i := len(weights) - 1; i >= 0; i-- {
		e := weights[i]
		if len(e.RecordedAt) >= 10 && e.RecordedAt[:10] < from {
			continue
		}
		series = append(series, map[string]any{
			"date":   e.RecordedAt,
			"day":    e.RecordedAt[:min(10, len(e.RecordedAt))],
			"weight": e.WeightKg,
		})
	}
	trend := movingAverage(weights, 7)

	totals, err := s.store.TotalsRange(ctx, rc.User.UserID, from, to)
	if err != nil {
		return err
	}
	plan := nutrition.Calculate(s.nutritionProfile(ctx, rc.User))

	writeJSON(w, http.StatusOK, map[string]any{
		"from":          from,
		"to":            to,
		"weights":       series,
		"weight_trend":  trend,
		"daily_totals":  totals,
		"plan":          plan,
		"projection":    s.projection(ctx, rc.User, plan),
		"healthy_range": []float64{plan.HealthyLowKg, plan.HealthyHighKg},
	})
	return nil
}

// projection builds the low-pressure forecast (current BMI, weeks to the next
// milestone and to the goal) for the profile's latest weight.
func (s *Server) projection(ctx context.Context, prof store.Profile, plan nutrition.Plan) nutrition.Projection {
	current := prof.StartWeightKg
	if wgt, err := s.store.LatestWeight(ctx, prof.UserID); err == nil {
		current = wgt.WeightKg
	}
	return nutrition.Project(prof.StartWeightKg, current, plan.TargetWeightKg, prof.HeightCm, plan.WeeklyChangeKg)
}

// movingAverage smooths the weight series. Day-to-day weight swings are mostly
// water; a seven-point average is what actually shows whether things are moving.
func movingAverage(weights []store.WeightEntry, window int) []map[string]any {
	if len(weights) == 0 || window <= 1 {
		return []map[string]any{}
	}
	// Reverse into chronological order.
	asc := make([]store.WeightEntry, len(weights))
	for i, e := range weights {
		asc[len(weights)-1-i] = e
	}

	out := make([]map[string]any, 0, len(asc))
	for i := range asc {
		start := i - window + 1
		if start < 0 {
			start = 0
		}
		sum := 0.0
		for j := start; j <= i; j++ {
			sum += asc[j].WeightKg
		}
		avg := sum / float64(i-start+1)
		out = append(out, map[string]any{
			"date":   asc[i].RecordedAt,
			"weight": float64(int(avg*100+0.5)) / 100,
		})
	}
	return out
}

func (s *Server) handleGamify(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	if err := s.requireSetup(rc); err != nil {
		return err
	}
	ctx := r.Context()
	plan := nutrition.Calculate(s.nutritionProfile(ctx, rc.User))
	stats, err := gamify.Compute(ctx, s.store, rc.User.UserID, rc.User, plan, rc.Now)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"gamify":     stats,
		"plan":       plan,
		"projection": s.projection(ctx, rc.User, plan),
	})
	return nil
}

func (s *Server) handleGetTrackers(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	ctx := r.Context()
	links, err := s.store.TrackerLinks(ctx, rc.User.UserID)
	if err != nil {
		return err
	}
	resp := map[string]any{
		"kinds":     hass.TrackerKinds(),
		"links":     links,
		"available": s.ha.Enabled(),
	}
	if s.ha.Enabled() && r.URL.Query().Get("suggest") == "1" {
		// Discovery is a nice-to-have: a Home Assistant hiccup should not stop
		// the settings screen from rendering the links that are already saved.
		if sug, err := s.ha.SuggestTrackers(ctx); err == nil {
			resp["suggestions"] = sug
		} else {
			s.log.Warn("tracker discovery failed", "err", err)
			resp["suggestions"] = []any{}
		}
	}
	writeJSON(w, http.StatusOK, resp)
	return nil
}

func (s *Server) handleSetTracker(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	var req struct {
		Kind     string `json:"kind"`
		EntityID string `json:"entity_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	valid := false
	for _, k := range hass.TrackerKinds() {
		if k == req.Kind {
			valid = true
			break
		}
	}
	if !valid {
		return badRequest("unknown tracker kind")
	}
	if err := s.store.SetTrackerLink(r.Context(), rc.User.UserID, req.Kind, req.EntityID); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// handleSyncTrackers pulls today's values from the linked Home Assistant
// entities. Weight is only imported when it differs from the last recorded
// value, so a smart scale that reports continuously does not flood the log.
func (s *Server) handleSyncTrackers(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	if !s.ha.Enabled() {
		return apiError{Status: http.StatusPreconditionFailed,
			Msg: "no connection to Home Assistant (add-on running outside the Supervisor?)"}
	}
	ctx := r.Context()
	links, err := s.store.TrackerLinks(ctx, rc.User.UserID)
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return badRequest("no tracker entities linked yet")
	}

	day := store.Day(rc.Now)
	imported := map[string]any{}
	var steps int
	var activeKcal float64

	for kind, entity := range links {
		state, err := s.ha.GetState(ctx, entity)
		if err != nil {
			s.log.Warn("tracker read failed", "kind", kind, "entity", entity, "err", err)
			continue
		}
		v, ok := state.Float()
		if !ok {
			continue
		}
		switch kind {
		case "steps":
			steps = int(v)
			imported["steps"] = steps
		case "active_energy":
			activeKcal = v
			imported["active_energy"] = v
		case "weight":
			if err := s.importWeight(ctx, rc, v); err != nil {
				return err
			}
			imported["weight"] = v
		case "heart_rate", "sleep":
			imported[kind] = v
		}
	}

	if steps > 0 || activeKcal > 0 {
		if err := s.store.UpsertTrackerWorkout(ctx, rc.User.UserID, store.Workout{
			Day:    day,
			Kind:   "tracker",
			Kcal:   activeKcal,
			Steps:  steps,
			Source: "tracker",
		}); err != nil {
			return err
		}
	}

	totals, err := s.store.TotalsForDay(ctx, rc.User.UserID, day)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "totals": totals})
	return nil
}

// importWeight records a scale reading, skipping near-duplicates.
func (s *Server) importWeight(ctx context.Context, rc *reqCtx, kg float64) error {
	if kg < 30 || kg > 350 {
		return nil // implausible reading, ignore rather than corrupt the chart
	}
	last, err := s.store.LatestWeight(ctx, rc.User.UserID)
	if err == nil {
		sameDay := len(last.RecordedAt) >= 10 && last.RecordedAt[:10] == store.Day(rc.Now)
		if sameDay && abs(last.WeightKg-kg) < 0.05 {
			return nil
		}
	}
	_, err = s.store.AddWeight(ctx, rc.User.UserID, store.WeightEntry{
		WeightKg: kg,
		Source:   "tracker",
	})
	return err
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// publishSensors mirrors the user's key numbers into Home Assistant so they can
// be used on dashboards and in automations. Failures are logged, never fatal:
// the app's own storage is the source of truth.
func (s *Server) publishSensors(ctx context.Context, prof store.Profile, plan nutrition.Plan) {
	if !s.cfg.PublishSensors || !s.ha.Enabled() {
		return
	}

	slug := hass.SlugifyUser(prof.DisplayName)
	if slug == "user" || slug == "" {
		slug = hass.SlugifyUser(prof.UserID)
	}

	// Publishing must not block the request that triggered it.
	go func() {
		pubCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		now := time.Now()
		today := store.Day(now)

		weight := prof.StartWeightKg
		if wgt, err := s.store.LatestWeight(pubCtx, prof.UserID); err == nil {
			weight = wgt.WeightKg
		}
		totals, err := s.store.TotalsForDay(pubCtx, prof.UserID, today)
		if err != nil {
			s.log.Warn("sensor publish: totals failed", "err", err)
			return
		}
		stats, err := gamify.Compute(pubCtx, s.store, prof.UserID, prof, plan, now)
		if err != nil {
			s.log.Warn("sensor publish: stats failed", "err", err)
			return
		}

		bmi := nutrition.BMI(weight, prof.HeightCm)
		sensors := []struct {
			id    string
			state any
			attrs map[string]any
		}{
			{"sensor.gofitness_" + slug + "_weight", round1f(weight), map[string]any{
				"friendly_name":       prof.DisplayName + " Gewicht",
				"unit_of_measurement": "kg",
				"device_class":        "weight",
				"state_class":         "measurement",
				"icon":                "mdi:scale-bathroom",
				"start_weight":        prof.StartWeightKg,
				"target_weight":       plan.TargetWeightKg,
			}},
			{"sensor.gofitness_" + slug + "_bmi", round1f(bmi), map[string]any{
				"friendly_name": prof.DisplayName + " BMI",
				"icon":          "mdi:human",
				"state_class":   "measurement",
				"category":      plan.BMICategory,
				"healthy_low":   plan.HealthyLowKg,
				"healthy_high":  plan.HealthyHighKg,
			}},
			{"sensor.gofitness_" + slug + "_kcal_today", round0(totals.Kcal), map[string]any{
				"friendly_name":       prof.DisplayName + " Kalorien heute",
				"unit_of_measurement": "kcal",
				"icon":                "mdi:food-apple",
				"state_class":         "total",
				"target":              plan.TargetKcal,
				"remaining":           round0(plan.TargetKcal - totals.Kcal),
				"protein_g":           totals.ProteinG,
				"carbs_g":             totals.CarbsG,
				"fat_g":               totals.FatG,
			}},
			{"sensor.gofitness_" + slug + "_kcal_target", plan.TargetKcal, map[string]any{
				"friendly_name":       prof.DisplayName + " Kalorienziel",
				"unit_of_measurement": "kcal",
				"icon":                "mdi:target",
				"tdee":                plan.TDEE,
				"bmr":                 plan.BMR,
				"protein_g":           plan.ProteinG,
				"carbs_g":             plan.CarbsG,
				"fat_g":               plan.FatG,
			}},
			{"sensor.gofitness_" + slug + "_streak", stats.CurrentStreak, map[string]any{
				"friendly_name":       prof.DisplayName + " Streak",
				"unit_of_measurement": "d",
				"icon":                "mdi:fire",
				"longest":             stats.LongestStreak,
				"level":               stats.Level,
				"xp":                  stats.XP,
				"badges":              stats.UnlockedCount,
			}},
		}

		for _, sensor := range sensors {
			if err := s.ha.SetSensor(pubCtx, sensor.id, sensor.state, sensor.attrs); err != nil {
				s.log.Warn("sensor publish failed", "entity", sensor.id, "err", err)
			}
		}
	}()
}

func round1f(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
