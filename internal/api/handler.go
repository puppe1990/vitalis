// Package api is Vitalis's machine-facing JSON surface.
//
// The MCP server (mcp/) is its only consumer, but keeping the transport as a
// plain bearer-authenticated HTTP API means the same contract serves a curl
// script, a shortcut or a future mobile client without touching the MCP code.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/store"
)

// Handler is the /api/v1 mux. It is mounted under the app's CSRF-exempt prefix
// and authenticates every request with a bearer token.
type Handler struct {
	store store.Store
	mux   *http.ServeMux
}

// Prefix is where the app mounts this handler. The prefix lives here rather
// than at the call site so the routes and their tests cannot drift apart.
const Prefix = "/api/v1"

// NewHandler wires the routes under Prefix.
func NewHandler(s store.Store) http.Handler {
	h := &Handler{store: s, mux: http.NewServeMux()}

	h.mux.HandleFunc("GET /api/v1/diary", h.auth(h.diary))
	h.mux.HandleFunc("GET /api/v1/foods", h.auth(h.searchFoods))
	h.mux.HandleFunc("POST /api/v1/foods", h.auth(h.createFood))
	h.mux.HandleFunc("GET /api/v1/foods/{id}", h.auth(h.foodDetail))
	h.mux.HandleFunc("POST /api/v1/entries", h.auth(h.logEntry))
	h.mux.HandleFunc("DELETE /api/v1/entries/{id}", h.auth(h.deleteEntry))
	h.mux.HandleFunc("POST /api/v1/water", h.auth(h.addWater))
	h.mux.HandleFunc("GET /api/v1/goal", h.auth(h.getGoal))
	h.mux.HandleFunc("POST /api/v1/goal", h.auth(h.saveGoal))
	h.mux.HandleFunc("GET /api/v1/progress", h.auth(h.progress))
	h.mux.HandleFunc("GET /api/v1/weight", h.auth(h.listWeight))
	h.mux.HandleFunc("POST /api/v1/weight", h.auth(h.saveWeight))

	return h.mux
}

// userIDKey carries the authenticated user between the middleware and the
// handler. A private key type keeps another package from spoofing it.
type userIDKey struct{}

// auth resolves the bearer token to a user or answers 401. Every route is
// wrapped, so an unauthenticated request can never reach a handler.
func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		userID, ok := h.store.FindUserIDByAPIToken(token)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid or missing bearer token")
			return
		}
		next(w, r.WithContext(withUserID(r, userID)))
	}
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return ""
	}
	return header[len(prefix):]
}

func userIDFrom(r *http.Request) int64 {
	id, _ := r.Context().Value(userIDKey{}).(int64)
	return id
}

// resolveDay reads ?d=YYYY-MM-DD, defaulting to today. The MCP client always
// sends an explicit date so a long-running session cannot drift across
// midnight.
func resolveDay(r *http.Request) (time.Time, error) {
	raw := r.URL.Query().Get("d")
	if raw == "" {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q must be YYYY-MM-DD: %w", raw, err)
	}
	return t.UTC(), nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

// foodJSON is the API's food shape. Per-100g values are flattened so an MCP
// tool can read one object instead of nesting a second.
type foodJSON struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Brand        string  `json:"brand,omitempty"`
	Category     string  `json:"category,omitempty"`
	Kcal100g     float64 `json:"kcal_100g"`
	Protein100g  float64 `json:"protein_100g"`
	Carb100g     float64 `json:"carb_100g"`
	Fat100g      float64 `json:"fat_100g"`
	PortionLabel string  `json:"portion_label,omitempty"`
	PortionGrams float64 `json:"portion_grams"`
	Verified     bool    `json:"verified"`
}

func toFoodJSON(f models.Food) foodJSON {
	return foodJSON{
		ID: f.ID, Name: f.Name, Brand: f.Brand, Category: f.Category,
		Kcal100g: f.Per100g.Kcal, Protein100g: f.Per100g.Protein,
		Carb100g: f.Per100g.Carb, Fat100g: f.Per100g.Fat,
		PortionLabel: f.PortionLabel, PortionGrams: f.PortionGrams,
		Verified: f.Verified,
	}
}

// entryJSON flattens an entry for the client, keeping the joined food name so
// a diary listing renders without a second call per row.
type entryJSON struct {
	ID       int64   `json:"id"`
	FoodID   int64   `json:"food_id"`
	FoodName string  `json:"food_name"`
	Meal     string  `json:"meal"`
	Date     string  `json:"date"`
	Grams    float64 `json:"grams"`
	Kcal     float64 `json:"kcal"`
	Protein  float64 `json:"protein_g"`
	Carb     float64 `json:"carb_g"`
	Fat      float64 `json:"fat_g"`
}

func toEntryJSON(e models.Entry) entryJSON {
	return entryJSON{
		ID: e.ID, FoodID: e.FoodID, FoodName: e.FoodName,
		Meal: string(e.Meal), Date: e.LoggedOn.Format(time.DateOnly),
		Grams: e.Grams, Kcal: round1(e.Nutrients.Kcal),
		Protein: round1(e.Nutrients.Protein), Carb: round1(e.Nutrients.Carb),
		Fat: round1(e.Nutrients.Fat),
	}
}

// round1 keeps the JSON readable: 156.80000000000001 is not a calorie count.
func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// goalJSON is the budget plus what it was derived from.
type goalJSON struct {
	// TargetSource is "manual" when the user pinned the target, "calculated"
	// when it came from the profile formula. target_override carries the pinned
	// number so a client can show or clear it.
	TargetSource   string  `json:"target_source"`
	TargetOverride float64 `json:"target_override"`
	TargetKcal     float64 `json:"target_kcal"`
	ProteinG       float64 `json:"protein_g"`
	CarbG          float64 `json:"carb_g"`
	FatG           float64 `json:"fat_g"`
	WaterML        float64 `json:"water_ml"`
	Sex            string  `json:"sex"`
	AgeYears       int     `json:"age_years"`
	HeightCM       float64 `json:"height_cm"`
	WeightKG       float64 `json:"weight_kg"`
	Activity       string  `json:"activity"`
	Direction      string  `json:"direction"`
}

func toGoalJSON(g models.Goal) goalJSON {
	return goalJSON{
		TargetSource:   nutrition.TargetSource(g.TargetOverride),
		TargetOverride: g.TargetOverride,
		TargetKcal:     g.TargetKcal, ProteinG: g.ProteinG, CarbG: g.CarbG, FatG: g.FatG,
		WaterML: g.WaterML, Sex: string(g.Sex), AgeYears: g.AgeYears,
		HeightCM: g.HeightCM, WeightKG: g.WeightKG,
		Activity: string(g.Activity), Direction: string(g.Direction),
	}
}
