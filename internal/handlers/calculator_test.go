package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	appi18n "github.com/puppe1990/vitalis/internal/i18n"
	"github.com/puppe1990/vitalis/internal/nutrition"
)

func newCalculatorHandler(t *testing.T) *CalculatorHandler {
	t.Helper()
	return NewCalculatorHandler(setupTestViews(t), testSite(), appi18n.DefaultCatalog(), cais.Config{})
}

func TestCalculator_guestFormAsksWeightHeightAgeAndSex(t *testing.T) {
	h := newCalculatorHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/calculadora", nil)
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rr.Code, firstLines(rr.Body.String()))
	}
	body := rr.Body.String()
	for _, want := range []string{
		`name="weight_kg"`,
		`name="height_cm"`,
		`name="age_years"`,
		`name="sex"`,
		`action="/calculadora"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("form missing %q", want)
		}
	}
	if strings.Contains(body, `data-testid="calc-result"`) {
		t.Error("empty GET must not show a calorie result")
	}
	if strings.Contains(body, `name="activity"`) {
		t.Error("guest calculator must not ask for activity; only weight, height, age and sex")
	}
	if strings.Contains(body, `name="direction"`) {
		t.Error("guest calculator must not ask for a goal direction")
	}
}

func TestCalculator_postShowsMaintenanceCaloriesFromMifflin(t *testing.T) {
	h := newCalculatorHandler(t)

	form := url.Values{
		"sex":       {"male"},
		"age_years": {"30"},
		"height_cm": {"180"},
		"weight_kg": {"82"},
	}
	req := httptest.NewRequest(http.MethodPost, "/calculadora", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Calculate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rr.Code, firstLines(rr.Body.String()))
	}
	body := rr.Body.String()
	if !strings.Contains(body, `data-testid="calc-result"`) {
		t.Fatal("missing calorie result")
	}
	// Mifflin-St Jeor male 30y 180 cm 82 kg: BMR 1800, TDEE moderate 2790.
	want := nutrition.GoalsFor(nutrition.Profile{
		Sex:      nutrition.SexMale,
		AgeYears: 30,
		HeightCM: 180,
		WeightKG: 82,
		Activity: nutrition.ActivityModerate,
		Goal:     nutrition.GoalMaintain,
	})
	if want.TargetKcal != 2790 || want.BMR != 1800 {
		t.Fatalf("fixture drifted: TargetKcal=%v BMR=%v", want.TargetKcal, want.BMR)
	}
	if !strings.Contains(body, formatNumber(want.TargetKcal)) {
		t.Errorf("result missing maintenance %s kcal", formatNumber(want.TargetKcal))
	}
	if !strings.Contains(body, formatNumber(want.BMR)) {
		t.Errorf("result missing BMR %s kcal", formatNumber(want.BMR))
	}
}

func TestCalculator_rejectsOutOfRangeWeight(t *testing.T) {
	h := newCalculatorHandler(t)

	form := url.Values{
		"sex":       {"female"},
		"age_years": {"28"},
		"height_cm": {"165"},
		"weight_kg": {"10"},
	}
	req := httptest.NewRequest(http.MethodPost, "/calculadora", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Calculate(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, `data-testid="calc-result"`) {
		t.Error("invalid input must not show a calorie result")
	}
	if !strings.Contains(body, "peso") && !strings.Contains(body, "weight") {
		t.Error("expected a weight validation message")
	}
}

func TestCalculator_doesNotWriteAGoal(t *testing.T) {
	env := newTestEnv(t)
	h := newCalculatorHandler(t)

	before, err := env.store.GetGoal(env.userID)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"sex":       {"male"},
		"age_years": {"34"},
		"height_cm": {"180"},
		"weight_kg": {"93.2"},
	}
	req := httptest.NewRequest(http.MethodPost, "/calculadora", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = session.WithUserID(req, env.userID)
	rr := httptest.NewRecorder()
	h.Calculate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	after, err := env.store.GetGoal(env.userID)
	if err != nil {
		t.Fatal(err)
	}
	if after.TargetKcal != before.TargetKcal || after.TargetOverride != before.TargetOverride {
		t.Errorf("calculator wrote a goal: before=%+v after=%+v", before, after)
	}
}
