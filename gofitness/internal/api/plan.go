package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/gamify"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/mealplan"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/nutrition"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/recipes"
	"github.com/marvin-w/gofitness-homeassistant/gofitness/internal/store"
)

// handleGetPlan returns the stored plan for a week. If none exists yet the week
// is generated on the fly but not saved, so the user sees a proposal they can
// reshuffle before committing to a shopping list.
func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	if err := s.requireSetup(rc); err != nil {
		return err
	}
	ctx := r.Context()
	week := parseWeek(r, rc.Now)
	weekKey := week.Format("2006-01-02")

	planID, err := s.store.PlanID(ctx, store.HouseholdID, weekKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if errors.Is(err, store.ErrNotFound) {
		generated := s.generate(ctx, rc, week, 0)
		writeJSON(w, http.StatusOK, map[string]any{
			"week_start": weekKey,
			"saved":      false,
			"plan":       generated,
			"shopping":   mealplan.ShoppingList(s.book, generated, rc.Lang),
		})
		return nil
	}

	plan, shopping, err := s.loadStoredPlan(ctx, rc, planID, week)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"week_start": weekKey,
		"saved":      true,
		"plan":       plan,
		"shopping":   shopping,
	})
	return nil
}

// handleGeneratePlan builds a week and persists it together with its shopping
// list, replacing whatever was stored for that week before.
func (s *Server) handleGeneratePlan(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	if err := s.requireSetup(rc); err != nil {
		return err
	}
	var req struct {
		Week    string `json:"week"`
		Shuffle int    `json:"shuffle"`
	}
	if err := decodeJSON(r, &req); err != nil {
		return err
	}

	ctx := r.Context()
	week := weekStart(rc.Now)
	if req.Week != "" {
		t, err := time.ParseInLocation("2006-01-02", req.Week, rc.Now.Location())
		if err != nil {
			return badRequest("week must be YYYY-MM-DD")
		}
		week = weekStart(t)
	}
	weekKey := week.Format("2006-01-02")

	generated := s.generate(ctx, rc, week, req.Shuffle)
	shopping := mealplan.ShoppingList(s.book, generated, rc.Lang)

	entries := make([]store.PlanEntry, 0, 28)
	for _, day := range generated.Days {
		for _, e := range day.Entries {
			entries = append(entries, store.PlanEntry{
				DayIndex: e.DayIndex,
				MealType: e.MealType,
				RecipeID: e.RecipeID,
				Servings: e.Portions,
			})
		}
	}
	items := make([]store.ShoppingItem, 0, len(shopping))
	for _, it := range shopping {
		items = append(items, store.ShoppingItem{
			Name:     it.Name,
			Amount:   it.Amount,
			Unit:     it.Unit,
			Category: it.Category,
		})
	}

	planID, err := s.store.SavePlan(ctx, store.HouseholdID, weekKey, entries, items)
	if err != nil {
		return err
	}

	plan, shoppingOut, err := s.loadStoredPlan(ctx, rc, planID, week)
	if err != nil {
		return err
	}

	nutriPlan := nutrition.Calculate(s.nutritionProfile(ctx, rc.User))
	stats, err := gamify.Compute(ctx, s.store, rc.User.UserID, rc.User, nutriPlan, rc.Now)
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"week_start": weekKey,
		"saved":      true,
		"plan":       plan,
		"shopping":   shoppingOut,
		"gamify":     stats,
	})
	return nil
}

// generate runs the planner for a week. The plan is shared by the whole
// household, so it is sized to the combined calorie target of everyone who has
// finished setup and made lactation-safe if anyone in the house is
// breastfeeding. "Cook once, eat twice" is always on.
func (s *Server) generate(ctx context.Context, rc *reqCtx, week time.Time, shuffle int) mealplan.Plan {
	target, breastfeeding := s.householdPlanInputs(ctx)
	if target <= 0 {
		// No one is fully set up yet: fall back to the requesting user's target
		// so the preview is still sensible.
		target = nutrition.Calculate(s.nutritionProfile(ctx, rc.User)).TargetKcal
	}

	dates := make([]string, 7)
	for i := range dates {
		dates[i] = week.AddDate(0, 0, i).Format("2006-01-02")
	}

	household := rc.User.Prefs.HouseholdSize
	if household < 1 {
		household = 1
	}

	return mealplan.Generate(s.book, mealplan.Options{
		WeekStart:         week.Format("2006-01-02"),
		Dates:             dates,
		TargetKcal:        target,
		Prefs:             rc.User.Prefs,
		BreastfeedingSafe: breastfeeding,
		Seed:              seedFor(store.HouseholdID, week.Format("2006-01-02"), shuffle),
		CookOnceEatTwice:  true,
		MaxPortions:       float64(household) * 2,
		Lang:              rc.Lang,
	})
}

// householdPlanInputs sums the daily calorie targets of every household member
// who has finished setup and reports whether anyone is breastfeeding. The plan
// is sized so there is enough cooked for everyone to eat to their own target.
func (s *Server) householdPlanInputs(ctx context.Context) (targetKcal float64, breastfeeding bool) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return 0, false
	}
	for _, u := range users {
		prof, err := s.store.GetProfile(ctx, u.ID)
		if err != nil || !prof.SetupDone {
			continue
		}
		targetKcal += nutrition.Calculate(s.nutritionProfile(ctx, prof)).TargetKcal
		if prof.Breastfeeding == "partial" || prof.Breastfeeding == "exclusive" {
			breastfeeding = true
		}
	}
	return targetKcal, breastfeeding
}

// storedEntry adds the database id to a plan entry so the frontend can tick it
// off. The generated plan has no ids; the stored one does.
type storedEntry struct {
	mealplan.Entry
	ID int64 `json:"id"`
}

// storedDay mirrors mealplan.Day but with identified entries.
type storedDay struct {
	Index    int           `json:"index"`
	Name     string        `json:"name"`
	Date     string        `json:"date"`
	Entries  []storedEntry `json:"entries"`
	Kcal     float64       `json:"kcal"`
	ProteinG float64       `json:"protein_g"`
	CarbsG   float64       `json:"carbs_g"`
	FatG     float64       `json:"fat_g"`
}

// storedPlan is the shape returned for a persisted week.
type storedPlan struct {
	WeekStart string          `json:"week_start"`
	Days      []storedDay     `json:"days"`
	AvgKcal   float64         `json:"avg_kcal"`
	Notes     []mealplan.Note `json:"notes"`
}

// loadStoredPlan reassembles a saved plan, translating recipe titles into the
// caller's language on the way out.
func (s *Server) loadStoredPlan(ctx context.Context, rc *reqCtx, planID int64, week time.Time) (storedPlan, []store.ShoppingItem, error) {
	entries, err := s.store.PlanEntries(ctx, planID)
	if err != nil {
		return storedPlan{}, nil, err
	}
	items, err := s.store.ShoppingItems(ctx, planID)
	if err != nil {
		return storedPlan{}, nil, err
	}

	names := mealplan.Weekdays(rc.Lang)
	out := storedPlan{WeekStart: week.Format("2006-01-02")}
	byDay := map[int]*storedDay{}
	for i := 0; i < 7; i++ {
		d := &storedDay{
			Index: i,
			Name:  names[i],
			Date:  week.AddDate(0, 0, i).Format("2006-01-02"),
		}
		byDay[i] = d
	}

	for _, e := range entries {
		day, ok := byDay[e.DayIndex]
		if !ok {
			continue
		}
		rec, ok := s.book.Get(e.RecipeID)
		if !ok {
			// A recipe removed in a later release: keep the slot visible rather
			// than silently dropping a meal the user planned around.
			day.Entries = append(day.Entries, storedEntry{
				ID: e.ID,
				Entry: mealplan.Entry{
					DayIndex: e.DayIndex,
					Day:      day.Name,
					MealType: e.MealType,
					RecipeID: e.RecipeID,
					Title:    e.RecipeID,
					Portions: e.Servings,
					Cooked:   e.Cooked,
				},
			})
			continue
		}
		loc := recipes.Localize(rec, rc.Lang)
		se := storedEntry{
			ID: e.ID,
			Entry: mealplan.Entry{
				DayIndex: e.DayIndex,
				Day:      day.Name,
				MealType: e.MealType,
				RecipeID: rec.ID,
				Title:    loc.Title,
				URL:      loc.URL,
				Portions: e.Servings,
				Kcal:     round0(rec.Kcal * e.Servings),
				ProteinG: round0(rec.ProteinG * e.Servings),
				CarbsG:   round0(rec.CarbsG * e.Servings),
				FatG:     round0(rec.FatG * e.Servings),
				PortionG: rec.PortionG,
				Cooked:   e.Cooked,
			},
		}
		day.Entries = append(day.Entries, se)
		day.Kcal += se.Kcal
		day.ProteinG += se.ProteinG
		day.CarbsG += se.CarbsG
		day.FatG += se.FatG
	}

	total := 0.0
	for i := 0; i < 7; i++ {
		out.Days = append(out.Days, *byDay[i])
		total += byDay[i].Kcal
	}
	out.AvgKcal = round0(total / 7)

	// Shopping items were stored in the language they were generated in;
	// translate them so switching language does not leave a half-German list.
	for i := range items {
		items[i].Name = recipes.TranslateIngredient(items[i].Name, rc.Lang)
		items[i].Unit = recipes.TranslateUnit(items[i].Unit, rc.Lang)
		items[i].Category = recipes.TranslateCategory(items[i].Category, rc.Lang)
	}

	return out, items, nil
}

func round0(v float64) float64 { return float64(int(v + 0.5)) }

func (s *Server) handleEntryCooked(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	id, err := pathInt(r, "id")
	if err != nil {
		return err
	}
	var req struct {
		Cooked bool `json:"cooked"`
	}
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	ctx := r.Context()
	if err := s.store.SetEntryCooked(ctx, store.HouseholdID, id, req.Cooked); err != nil {
		return err
	}
	plan := nutrition.Calculate(s.nutritionProfile(ctx, rc.User))
	stats, err := gamify.Compute(ctx, s.store, rc.User.UserID, rc.User, plan, rc.Now)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "gamify": stats})
	return nil
}

// handleLogPlannedMeal records a planned meal as eaten. Because the recipe's
// nutrition is known exactly, this is far more accurate than estimating the
// same plate from a photo afterwards.
func (s *Server) handleLogPlannedMeal(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	id, err := pathInt(r, "id")
	if err != nil {
		return err
	}
	var req struct {
		Day      string  `json:"day"`
		Portions float64 `json:"portions"`
	}
	if err := decodeJSON(r, &req); err != nil {
		return err
	}

	ctx := r.Context()
	week := parseWeek(r, rc.Now)
	planID, err := s.store.PlanID(ctx, store.HouseholdID, week.Format("2006-01-02"))
	if err != nil {
		return notFound("no meal plan for this week")
	}
	entries, err := s.store.PlanEntries(ctx, planID)
	if err != nil {
		return err
	}

	var found *store.PlanEntry
	for i := range entries {
		if entries[i].ID == id {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		return notFound("meal not found in this week's plan")
	}
	rec, ok := s.book.Get(found.RecipeID)
	if !ok {
		return notFound("recipe not found")
	}

	portions := req.Portions
	if portions <= 0 {
		portions = found.Servings
	}
	if portions <= 0 {
		portions = 1
	}
	day := req.Day
	if day == "" {
		day = week.AddDate(0, 0, found.DayIndex).Format("2006-01-02")
	}

	loc := recipes.Localize(rec, rc.Lang)
	entryID, err := s.store.AddFood(ctx, rc.User.UserID, store.FoodEntry{
		Day:        day,
		MealType:   found.MealType,
		Name:       loc.Title,
		Amount:     strconv.FormatFloat(portions, 'g', -1, 64) + " × " + portionWord(rc.Lang),
		Kcal:       round0(rec.Kcal * portions),
		ProteinG:   round0(rec.ProteinG * portions),
		CarbsG:     round0(rec.CarbsG * portions),
		FatG:       round0(rec.FatG * portions),
		Source:     "recipe",
		RecipeID:   rec.ID,
		Confidence: "high",
	})
	if err != nil {
		return err
	}
	if err := s.store.SetEntryCooked(ctx, store.HouseholdID, id, true); err != nil {
		return err
	}

	totals, err := s.store.TotalsForDay(ctx, rc.User.UserID, day)
	if err != nil {
		return err
	}
	plan := nutrition.Calculate(s.nutritionProfile(ctx, rc.User))
	stats, err := gamify.Compute(ctx, s.store, rc.User.UserID, rc.User, plan, rc.Now)
	if err != nil {
		return err
	}
	s.publishSensors(ctx, rc.User, plan)

	writeJSON(w, http.StatusOK, map[string]any{
		"id": entryID, "day": day, "totals": totals, "gamify": stats,
	})
	return nil
}

func portionWord(lang string) string {
	if recipes.NormalizeLang(lang) == recipes.LangEN {
		return "portion"
	}
	return "Portion"
}

func (s *Server) handleCheckShopping(w http.ResponseWriter, r *http.Request, rc *reqCtx) error {
	id, err := pathInt(r, "id")
	if err != nil {
		return err
	}
	var req struct {
		Checked bool `json:"checked"`
	}
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if err := s.store.SetShoppingChecked(r.Context(), store.HouseholdID, id, req.Checked); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}
