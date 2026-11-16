package nutrition

import (
	"math"
	"testing"
	"time"
)

// approx compares floats with the tolerance a kcal/g display can absorb, so a
// rounding change in Scale does not turn a correct percentage into a failure.
func approx(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.4f, want %.4f (±%.4f)", label, got, want, tol)
	}
}

func TestScaleNutrients_per100gProfile(t *testing.T) {
	food := Nutrients{Kcal: 165, Protein: 31, Carb: 0, Fat: 3.6}
	got := Scale(food, 150)

	approx(t, "kcal", got.Kcal, 247.5, 0.01)
	approx(t, "protein", got.Protein, 46.5, 0.01)
	approx(t, "carb", got.Carb, 0, 0.01)
	approx(t, "fat", got.Fat, 5.4, 0.01)
}

func TestScaleNutrients_zeroGramsIsZero(t *testing.T) {
	got := Scale(Nutrients{Kcal: 900, Protein: 10, Carb: 10, Fat: 10}, 0)
	if got.Kcal != 0 || got.Protein != 0 || got.Carb != 0 || got.Fat != 0 {
		t.Errorf("Scale(0g) = %+v, want all zero", got)
	}
}

func TestScaleNutrients_negativeGramsIsZero(t *testing.T) {
	got := Scale(Nutrients{Kcal: 900}, -50)
	if got.Kcal != 0 {
		t.Errorf("Scale(-50g).Kcal = %v, want 0", got.Kcal)
	}
}

func TestSumTotals_addsEveryEntry(t *testing.T) {
	totals := Sum(
		Nutrients{Kcal: 248, Protein: 46.5, Carb: 0, Fat: 5.4},
		Nutrients{Kcal: 130, Protein: 2.5, Carb: 28, Fat: 0.3},
		Nutrients{Kcal: 78, Protein: 1, Carb: 20, Fat: 0.1},
	)

	approx(t, "kcal", totals.Kcal, 456, 0.01)
	approx(t, "protein", totals.Protein, 50, 0.01)
	approx(t, "carb", totals.Carb, 48, 0.01)
	approx(t, "fat", totals.Fat, 5.8, 0.01)
}

func TestSumTotals_emptyIsZero(t *testing.T) {
	if got := Sum(); got.Kcal != 0 {
		t.Errorf("Sum() = %+v, want zero", got)
	}
}

func TestRemaining_subtractsConsumedFromTarget(t *testing.T) {
	rem := Remaining(2100, 1450)
	if rem != 650 {
		t.Errorf("Remaining = %v, want 650", rem)
	}
}

func TestRemaining_canGoNegativeOnOvershoot(t *testing.T) {
	rem := Remaining(2100, 2400)
	if rem != -300 {
		t.Errorf("Remaining = %v, want -300", rem)
	}
}

func TestMacroShare_percentOfKcal(t *testing.T) {
	// 46.5*4 + 50*4 + 5.8*9 = 186 + 200 + 52.2 = 438.2 kcal
	shares := MacroShare(Nutrients{Protein: 46.5, Carb: 50, Fat: 5.8})

	approx(t, "protein %", shares.Protein, 186.0/438.2*100, 0.01)
	approx(t, "carb %", shares.Carb, 200.0/438.2*100, 0.01)
	approx(t, "fat %", shares.Fat, 52.2/438.2*100, 0.01)
	approx(t, "sum", shares.Protein+shares.Carb+shares.Fat, 100, 0.01)
}

func TestMacroShare_emptyHasNoNaN(t *testing.T) {
	shares := MacroShare(Nutrients{})
	if math.IsNaN(shares.Protein) || math.IsNaN(shares.Carb) || math.IsNaN(shares.Fat) {
		t.Errorf("MacroShare(zero) produced NaN: %+v", shares)
	}
	if shares.Protein != 0 || shares.Carb != 0 || shares.Fat != 0 {
		t.Errorf("MacroShare(zero) = %+v, want zero", shares)
	}
}

func TestProgressRatio_clampsTo100(t *testing.T) {
	if got := ProgressRatio(2400, 2400); got != 100 {
		t.Errorf("ProgressRatio(exact) = %v, want 100", got)
	}
	if got := ProgressRatio(3000, 2400); got != 100 {
		t.Errorf("ProgressRatio(over) = %v, want 100", got)
	}
	if got := ProgressRatio(1200, 2400); got != 50 {
		t.Errorf("ProgressRatio(half) = %v, want 50", got)
	}
}

func TestProgressRatio_zeroTargetIsZero(t *testing.T) {
	if got := ProgressRatio(500, 0); got != 0 {
		t.Errorf("ProgressRatio(_, 0) = %v, want 0", got)
	}
}

func TestBalance_burnsReduceConsumed(t *testing.T) {
	got := Balance(BalanceInput{
		TargetKcal:   2100,
		ConsumedKcal: 1450,
		BurnedKcal:   340,
	})
	// 2100 - (1450 - 340) = 990
	if got.RemainingKcal != 990 {
		t.Errorf("RemainingKcal = %v, want 990", got.RemainingKcal)
	}
	if got.NetKcal != 1110 {
		t.Errorf("NetKcal = %v, want 1110", got.NetKcal)
	}
}

func TestBalance_statusLabels(t *testing.T) {
	cases := []struct {
		name  string
		input BalanceInput
		want  string
	}{
		{"moderate deficit", BalanceInput{TargetKcal: 2100, ConsumedKcal: 1450}, StatusDeficitModerate},
		{"light deficit", BalanceInput{TargetKcal: 2100, ConsumedKcal: 1900}, StatusDeficitLight},
		{"aggressive deficit", BalanceInput{TargetKcal: 2100, ConsumedKcal: 1150}, StatusDeficitHigh},
		{"on target", BalanceInput{TargetKcal: 2100, ConsumedKcal: 2060}, StatusOnTarget},
		{"surplus", BalanceInput{TargetKcal: 2100, ConsumedKcal: 2450}, StatusSurplus},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Balance(tc.input).Status; got != tc.want {
				t.Errorf("Status = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWater_glassesFromML(t *testing.T) {
	got := Water{TargetML: 2500, DrunkML: 1000}
	if got.Glasses() != 4 {
		t.Errorf("Glasses() = %d, want 4", got.Glasses())
	}
	if got.GlassesTarget() != 10 {
		t.Errorf("GlassesTarget() = %d, want 10", got.GlassesTarget())
	}
	approx(t, "percent", got.Percent(), 40, 0.01)
}

func TestWater_neverExceeds100Percent(t *testing.T) {
	got := Water{TargetML: 2000, DrunkML: 3000}
	if got.Percent() != 100 {
		t.Errorf("Percent() = %v, want 100", got.Percent())
	}
}

func TestWater_zeroTargetHasNoNaN(t *testing.T) {
	got := Water{TargetML: 0, DrunkML: 500}
	if got.Percent() != 0 || got.GlassesTarget() != 0 {
		t.Errorf("zero target = %v pct / %d glasses, want 0", got.Percent(), got.GlassesTarget())
	}
}

func TestWeekly_averagesAndAdherence(t *testing.T) {
	days := []DayTotal{
		{Label: "Seg", Kcal: 1820, Target: 1950},
		{Label: "Ter", Kcal: 1900, Target: 1950},
		{Label: "Qua", Kcal: 1880, Target: 1950},
		{Label: "Qui", Kcal: 2120, Target: 1950},
		{Label: "Sex", Kcal: 1790, Target: 1950},
		{Label: "Sáb", Kcal: 2010, Target: 1950},
		{Label: "Dom", Kcal: 1710, Target: 1950},
	}
	got := Weekly(days)

	approx(t, "average", got.AverageKcal, 1890, 0.01)
	if got.BestDay.Label != "Dom" {
		t.Errorf("BestDay = %q, want Dom (lowest kcal)", got.BestDay.Label)
	}
	if got.WorstDay.Label != "Qui" {
		t.Errorf("WorstDay = %q, want Qui (highest kcal)", got.WorstDay.Label)
	}
	// 13230 eaten against 13650 target → -420, matching the Stitch tile's
	// signed "Déficit Total" convention where under-target is negative.
	if got.BalanceKcal != -420 {
		t.Errorf("BalanceKcal = %v, want -420", got.BalanceKcal)
	}
}

func TestWeekly_adherenceCountsDaysWithinTarget(t *testing.T) {
	days := []DayTotal{
		{Kcal: 1900, Target: 2000}, // within 10%
		{Kcal: 2100, Target: 2000}, // within 10%
		{Kcal: 2600, Target: 2000}, // over
	}
	got := Weekly(days)
	approx(t, "adherence", got.AdherencePercent, 200.0/3, 0.01)
}

func TestWeekly_emptyIsZero(t *testing.T) {
	got := Weekly(nil)
	if got.AverageKcal != 0 || got.AdherencePercent != 0 || got.BestDay.Label != "" {
		t.Errorf("Weekly(nil) = %+v, want zero", got)
	}
}

// A week with mostly empty days is absence of data, not a heroic deficit. This
// is the regression guard for the 10.500 kcal phantom deficit the first render
// showed on an untouched week.
func TestWeekly_ignoresDaysWithNoEntries(t *testing.T) {
	days := []DayTotal{
		{Label: "Seg", Kcal: 0, Target: 1622},
		{Label: "Ter", Kcal: 0, Target: 1622},
		{Label: "Qua", Kcal: 0, Target: 1622},
		{Label: "Qui", Kcal: 0, Target: 1622},
		{Label: "Sex", Kcal: 0, Target: 1622},
		{Label: "Sáb", Kcal: 0, Target: 1622},
		{Label: "Dom", Kcal: 854, Target: 1622},
	}
	got := Weekly(days)

	approx(t, "average", got.AverageKcal, 854, 0.01)
	approx(t, "adherence", got.AdherencePercent, 0, 0.01)
	approx(t, "balance", got.BalanceKcal, 854-1622, 0.01)
	if got.BestDay.Label != "Dom" || got.WorstDay.Label != "Dom" {
		t.Errorf("best/worst = %q/%q, want both Dom", got.BestDay.Label, got.WorstDay.Label)
	}
}

func TestWeekly_allEmptyIsZero(t *testing.T) {
	days := []DayTotal{
		{Label: "Seg", Kcal: 0, Target: 1622},
		{Label: "Ter", Kcal: 0, Target: 1622},
	}
	got := Weekly(days)
	if got.AverageKcal != 0 || got.AdherencePercent != 0 || got.BalanceKcal != 0 {
		t.Errorf("Weekly(empty) = %+v, want zeros", got)
	}
	if got.BestDay.Label != "" || got.WorstDay.Label != "" {
		t.Errorf("empty week named a best/worst day: %q/%q", got.BestDay.Label, got.WorstDay.Label)
	}
}

// Both the dashboard badge and the MCP server read DisplayStatus, so the day
// cannot be "in progress" in one surface and "steep deficit" in the other.
func TestDisplayStatus_hedgesUntilTheDayIsOver(t *testing.T) {
	day := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	morning := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	late := time.Date(2026, 11, 16, 22, 0, 0, 0, time.UTC)

	if got := DisplayStatus(StatusDeficitHigh, 1, day, morning); got != DisplayInProgress {
		t.Errorf("morning = %q, want %q", got, DisplayInProgress)
	}
	if got := DisplayStatus(StatusDeficitHigh, 1, day, late); got != StatusDeficitHigh {
		t.Errorf("late = %q, want %q", got, StatusDeficitHigh)
	}
	if got := DisplayStatus(StatusDeficitHigh, 0, day, late); got != DisplayNotStarted {
		t.Errorf("empty = %q, want %q", got, DisplayNotStarted)
	}
}

func TestDisplayStatus_pastDayKeepsItsBand(t *testing.T) {
	past := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	if got := DisplayStatus(StatusSurplus, 2, past, now); got != StatusSurplus {
		t.Errorf("past day = %q, want %q", got, StatusSurplus)
	}
}
