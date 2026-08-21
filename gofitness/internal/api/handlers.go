package api

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/ai"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/gamify"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/nutrition"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/recipes"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

// handleBootstrap returns everything the app needs on first paint: who you are,
// your plan, today's numbers and your gamification state. One round trip keeps
// the interface snappy on a phone.
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	ctx := r.Context()
	today := store.Day(rc.Now)

	resp := map[string]any{
		"user": map[string]any{
			"id":           rc.HA.ID,
			"name":         rc.HA.Name,
			"via_ingress":  rc.HA.ViaIngress,
			"ingress_path": rc.HA.IngressPath,
		},
		"lang": rc.Lang,
		"capabilities": map[string]any{
			"ai":             s.ai.Enabled(),
			"ai_model":       s.ai.Model(),
			"home_assistant": s.ha.Enabled(),
			"recipes":        s.book.Len(),
		},
		"setup_done": rc.User.SetupDone,
		"today":      today,
	}

	if !rc.User.SetupDone {
		// Nothing else is meaningful before setup; the wizard takes over.
		resp["profile"] = rc.User
		writeJSON(w, http.StatusOK, resp)
		return nil
	}

	np := s.nutritionProfile(ctx, rc.User)
	plan := nutrition.Calculate(np)

	totals, err := s.store.TotalsForDay(ctx, rc.User.UserID, today)
	if err != nil {
		return err
	}
	food, err := s.store.FoodByDay(ctx, rc.User.UserID, today)
	if err != nil {
		return err
	}
	workouts, err := s.store.WorkoutsByDay(ctx, rc.User.UserID, today)
	if err != nil {
		return err
	}
	stats, err := gamify.Compute(ctx, s.store, rc.User.UserID, rc.User, plan, rc.Now)
	if err != nil {
		return err
	}

	latest, err := s.store.LatestWeight(ctx, rc.User.UserID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}

	resp["profile"] = rc.User
	resp["plan"] = plan
	resp["totals"] = totals
	resp["food"] = food
	resp["workouts"] = workouts
	resp["gamify"] = stats
	resp["current_weight"] = np.WeightKg
	if latest.RecordedAt != "" {
		resp["last_weigh_in"] = latest
	}

	writeJSON(w, http.StatusOK, resp)
	return nil
}

// profileRequest is the setup wizard's payload.
type profileRequest struct {
	DisplayName    string      `json:"display_name"`
	Sex            string      `json:"sex"`
	BirthDate      string      `json:"birth_date"`
	HeightCm       float64     `json:"height_cm"`
	WeightKg       float64     `json:"weight_kg"`
	TargetWeightKg float64     `json:"target_weight_kg"`
	Activity       string      `json:"activity"`
	Goal           string      `json:"goal"`
	Breastfeeding  string      `json:"breastfeeding"`
	Prefs          store.Prefs `json:"prefs"`
}

// validate normalises and range-checks the wizard input. Values outside these
// ranges are almost always typos (cm vs m, lbs vs kg) and would produce
// nonsense calorie targets, so they are rejected rather than clamped.
func (p *profileRequest) validate() error {
	switch p.Sex {
	case "female", "male", "divers":
	default:
		return badRequest("sex must be female, male or divers")
	}
	switch p.Activity {
	case "sedentary", "light", "moderate", "active", "very_active":
	default:
		return badRequest("invalid activity level")
	}
	switch p.Goal {
	case "lose", "maintain", "gain_muscle", "recomp":
	default:
		return badRequest("invalid goal")
	}
	switch p.Breastfeeding {
	case "", "none", "partial", "exclusive":
		if p.Breastfeeding == "" {
			p.Breastfeeding = "none"
		}
	default:
		return badRequest("invalid breastfeeding value")
	}
	if p.HeightCm < 120 || p.HeightCm > 230 {
		return badRequest("height must be between 120 and 230 cm")
	}
	if p.WeightKg < 30 || p.WeightKg > 350 {
		return badRequest("weight must be between 30 and 350 kg")
	}
	if p.TargetWeightKg != 0 && (p.TargetWeightKg < 30 || p.TargetWeightKg > 350) {
		return badRequest("target weight must be between 30 and 350 kg")
	}
	if p.BirthDate != "" {
		t, err := time.Parse("2006-01-02", p.BirthDate)
		if err != nil {
			return badRequest("birth date must be YYYY-MM-DD")
		}
		if t.After(time.Now()) {
			return badRequest("birth date is in the future")
		}
	}
	return nil
}

// toProfile merges the request into an existing profile.
func (p profileRequest) toProfile(existing store.Profile, userID, fallbackName string) store.Profile {
	prefs := p.Prefs
	if prefs.MealsPerDay == 0 {
		prefs.MealsPerDay = 4
	}
	if prefs.HouseholdSize == 0 {
		prefs.HouseholdSize = 1
	}
	if prefs.FishPolicy == "" {
		prefs.FishPolicy = "breaded_only"
	}
	if prefs.VeggieLevel == "" {
		prefs.VeggieLevel = "low"
	}
	if prefs.MaxCookMinutes == 0 {
		prefs.MaxCookMinutes = 45
	}
	if prefs.Language == "" {
		prefs.Language = "de"
	}
	prefs.Language = recipes.NormalizeLang(prefs.Language)

	name := strings.TrimSpace(p.DisplayName)
	if name == "" {
		name = existing.DisplayName
	}
	if name == "" {
		name = fallbackName
	}

	start := existing.StartWeightKg
	if start == 0 {
		// The first weight entered is the baseline every milestone is measured
		// against; later profile edits must not move the goalposts.
		start = p.WeightKg
	}

	return store.Profile{
		UserID:         userID,
		DisplayName:    name,
		Sex:            p.Sex,
		BirthDate:      p.BirthDate,
		HeightCm:       p.HeightCm,
		StartWeightKg:  start,
		TargetWeightKg: p.TargetWeightKg,
		Activity:       p.Activity,
		Goal:           p.Goal,
		Breastfeeding:  p.Breastfeeding,
		Prefs:          prefs,
		SetupDone:      true,
	}
}

func (s *Server) handleSaveProfile(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	var req profileRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if err := req.validate(); err != nil {
		return err
	}
	ctx := r.Context()

	prof := req.toProfile(rc.User, rc.HA.ID, rc.HA.Name)
	if err := s.store.SaveProfile(ctx, prof); err != nil {
		return err
	}

	// The setup wizard doubles as the first weigh-in, so the weight chart is
	// never empty after onboarding.
	if _, err := s.store.LatestWeight(ctx, prof.UserID); errors.Is(err, store.ErrNotFound) {
		if _, err := s.store.AddWeight(ctx, prof.UserID, store.WeightEntry{
			WeightKg: req.WeightKg,
			Source:   "setup",
		}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	np := s.nutritionProfile(ctx, prof)
	plan := nutrition.Calculate(np)
	s.publishSensors(ctx, prof, plan)

	writeJSON(w, http.StatusOK, map[string]any{"profile": prof, "plan": plan})
	return nil
}

// handlePreviewProfile calculates a plan without saving anything, so the wizard
// can show live numbers while the user is still choosing.
func (s *Server) handlePreviewProfile(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	var req profileRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if err := req.validate(); err != nil {
		return err
	}
	np := nutrition.Profile{
		Sex:           nutrition.Sex(req.Sex),
		HeightCm:      req.HeightCm,
		WeightKg:      req.WeightKg,
		Activity:      nutrition.Activity(req.Activity),
		Goal:          nutrition.Goal(req.Goal),
		Breastfeeding: nutrition.Breastfeeding(req.Breastfeeding),
		TargetWeight:  req.TargetWeightKg,
	}
	np.Age = store.Profile{BirthDate: req.BirthDate}.Age()

	plan := nutrition.Calculate(np)
	writeJSON(w, http.StatusOK, map[string]any{
		"plan":       plan,
		"milestones": nutrition.Milestones(req.WeightKg, req.WeightKg, plan.TargetWeightKg, req.HeightCm),
	})
	return nil
}

func (s *Server) requireSetup(rc *reqCtx) error {
	if !rc.User.SetupDone {
		return apiError{Status: http.StatusPreconditionRequired, Msg: "setup not completed"}
	}
	return nil
}

func (s *Server) handleListWeights(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.store.Weights(r.Context(), rc.User.UserID, limit)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"weights": list})
	return nil
}

func (s *Server) handleAddWeight(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	var req struct {
		WeightKg   float64  `json:"weight_kg"`
		BodyFatPct *float64 `json:"body_fat_pct"`
		RecordedAt string   `json:"recorded_at"`
		Note       string   `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if req.WeightKg < 30 || req.WeightKg > 350 {
		return badRequest("weight must be between 30 and 350 kg")
	}
	if req.BodyFatPct != nil && (*req.BodyFatPct < 3 || *req.BodyFatPct > 70) {
		return badRequest("body fat must be between 3 and 70 %")
	}
	if req.RecordedAt != "" {
		if _, err := time.Parse(time.RFC3339, req.RecordedAt); err != nil {
			return badRequest("recorded_at must be RFC3339")
		}
	}

	ctx := r.Context()
	id, err := s.store.AddWeight(ctx, rc.User.UserID, store.WeightEntry{
		WeightKg:   req.WeightKg,
		BodyFatPct: req.BodyFatPct,
		RecordedAt: req.RecordedAt,
		Note:       req.Note,
	})
	if err != nil {
		return err
	}

	plan := nutrition.Calculate(s.nutritionProfile(ctx, rc.User))
	stats, err := gamify.Compute(ctx, s.store, rc.User.UserID, rc.User, plan, rc.Now)
	if err != nil {
		return err
	}
	s.publishSensors(ctx, rc.User, plan)

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "plan": plan, "gamify": stats})
	return nil
}

func (s *Server) handleDeleteWeight(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	id, err := pathInt(r, "id")
	if err != nil {
		return err
	}
	if err := s.store.DeleteWeight(r.Context(), rc.User.UserID, id); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (s *Server) handleListFood(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	ctx := r.Context()
	day := queryDay(r, "day", rc.Now)
	list, err := s.store.FoodByDay(ctx, rc.User.UserID, day)
	if err != nil {
		return err
	}
	totals, err := s.store.TotalsForDay(ctx, rc.User.UserID, day)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"day": day, "food": list, "totals": totals})
	return nil
}

func (s *Server) handleAddFood(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	var req struct {
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
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if strings.TrimSpace(req.Name) == "" {
		return badRequest("name is required")
	}
	if req.Kcal < 0 || req.Kcal > 10000 {
		return badRequest("kcal out of range")
	}

	ctx := r.Context()
	if req.Day == "" {
		req.Day = store.Day(rc.Now)
	}
	id, err := s.store.AddFood(ctx, rc.User.UserID, store.FoodEntry{
		Day:        req.Day,
		MealType:   req.MealType,
		Name:       strings.TrimSpace(req.Name),
		Amount:     req.Amount,
		Kcal:       req.Kcal,
		ProteinG:   req.ProteinG,
		CarbsG:     req.CarbsG,
		FatG:       req.FatG,
		Source:     req.Source,
		RecipeID:   req.RecipeID,
		Confidence: req.Confidence,
	})
	if err != nil {
		return err
	}

	totals, err := s.store.TotalsForDay(ctx, rc.User.UserID, req.Day)
	if err != nil {
		return err
	}
	plan := nutrition.Calculate(s.nutritionProfile(ctx, rc.User))
	stats, err := gamify.Compute(ctx, s.store, rc.User.UserID, rc.User, plan, rc.Now)
	if err != nil {
		return err
	}
	s.publishSensors(ctx, rc.User, plan)

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "totals": totals, "gamify": stats})
	return nil
}

func (s *Server) handleDeleteFood(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	id, err := pathInt(r, "id")
	if err != nil {
		return err
	}
	if err := s.store.DeleteFood(r.Context(), rc.User.UserID, id); err != nil {
		return err
	}
	totals, err := s.store.TotalsForDay(r.Context(), rc.User.UserID, queryDay(r, "day", rc.Now))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "totals": totals})
	return nil
}

// maxImageBytes caps uploaded photos. Phone cameras produce far larger files
// than a calorie estimate needs, and the API request has to stay small.
const maxImageBytes = 6 << 20

// handleEstimateFood estimates calories from text or a photo. It never saves
// anything: the frontend shows the result and asks the user to confirm first.
func (s *Server) handleEstimateFood(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	ctx := r.Context()
	contentType := r.Header.Get("Content-Type")

	// Photo upload.
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxImageBytes); err != nil {
			return badRequest("image too large or malformed (max 6 MB)")
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			return badRequest("no image supplied")
		}
		defer file.Close()

		data, err := io.ReadAll(io.LimitReader(file, maxImageBytes))
		if err != nil {
			return badRequest("could not read image")
		}
		mediaType := detectImageType(data, header.Header.Get("Content-Type"))
		if mediaType == "" {
			return badRequest("unsupported image format — use JPEG, PNG, GIF or WebP")
		}
		if !s.ai.Enabled() {
			return apiError{Status: http.StatusPreconditionFailed,
				Msg: "photo recognition needs an Anthropic API key in the add-on configuration"}
		}
		est, err := s.ai.EstimateImage(ctx, data, mediaType, r.FormValue("note"), rc.Lang)
		if err != nil {
			return apiError{Status: http.StatusBadGateway, Msg: "AI estimate failed", Err: err}
		}
		writeJSON(w, http.StatusOK, map[string]any{"estimate": est})
		return nil
	}

	// Text description.
	var req struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if strings.TrimSpace(req.Text) == "" {
		return badRequest("text is required")
	}

	if s.ai.Enabled() {
		est, err := s.ai.EstimateText(ctx, req.Text, rc.Lang)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"estimate": est})
			return nil
		}
		// The local table is a genuinely useful answer, so a failed API call
		// degrades to it instead of surfacing an error.
		s.log.Warn("ai estimate failed, using local food table", "err", err)
	}

	est, err := ai.EstimateLocal(req.Text, rc.Lang)
	if err != nil {
		return apiError{Status: http.StatusNotFound,
			Msg: "no match in the local food table — enter the calories manually or configure an API key"}
	}
	writeJSON(w, http.StatusOK, map[string]any{"estimate": est})
	return nil
}

// detectImageType sniffs the image format, falling back to the declared type.
// The magic bytes are trusted over the header because browsers and phones are
// inconsistent about what they declare.
func detectImageType(data []byte, declared string) string {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	case len(data) >= 8 && data[0] == 0x89 && string(data[1:4]) == "PNG":
		return "image/png"
	case len(data) >= 3 && string(data[0:3]) == "GIF":
		return "image/gif"
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	}
	switch strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0])) {
	case "image/jpeg", "image/jpg":
		return "image/jpeg"
	case "image/png":
		return "image/png"
	case "image/gif":
		return "image/gif"
	case "image/webp":
		return "image/webp"
	}
	return ""
}

func (s *Server) handleSearchFood(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 15
	}
	found := ai.SearchFoods(q, rc.Lang, limit)

	type row struct {
		Name     string  `json:"name"`
		Portion  string  `json:"portion"`
		PortionG float64 `json:"portion_g"`
		Kcal     float64 `json:"kcal"`
		ProteinG float64 `json:"protein_g"`
		CarbsG   float64 `json:"carbs_g"`
		FatG     float64 `json:"fat_g"`
		Kcal100  float64 `json:"kcal_100g"`
	}
	out := make([]row, 0, len(found))
	for _, f := range found {
		factor := f.PortionG / 100
		out = append(out, row{
			Name:     f.Name(rc.Lang),
			Portion:  f.PortionLabel(rc.Lang),
			PortionG: f.PortionG,
			Kcal:     rnd(f.Kcal100 * factor),
			ProteinG: rnd(f.P100 * factor),
			CarbsG:   rnd(f.C100 * factor),
			FatG:     rnd(f.F100 * factor),
			Kcal100:  f.Kcal100,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
	return nil
}

func (s *Server) handleListWorkouts(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	day := queryDay(r, "day", rc.Now)
	list, err := s.store.WorkoutsByDay(r.Context(), rc.User.UserID, day)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"day": day, "workouts": list})
	return nil
}

func (s *Server) handleAddWorkout(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	var req struct {
		Day     string  `json:"day"`
		Kind    string  `json:"kind"`
		Minutes float64 `json:"minutes"`
		Kcal    float64 `json:"kcal"`
		Steps   int     `json:"steps"`
		Note    string  `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if strings.TrimSpace(req.Kind) == "" {
		return badRequest("kind is required")
	}
	if req.Minutes < 0 || req.Minutes > 1440 {
		return badRequest("minutes out of range")
	}
	if req.Kcal < 0 || req.Kcal > 10000 {
		return badRequest("kcal out of range")
	}

	ctx := r.Context()
	if req.Day == "" {
		req.Day = store.Day(rc.Now)
	}
	// A rough estimate is better than a zero: without it the workout does not
	// show up in the day's energy balance at all.
	if req.Kcal == 0 && req.Minutes > 0 {
		req.Kcal = estimateWorkoutKcal(req.Kind, req.Minutes, s.currentWeight(ctx, rc))
	}

	id, err := s.store.AddWorkout(ctx, rc.User.UserID, store.Workout{
		Day:     req.Day,
		Kind:    req.Kind,
		Minutes: req.Minutes,
		Kcal:    req.Kcal,
		Steps:   req.Steps,
		Note:    req.Note,
	})
	if err != nil {
		return err
	}
	plan := nutrition.Calculate(s.nutritionProfile(ctx, rc.User))
	stats, err := gamify.Compute(ctx, s.store, rc.User.UserID, rc.User, plan, rc.Now)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "kcal": req.Kcal, "gamify": stats})
	return nil
}

// metEquivalents are MET values for the workout kinds the app offers.
var metEquivalents = map[string]float64{
	"walk":      3.5,
	"run":       9.0,
	"cycle":     7.0,
	"strength":  5.0,
	"swim":      7.0,
	"yoga":      3.0,
	"hiit":      8.0,
	"stroller":  3.3, // pushing a pram counts
	"housework": 3.0,
	"other":     4.0,
}

// estimateWorkoutKcal uses the standard MET formula: kcal = MET × kg × hours.
func estimateWorkoutKcal(kind string, minutes, weightKg float64) float64 {
	met, ok := metEquivalents[kind]
	if !ok {
		met = metEquivalents["other"]
	}
	if weightKg <= 0 {
		weightKg = 75
	}
	return float64(int(met*weightKg*(minutes/60) + 0.5))
}

func (s *Server) currentWeight(ctx context.Context, rc *reqCtx) float64 {
	if w, err := s.store.LatestWeight(ctx, rc.User.UserID); err == nil {
		return w.WeightKg
	}
	return rc.User.StartWeightKg
}

func (s *Server) handleDeleteWorkout(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	id, err := pathInt(r, "id")
	if err != nil {
		return err
	}
	if err := s.store.DeleteWorkout(r.Context(), rc.User.UserID, id); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (s *Server) handleListRecipes(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	q := r.URL.Query()
	f := recipes.Filter{
		MealType:      q.Get("meal_type"),
		Query:         q.Get("q"),
		MaxVeggieRank: recipes.RankFor(orDefault(q.Get("veggie_level"), rc.User.Prefs.VeggieLevel)),
		FishPolicy:    orDefault(q.Get("fish_policy"), rc.User.Prefs.FishPolicy),
		MealPrepOnly:  q.Get("meal_prep") == "1",
	}
	if q.Get("all") == "1" {
		// The recipe browser can deliberately ignore the household filters.
		f = recipes.Filter{MealType: q.Get("meal_type"), Query: q.Get("q"), MaxVeggieRank: 2, FishPolicy: "any"}
	}
	if rc.User.Breastfeeding == "partial" || rc.User.Breastfeeding == "exclusive" {
		f.BreastfeedingSafe = true
	}

	list := recipes.LocalizeAll(s.book.Select(f), rc.Lang)
	writeJSON(w, http.StatusOK, map[string]any{"recipes": list, "count": len(list)})
	return nil
}

func (s *Server) handleGetRecipe(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	rec, ok := s.book.Get(r.PathValue("id"))
	if !ok {
		return notFound("recipe not found")
	}
	servings := rc.User.Prefs.HouseholdSize
	if v, err := strconv.Atoi(r.URL.Query().Get("servings")); err == nil && v > 0 && v <= 20 {
		servings = v
	}
	if servings <= 0 {
		servings = rec.Servings
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"recipe":   recipes.Localize(rec, rc.Lang),
		"servings": servings,
		"scaled_ingredients": recipes.LocalizeIngredients(
			rec.ScaleIngredients(float64(servings)), rc.Lang),
	})
	return nil
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func rnd(v float64) float64 { return float64(int(v*10+0.5)) / 10 }

// weekStart returns the Monday of the week containing t.
func weekStart(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // Monday = 0
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, -offset)
}

// parseWeek reads a ?week=YYYY-MM-DD parameter and snaps it to a Monday.
func parseWeek(r *http.Request, now time.Time) time.Time {
	v := strings.TrimSpace(r.URL.Query().Get("week"))
	if v == "" {
		return weekStart(now)
	}
	t, err := time.ParseInLocation("2006-01-02", v, now.Location())
	if err != nil {
		return weekStart(now)
	}
	return weekStart(t)
}

// seedFor derives a stable shuffle seed. The same user and week always
// regenerate the same plan unless they explicitly ask to reshuffle.
func seedFor(userID, week string, shuffle int) uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%s|%d", userID, week, shuffle)
	return h.Sum64()
}
