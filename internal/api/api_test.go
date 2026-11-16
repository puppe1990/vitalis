package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/store"
	"github.com/puppe1990/vitalis/internal/testdata"
)

func newTestAPI(t *testing.T) (http.Handler, string, int64, store.Store) {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	userID, err := s.CreateUser("api@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.CreateAPIToken(userID, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SeedFoods(store.StarterFoods()); err != nil {
		t.Fatal(err)
	}

	return NewHandler(s, nil), token, userID, s
}

func doJSON(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestHandler_requiresBearerToken(t *testing.T) {
	h, _, _, _ := newTestAPI(t)

	cases := []struct {
		name   string
		token  string
		status int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"unknown", "vit_not_a_real_token", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := doJSON(t, h, http.MethodGet, "/api/v1/diary", tc.token, nil)
			if rr.Code != tc.status {
				t.Errorf("status = %d, want %d", rr.Code, tc.status)
			}
		})
	}
}

func TestHandler_diaryReturnsDayTotals(t *testing.T) {
	h, token, _, s := newTestAPI(t)
	userID, _ := s.FindUserIDByAPIToken(token)
	foods, _ := s.ListFoods("Peito de frango", "", 1)
	if len(foods) == 0 {
		t.Fatal("seeded food missing")
	}

	day := testdata.Day(2026, 11, 16)
	_, err := s.CreateEntry(models.Entry{
		UserID: userID, FoodID: foods[0].ID, Meal: nutrition.MealLunch,
		LoggedOn: day, Grams: 150,
		Nutrients: nutrition.Scale(foods[0].Per100g, 150),
	})
	if err != nil {
		t.Fatal(err)
	}

	rr := doJSON(t, h, http.MethodGet, "/api/v1/diary?d=2026-11-16", token, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got struct {
		Day    string `json:"date"`
		Totals struct {
			Kcal    float64 `json:"kcal"`
			Protein float64 `json:"protein_g"`
		} `json:"totals"`
		Balance struct {
			RemainingKcal float64 `json:"remaining_kcal"`
		} `json:"balance"`
		Meals []struct {
			Meal  string `json:"meal"`
			Count int    `json:"count"`
		} `json:"meals"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, rr.Body.String())
	}
	if got.Day != "2026-11-16" {
		t.Errorf("date = %q", got.Day)
	}
	if got.Totals.Kcal < 247 || got.Totals.Kcal > 249 {
		t.Errorf("totals.kcal = %v, want ~247.5", got.Totals.Kcal)
	}
	if got.Balance.RemainingKcal <= 0 {
		t.Errorf("remaining_kcal = %v, want positive", got.Balance.RemainingKcal)
	}

	var lunchCount int
	for _, m := range got.Meals {
		if m.Meal == string(nutrition.MealLunch) {
			lunchCount = m.Count
		}
	}
	if lunchCount != 1 {
		t.Errorf("lunch count = %d, want 1", lunchCount)
	}
}

func TestHandler_searchFoods(t *testing.T) {
	h, token, _, _ := newTestAPI(t)

	rr := doJSON(t, h, http.MethodGet, "/api/v1/foods?q=grelhado", token, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got struct {
		Foods []struct {
			ID       int64   `json:"id"`
			Name     string  `json:"name"`
			Kcal100g float64 `json:"kcal_100g"`
		} `json:"foods"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Foods) == 0 {
		t.Fatal("no foods matched 'grelhado'")
	}
	if got.Foods[0].ID == 0 || got.Foods[0].Kcal100g <= 0 {
		t.Errorf("food payload incomplete: %+v", got.Foods[0])
	}
}

func TestHandler_foodDetailIncludesPer100g(t *testing.T) {
	h, token, _, s := newTestAPI(t)
	foods, _ := s.ListFoods("Arroz", "", 1)

	rr := doJSON(t, h, http.MethodGet, "/api/v1/foods/"+itoa(foods[0].ID), token, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}

	var got struct {
		Name    string `json:"name"`
		Per100g struct {
			Kcal float64 `json:"kcal"`
		} `json:"per_100g"`
		Portion struct {
			Grams float64 `json:"grams"`
		} `json:"portion"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Per100g.Kcal != 130 {
		t.Errorf("per_100g.kcal = %v, want 130", got.Per100g.Kcal)
	}
	if got.Portion.Grams <= 0 {
		t.Errorf("portion.grams = %v", got.Portion.Grams)
	}
}

func TestHandler_logEntry_scalesFromPer100g(t *testing.T) {
	h, token, _, s := newTestAPI(t)
	foods, _ := s.ListFoods("Banana", "", 1)

	body := map[string]any{
		"food_id": foods[0].ID,
		"grams":   160,
		"meal":    "lanche",
		"date":    "2026-11-16",
	}
	rr := doJSON(t, h, http.MethodPost, "/api/v1/entries", token, body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got struct {
		Entry struct {
			ID    int64   `json:"id"`
			Kcal  float64 `json:"kcal"`
			Meal  string  `json:"meal"`
			Grams float64 `json:"grams"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Entry.ID == 0 {
		t.Fatal("entry id missing")
	}
	// 98 kcal/100g * 1.6 = 156.8
	if got.Entry.Kcal < 156 || got.Entry.Kcal > 158 {
		t.Errorf("kcal = %v, want ~156.8", got.Entry.Kcal)
	}
	if got.Entry.Meal != "lanche" {
		t.Errorf("meal = %q, want lanche", got.Entry.Meal)
	}

	userID, _ := s.FindUserIDByAPIToken(token)
	stored, _ := s.ListEntries(userID, testdata.Day(2026, 11, 16))
	if len(stored) != 1 {
		t.Errorf("stored entries = %d, want 1", len(stored))
	}
}

func TestHandler_logEntry_validatesInput(t *testing.T) {
	h, token, _, s := newTestAPI(t)
	foods, _ := s.ListFoods("Banana", "", 1)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing food", map[string]any{"grams": 100}},
		{"unknown food", map[string]any{"food_id": 99999, "grams": 100}},
		{"zero grams", map[string]any{"food_id": foods[0].ID, "grams": 0}},
		{"silly grams", map[string]any{"food_id": foods[0].ID, "grams": 90000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := doJSON(t, h, http.MethodPost, "/api/v1/entries", token, tc.body)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 (body %s)", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestHandler_deleteEntry(t *testing.T) {
	h, token, _, s := newTestAPI(t)
	userID, _ := s.FindUserIDByAPIToken(token)
	foods, _ := s.ListFoods("Banana", "", 1)
	day := testdata.Day(2026, 11, 16)

	entryID, err := s.CreateEntry(models.Entry{
		UserID: userID, FoodID: foods[0].ID, Meal: nutrition.MealSnack,
		LoggedOn: day, Grams: 80,
		Nutrients: nutrition.Scale(foods[0].Per100g, 80),
	})
	if err != nil {
		t.Fatal(err)
	}

	rr := doJSON(t, h, http.MethodDelete, "/api/v1/entries/"+itoa(entryID), token, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if left, _ := s.ListEntries(userID, day); len(left) != 0 {
		t.Errorf("entry survived delete: %d", len(left))
	}
}

func TestHandler_waterAccumulates(t *testing.T) {
	h, token, _, s := newTestAPI(t)

	body := map[string]any{"ml": 250, "date": "2026-11-16"}
	for i := 0; i < 2; i++ {
		rr := doJSON(t, h, http.MethodPost, "/api/v1/water", token, body)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
		}
	}

	userID, _ := s.FindUserIDByAPIToken(token)
	log, err := s.GetDayLog(userID, testdata.Day(2026, 11, 16))
	if err != nil {
		t.Fatal(err)
	}
	if log.WaterML != 500 {
		t.Errorf("WaterML = %v, want 500", log.WaterML)
	}
}

func TestHandler_goalRoundTrip(t *testing.T) {
	h, token, _, _ := newTestAPI(t)

	body := map[string]any{
		"sex": "female", "age_years": 32, "height_cm": 168,
		"weight_kg": 64, "activity": "moderate", "direction": "lose",
	}
	rr := doJSON(t, h, http.MethodPost, "/api/v1/goal", token, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var saved struct {
		Goal struct {
			TargetKcal float64 `json:"target_kcal"`
			ProteinG   float64 `json:"protein_g"`
			WaterML    float64 `json:"water_ml"`
		} `json:"goal"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Goal.TargetKcal < 1200 {
		t.Errorf("target_kcal = %v, want >= 1200", saved.Goal.TargetKcal)
	}

	read := doJSON(t, h, http.MethodGet, "/api/v1/goal", token, nil)
	if read.Code != http.StatusOK {
		t.Fatalf("read status = %d", read.Code)
	}
	var got struct {
		Goal struct {
			TargetKcal float64 `json:"target_kcal"`
		} `json:"goal"`
	}
	if err := json.Unmarshal(read.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Goal.TargetKcal != saved.Goal.TargetKcal {
		t.Errorf("read back %v, wrote %v", got.Goal.TargetKcal, saved.Goal.TargetKcal)
	}
}

func TestHandler_progressReturnsWeek(t *testing.T) {
	h, token, _, _ := newTestAPI(t)

	rr := doJSON(t, h, http.MethodGet, "/api/v1/progress?d=2026-11-16", token, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got struct {
		Days []struct {
			Label string `json:"label"`
		} `json:"days"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Days) != 7 {
		t.Errorf("days = %d, want 7", len(got.Days))
	}
}

func TestHandler_createsManualFood(t *testing.T) {
	h, token, _, s := newTestAPI(t)

	body := map[string]any{
		"name":          "Tapioca com queijo",
		"category":      "Cereais",
		"kcal_100g":     230,
		"protein_100g":  6,
		"carb_100g":     38,
		"fat_100g":      5,
		"portion_grams": 120,
	}
	rr := doJSON(t, h, http.MethodPost, "/api/v1/foods", token, body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	foods, err := s.ListFoods("Tapioca", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(foods) != 1 {
		t.Fatalf("stored foods = %d, want 1", len(foods))
	}
}

func TestHandler_rejectsAnotherUsersEntry(t *testing.T) {
	h, token, _, s := newTestAPI(t)

	other, err := s.CreateUser("other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	foods, _ := s.ListFoods("Banana", "", 1)
	entryID, err := s.CreateEntry(models.Entry{
		UserID: other, FoodID: foods[0].ID, Meal: nutrition.MealSnack,
		LoggedOn: testdata.Day(2026, 11, 16), Grams: 80,
		Nutrients: nutrition.Scale(foods[0].Per100g, 80),
	})
	if err != nil {
		t.Fatal(err)
	}

	rr := doJSON(t, h, http.MethodDelete, "/api/v1/entries/"+itoa(entryID), token, nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for another user's entry", rr.Code)
	}
}

// A client that knows its target must be able to pin it; the API recomputes
// macros from the pinned number and reports where the target came from.
func TestHandler_goalAcceptsPinnedTarget(t *testing.T) {
	h, token, _, _ := newTestAPI(t)

	body := map[string]any{
		"sex": "male", "age_years": 34, "height_cm": 180,
		"weight_kg": 93.2, "activity": "moderate", "direction": "lose",
		"target_kcal": 2500,
	}
	rr := doJSON(t, h, http.MethodPost, "/api/v1/goal", token, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got struct {
		Goal struct {
			TargetKcal     float64 `json:"target_kcal"`
			TargetOverride float64 `json:"target_override"`
			TargetSource   string  `json:"target_source"`
			ProteinG       float64 `json:"protein_g"`
		} `json:"goal"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Goal.TargetKcal != 2500 {
		t.Errorf("target_kcal = %v, want the pinned 2500", got.Goal.TargetKcal)
	}
	if got.Goal.TargetOverride != 2500 {
		t.Errorf("target_override = %v, want 2500", got.Goal.TargetOverride)
	}
	if got.Goal.TargetSource != "manual" {
		t.Errorf("target_source = %q, want manual", got.Goal.TargetSource)
	}
	if got.Goal.ProteinG <= 0 {
		t.Error("macros were not split from the pinned target")
	}

	read := doJSON(t, h, http.MethodGet, "/api/v1/goal", token, nil)
	var reread struct {
		Goal struct {
			TargetKcal float64 `json:"target_kcal"`
		} `json:"goal"`
	}
	if err := json.Unmarshal(read.Body.Bytes(), &reread); err != nil {
		t.Fatal(err)
	}
	if reread.Goal.TargetKcal != 2500 {
		t.Errorf("read back %v, want 2500", reread.Goal.TargetKcal)
	}
}

func TestHandler_goalRejectsAbsurdPinnedTarget(t *testing.T) {
	h, token, _, _ := newTestAPI(t)

	body := map[string]any{
		"sex": "male", "age_years": 34, "height_cm": 180,
		"weight_kg": 93.2, "activity": "moderate", "direction": "lose",
		"target_kcal": 25000,
	}
	rr := doJSON(t, h, http.MethodPost, "/api/v1/goal", token, body)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rr.Code)
	}
}

// Without a pinned target the budget must stay calculated.
func TestHandler_goalWithoutTargetStaysCalculated(t *testing.T) {
	h, token, _, _ := newTestAPI(t)

	body := map[string]any{
		"sex": "male", "age_years": 34, "height_cm": 180,
		"weight_kg": 93.2, "activity": "moderate", "direction": "lose",
	}
	rr := doJSON(t, h, http.MethodPost, "/api/v1/goal", token, body)
	var got struct {
		Goal struct {
			TargetSource string `json:"target_source"`
		} `json:"goal"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Goal.TargetSource != "calculated" {
		t.Errorf("target_source = %q, want calculated", got.Goal.TargetSource)
	}
}

// "Set my target to 2.500" must not require the whole profile: a client that
// only knows the number would otherwise invent height and age to get through
// validation, persisting a wrong profile.
func TestHandler_setTarget_keepsTheProfile(t *testing.T) {
	h, token, userID, s := newTestAPI(t)

	// Give the user a real profile first.
	if err := s.UpsertGoal(models.Goal{
		UserID: userID, TargetKcal: 2000, ProteinG: 150, CarbG: 200, FatG: 67,
		WaterML: 2800, Sex: nutrition.SexMale, AgeYears: 34, HeightCM: 180,
		WeightKG: 93.2, Activity: nutrition.ActivityModerate,
		Direction: nutrition.GoalLose,
	}); err != nil {
		t.Fatal(err)
	}

	rr := doJSON(t, h, http.MethodPost, "/api/v1/goal/target", token, map[string]any{"target_kcal": 2500})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got struct {
		Goal struct {
			TargetKcal     float64 `json:"target_kcal"`
			TargetOverride float64 `json:"target_override"`
			TargetSource   string  `json:"target_source"`
			HeightCM       float64 `json:"height_cm"`
			AgeYears       int     `json:"age_years"`
			WaterML        float64 `json:"water_ml"`
		} `json:"goal"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Goal.TargetKcal != 2500 || got.Goal.TargetOverride != 2500 {
		t.Errorf("target = %v / override = %v, want 2500", got.Goal.TargetKcal, got.Goal.TargetOverride)
	}
	if got.Goal.TargetSource != "manual" {
		t.Errorf("source = %q, want manual", got.Goal.TargetSource)
	}
	if got.Goal.HeightCM != 180 || got.Goal.AgeYears != 34 {
		t.Errorf("profile was clobbered: %+v", got.Goal)
	}
	if got.Goal.WaterML != 2800 {
		t.Errorf("water target = %v, want the stored 2800", got.Goal.WaterML)
	}
}

func TestHandler_setTarget_worksWithNoProfileYet(t *testing.T) {
	h, token, _, _ := newTestAPI(t)

	rr := doJSON(t, h, http.MethodPost, "/api/v1/goal/target", token, map[string]any{"target_kcal": 2500})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Goal struct {
			TargetKcal float64 `json:"target_kcal"`
		} `json:"goal"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Goal.TargetKcal != 2500 {
		t.Errorf("target = %v, want 2500", got.Goal.TargetKcal)
	}
}

func TestHandler_setTarget_rejectsAbsurdValues(t *testing.T) {
	h, token, _, _ := newTestAPI(t)

	for _, v := range []float64{500, 25000} {
		rr := doJSON(t, h, http.MethodPost, "/api/v1/goal/target", token, map[string]any{"target_kcal": v})
		if rr.Code != http.StatusUnprocessableEntity {
			t.Errorf("target %v: status = %d, want 422", v, rr.Code)
		}
	}
}
