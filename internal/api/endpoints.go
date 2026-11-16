package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/store"
)

func withUserID(r *http.Request, userID int64) context.Context {
	return context.WithValue(r.Context(), userIDKey{}, userID)
}

// maxBodyBytes caps a request body. The MCP client sends small JSON
// documents, so anything larger is a mistake or an attack, not a payload.
const maxBodyBytes = 64 << 10

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return false
	}
	return true
}

// diary answers the day summary: totals, balance, per-meal breakdown and
// hydration. It is the single call an MCP tool needs to answer "how am I doing
// today".
func (h *Handler) diary(w http.ResponseWriter, r *http.Request) {
	day, err := resolveDay(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	userID := userIDFrom(r)

	goal, err := h.store.GetGoal(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	entries, err := h.store.ListEntries(userID, day)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dayLog, err := h.store.GetDayLog(userID, day)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total := nutrition.Sum(nutrientsOf(entries)...)
	balance := nutrition.Balance(nutrition.BalanceInput{
		TargetKcal:   goal.TargetKcal,
		ConsumedKcal: total.Kcal,
		BurnedKcal:   dayLog.BurnedKcal,
	})

	type mealGroup struct {
		Meal    string      `json:"meal"`
		Entries []entryJSON `json:"entries"`
		Count   int         `json:"count"`
		Kcal    float64     `json:"kcal"`
	}

	byMeal := map[nutrition.Meal][]entryJSON{}
	for _, e := range entries {
		byMeal[e.Meal] = append(byMeal[e.Meal], toEntryJSON(e))
	}
	meals := make([]mealGroup, 0, len(nutrition.MealOrder()))
	for _, m := range nutrition.MealOrder() {
		items := byMeal[m]
		if items == nil {
			items = []entryJSON{}
		}
		var kcal float64
		for _, it := range items {
			kcal += it.Kcal
		}
		meals = append(meals, mealGroup{
			Meal: string(m), Entries: items, Count: len(items), Kcal: round1(kcal),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"date": day.Format(time.DateOnly),
		"totals": map[string]float64{
			"kcal":      round1(total.Kcal),
			"protein_g": round1(total.Protein),
			"carb_g":    round1(total.Carb),
			"fat_g":     round1(total.Fat),
		},
		"target": map[string]float64{
			"kcal":      goal.TargetKcal,
			"protein_g": goal.ProteinG,
			"carb_g":    goal.CarbG,
			"fat_g":     goal.FatG,
		},
		"balance": map[string]any{
			"target_kcal":    balance.TargetKcal,
			"consumed_kcal":  round1(balance.ConsumedKcal),
			"burned_kcal":    round1(balance.BurnedKcal),
			"net_kcal":       round1(balance.NetKcal),
			"remaining_kcal": round1(balance.RemainingKcal),
			"status":         balance.Status,
		},
		"water": map[string]float64{
			"drunk_ml":  dayLog.WaterML,
			"target_ml": goal.WaterML,
		},
		"meals": meals,
	})
}

func (h *Handler) searchFoods(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit := 20
	if raw := query.Get("limit"); raw != "" {
		if v, err := parsePositive(raw); err == nil {
			limit = v
		}
	}

	foods, err := h.store.ListFoods(query.Get("q"), query.Get("category"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]foodJSON, 0, len(foods))
	for _, f := range foods {
		out = append(out, toFoodJSON(f))
	}
	writeJSON(w, http.StatusOK, map[string]any{"foods": out, "count": len(out)})
}

func (h *Handler) foodDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	food, err := h.store.FindFood(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "food not found")
		return
	}

	portion := nutrition.Scale(food.Per100g, food.PortionGrams)
	writeJSON(w, http.StatusOK, map[string]any{
		"food": toFoodJSON(food),
		"per_100g": map[string]float64{
			"kcal":      food.Per100g.Kcal,
			"protein_g": food.Per100g.Protein,
			"carb_g":    food.Per100g.Carb,
			"fat_g":     food.Per100g.Fat,
		},
		"portion": map[string]any{
			"label":     food.PortionLabel,
			"grams":     food.PortionGrams,
			"kcal":      round1(portion.Kcal),
			"protein_g": round1(portion.Protein),
			"carb_g":    round1(portion.Carb),
			"fat_g":     round1(portion.Fat),
		},
	})
}

// createFoodRequest is the manual-food payload, mirroring the browser form.
type createFoodRequest struct {
	Name         string  `json:"name"`
	Brand        string  `json:"brand"`
	Category     string  `json:"category"`
	Kcal100g     float64 `json:"kcal_100g"`
	Protein100g  float64 `json:"protein_100g"`
	Carb100g     float64 `json:"carb_100g"`
	Fat100g      float64 `json:"fat_100g"`
	PortionLabel string  `json:"portion_label"`
	PortionGrams float64 `json:"portion_grams"`
}

func (h *Handler) createFood(w http.ResponseWriter, r *http.Request) {
	var req createFoodRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name is required")
		return
	}
	if req.Kcal100g <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "kcal_100g must be positive")
		return
	}
	if req.PortionGrams <= 0 {
		req.PortionGrams = 100
	}

	id, err := h.store.CreateFood(models.Food{
		Name: req.Name, Brand: req.Brand, Category: req.Category,
		Per100g: nutrition.Nutrients{
			Kcal: req.Kcal100g, Protein: req.Protein100g,
			Carb: req.Carb100g, Fat: req.Fat100g,
		},
		PortionLabel: req.PortionLabel,
		PortionGrams: req.PortionGrams,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	food, err := h.store.FindFood(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"food": toFoodJSON(food)})
}

type logEntryRequest struct {
	FoodID int64   `json:"food_id"`
	Grams  float64 `json:"grams"`
	Meal   string  `json:"meal"`
	Date   string  `json:"date"`
}

func (h *Handler) logEntry(w http.ResponseWriter, r *http.Request) {
	var req logEntryRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.FoodID <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "food_id is required")
		return
	}
	// The same 1-5000 g bounds the browser form uses, so an MCP client cannot
	// log a portion the UI would have rejected.
	if req.Grams < 1 || req.Grams > 5000 {
		writeError(w, http.StatusUnprocessableEntity, "grams must be between 1 and 5000")
		return
	}

	day := time.Now().UTC().Truncate(24 * time.Hour)
	if req.Date != "" {
		parsed, err := time.Parse(time.DateOnly, req.Date)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "date must be YYYY-MM-DD")
			return
		}
		day = parsed.UTC()
	}

	food, err := h.store.FindFood(req.FoodID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "food_id does not exist")
		return
	}

	entry := models.Entry{
		UserID:    userIDFrom(r),
		FoodID:    food.ID,
		Meal:      nutrition.NormalizeMeal(req.Meal),
		LoggedOn:  day,
		Grams:     req.Grams,
		Nutrients: nutrition.Scale(food.Per100g, req.Grams),
	}
	id, err := h.store.CreateEntry(entry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	entry.ID = id
	entry.FoodName = food.Name
	writeJSON(w, http.StatusCreated, map[string]any{"entry": toEntryJSON(entry)})
}

func (h *Handler) deleteEntry(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.DeleteEntry(userIDFrom(r), id); err != nil {
		// A cross-user id must read as 404, not 403: existence is not confirmed.
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "entry not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type waterRequest struct {
	ML   float64 `json:"ml"`
	Date string  `json:"date"`
}

func (h *Handler) addWater(w http.ResponseWriter, r *http.Request) {
	var req waterRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.ML == 0 {
		req.ML = nutrition.GlassML
	}

	day, err := resolveDay(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Date != "" {
		parsed, parseErr := time.Parse(time.DateOnly, req.Date)
		if parseErr != nil {
			writeError(w, http.StatusUnprocessableEntity, "date must be YYYY-MM-DD")
			return
		}
		day = parsed.UTC()
	}

	userID := userIDFrom(r)
	if err := h.store.AddWater(userID, day, req.ML); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	dayLog, err := h.store.GetDayLog(userID, day)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	goal, err := h.store.GetGoal(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	water := nutrition.Water{TargetML: goal.WaterML, DrunkML: dayLog.WaterML}
	writeJSON(w, http.StatusOK, map[string]any{
		"date":       day.Format(time.DateOnly),
		"drunk_ml":   water.DrunkML,
		"target_ml":  water.TargetML,
		"percent":    round1(water.Percent()),
		"glasses":    water.Glasses(),
		"difference": round1(water.TargetML - water.DrunkML),
	})
}

func (h *Handler) getGoal(w http.ResponseWriter, r *http.Request) {
	goal, err := h.store.GetGoal(userIDFrom(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"goal": toGoalJSON(goal)})
}

type saveGoalRequest struct {
	Sex       string  `json:"sex"`
	AgeYears  int     `json:"age_years"`
	HeightCM  float64 `json:"height_cm"`
	WeightKG  float64 `json:"weight_kg"`
	Activity  string  `json:"activity"`
	Direction string  `json:"direction"`
}

func (h *Handler) saveGoal(w http.ResponseWriter, r *http.Request) {
	var req saveGoalRequest
	if !decodeBody(w, r, &req) {
		return
	}

	profile := nutrition.Profile{
		Sex:      nutrition.Sex(req.Sex),
		AgeYears: req.AgeYears,
		HeightCM: req.HeightCM,
		WeightKG: req.WeightKG,
		Activity: nutrition.Activity(req.Activity),
		Goal:     nutrition.Goal(req.Direction),
	}
	if err := validateProfile(profile); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	goals := nutrition.GoalsFor(profile)
	userID := userIDFrom(r)
	goal := models.Goal{
		UserID: userID, TargetKcal: goals.TargetKcal,
		ProteinG: goals.ProteinG, CarbG: goals.CarbG, FatG: goals.FatG,
		WaterML: goals.WaterML, Sex: profile.Sex, AgeYears: profile.AgeYears,
		HeightCM: profile.HeightCM, WeightKG: profile.WeightKG,
		Activity: profile.Activity, Direction: profile.Goal,
	}
	if err := h.store.UpsertGoal(goal); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.store.SaveWeight(userID, time.Now().UTC(), profile.WeightKG); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"goal": toGoalJSON(goal)})
}

func (h *Handler) progress(w http.ResponseWriter, r *http.Request) {
	end, err := resolveDay(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	userID := userIDFrom(r)

	goal, err := h.store.GetGoal(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	labels := []string{"Seg", "Ter", "Qua", "Qui", "Sex", "Sáb", "Dom"}
	days := make([]nutrition.DayTotal, 0, 7)
	start := end.AddDate(0, 0, -6)
	for i := 0; i < 7; i++ {
		day := start.AddDate(0, 0, i)
		entries, err := h.store.ListEntries(userID, day)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		days = append(days, nutrition.DayTotal{
			Label:  labels[int(day.Weekday()+6)%7],
			Kcal:   round1(nutrition.Sum(nutrientsOf(entries)...).Kcal),
			Target: goal.TargetKcal,
		})
	}

	summary := nutrition.Weekly(days)
	writeJSON(w, http.StatusOK, map[string]any{
		"start":         start.Format(time.DateOnly),
		"end":           end.Format(time.DateOnly),
		"days":          summary.Days,
		"average_kcal":  round1(summary.AverageKcal),
		"target_kcal":   round1(summary.TargetKcal),
		"adherence_pct": round1(summary.AdherencePercent),
		"balance_kcal":  round1(summary.BalanceKcal),
		"best_day":      summary.BestDay,
		"worst_day":     summary.WorstDay,
	})
}

func (h *Handler) listWeight(w http.ResponseWriter, r *http.Request) {
	limit := 30
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := parsePositive(raw); err == nil {
			limit = v
		}
	}
	weights, err := h.store.ListWeights(userIDFrom(r), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]map[string]any, 0, len(weights))
	for _, wt := range weights {
		out = append(out, map[string]any{
			"date": wt.LoggedOn.Format(time.DateOnly),
			"kg":   round1(wt.KG),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"weights": out, "count": len(out)})
}

type saveWeightRequest struct {
	KG   float64 `json:"kg"`
	Date string  `json:"date"`
}

func (h *Handler) saveWeight(w http.ResponseWriter, r *http.Request) {
	var req saveWeightRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.KG < 35 || req.KG > 300 {
		writeError(w, http.StatusUnprocessableEntity, "kg must be between 35 and 300")
		return
	}

	day, err := resolveDay(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Date != "" {
		parsed, parseErr := time.Parse(time.DateOnly, req.Date)
		if parseErr != nil {
			writeError(w, http.StatusUnprocessableEntity, "date must be YYYY-MM-DD")
			return
		}
		day = parsed.UTC()
	}

	if err := h.store.SaveWeight(userIDFrom(r), day, req.KG); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date": day.Format(time.DateOnly),
		"kg":   round1(req.KG),
	})
}

func nutrientsOf(entries []models.Entry) []nutrition.Nutrients {
	out := make([]nutrition.Nutrients, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Nutrients)
	}
	return out
}
