package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApp_servesCalorieCalculator(t *testing.T) {
	h := testPWAApp(t).Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/calculadora", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /calculadora status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `name="weight_kg"`) {
		t.Error("calculator is missing the weight field")
	}
	if !strings.Contains(body, `name="height_cm"`) {
		t.Error("calculator is missing the height field")
	}
	if !strings.Contains(body, `name="age_years"`) {
		t.Error("calculator is missing the age field")
	}
	if !strings.Contains(body, `name="sex"`) {
		t.Error("calculator is missing the sex field")
	}
	if !strings.Contains(body, `name="direction"`) {
		t.Error("calculator is missing secar / manter / aumentar")
	}
}
