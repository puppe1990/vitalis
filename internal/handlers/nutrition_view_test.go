package handlers

import (
	"testing"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/testdata"
)

func TestBuildDayView_sumsEntriesIntoBalance(t *testing.T) {
	goal := models.Goal{TargetKcal: 2100, ProteinG: 158, CarbG: 158, FatG: 70, WaterML: 2500}
	entries := []models.Entry{
		{Meal: nutrition.MealBreakfast, Nutrients: nutrition.Nutrients{Kcal: 420, Protein: 24, Carb: 28, Fat: 14}},
		{Meal: nutrition.MealLunch, Nutrients: nutrition.Nutrients{Kcal: 680, Protein: 58, Carb: 75, Fat: 16}},
		{Meal: nutrition.MealSnack, Nutrients: nutrition.Nutrients{Kcal: 350, Protein: 6, Carb: 14, Fat: 25}},
	}
	dayLog := models.DayLog{BurnedKcal: 340}
	day := testdata.Day(2026, 11, 16)

	got := buildDayView(goal, entries, dayLog, day)

	if got.Total.Kcal != 1450 {
		t.Errorf("Total.Kcal = %v, want 1450", got.Total.Kcal)
	}
	if got.Balance.RemainingKcal != 990 {
		t.Errorf("RemainingKcal = %v, want 990", got.Balance.RemainingKcal)
	}
	if got.Balance.Status != nutrition.StatusDeficitModerate {
		t.Errorf("Status = %q, want moderate", got.Balance.Status)
	}
	if got.MealCount != 3 {
		t.Errorf("MealCount = %d, want 3", got.MealCount)
	}
}

func TestBuildDayView_alwaysReturnsFourMealCards(t *testing.T) {
	got := buildDayView(models.Goal{TargetKcal: 2000}, nil, models.DayLog{}, testdata.Day(2026, 11, 16))
	if len(got.Meals) != 4 {
		t.Fatalf("Meals = %d, want 4", len(got.Meals))
	}
	for i, m := range got.Meals {
		if m.Meal != nutrition.MealOrder()[i] {
			t.Errorf("meal %d = %q, want %q", i, m.Meal, nutrition.MealOrder()[i])
		}
		if m.HasItems {
			t.Errorf("empty day marked %q as having items", m.Meal)
		}
	}
}

func TestBuildDayView_groupsEntriesIntoTheirMeal(t *testing.T) {
	entries := []models.Entry{
		{Meal: nutrition.MealLunch, Nutrients: nutrition.Nutrients{Kcal: 500}},
		{Meal: nutrition.MealLunch, Nutrients: nutrition.Nutrients{Kcal: 200}},
		{Meal: nutrition.MealDinner, Nutrients: nutrition.Nutrients{Kcal: 400}},
	}
	got := buildDayView(models.Goal{TargetKcal: 2000}, entries, models.DayLog{}, testdata.Day(2026, 11, 16))

	for _, m := range got.Meals {
		switch m.Meal {
		case nutrition.MealLunch:
			if len(m.Entries) != 2 || m.Kcal != 700 {
				t.Errorf("lunch = %d entries / %v kcal, want 2 / 700", len(m.Entries), m.Kcal)
			}
		case nutrition.MealDinner:
			if len(m.Entries) != 1 || m.Kcal != 400 {
				t.Errorf("dinner = %d entries / %v kcal, want 1 / 400", len(m.Entries), m.Kcal)
			}
		default:
			if len(m.Entries) != 0 {
				t.Errorf("%q has stray entries", m.Meal)
			}
		}
	}
}

func TestBuildDayView_navigationMovesOneDay(t *testing.T) {
	day := testdata.Day(2026, 11, 16)
	got := buildDayView(models.Goal{TargetKcal: 2000}, nil, models.DayLog{}, day)

	if !got.PreviousDay.Equal(testdata.Day(2026, 11, 15)) {
		t.Errorf("PreviousDay = %v", got.PreviousDay)
	}
	if !got.NextDay.Equal(testdata.Day(2026, 11, 17)) {
		t.Errorf("NextDay = %v", got.NextDay)
	}
}

func TestBuildDayView_hydrationUsesDayLog(t *testing.T) {
	goal := models.Goal{TargetKcal: 2000, WaterML: 2500}
	got := buildDayView(goal, nil, models.DayLog{WaterML: 1750}, testdata.Day(2026, 11, 16))

	if got.Water.DrunkML != 1750 {
		t.Errorf("DrunkML = %v, want 1750", got.Water.DrunkML)
	}
	if got.Water.Percent() != 70 {
		t.Errorf("Percent = %v, want 70", got.Water.Percent())
	}
}

func TestBuildDayView_macroBarsClampAt100(t *testing.T) {
	goal := models.Goal{TargetKcal: 2000, ProteinG: 100, CarbG: 100, FatG: 50}
	entries := []models.Entry{
		{Nutrients: nutrition.Nutrients{Kcal: 2000, Protein: 250, Carb: 50, Fat: 50}},
	}
	got := buildDayView(goal, entries, models.DayLog{}, testdata.Day(2026, 11, 16))

	for _, m := range got.Macros {
		if m.Percent > 100 {
			t.Errorf("macro %q percent = %v, want clamped at 100", m.Key, m.Percent)
		}
		if m.LeftPct < 0 {
			t.Errorf("macro %q LeftPct = %v, want >= 0", m.Key, m.LeftPct)
		}
	}
}

func TestFormatNumber_brazilianThousands(t *testing.T) {
	cases := map[float64]string{
		0:      "0",
		650:    "650",
		1890:   "1.890",
		13650:  "13.650",
		-2450:  "-2.450",
		123456: "123.456",
	}
	for in, want := range cases {
		if got := formatNumber(in); got != want {
			t.Errorf("formatNumber(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatGrams_keepsOneDecimalWithComma(t *testing.T) {
	cases := map[float64]string{
		46.5: "46,5",
		100:  "100",
		0.3:  "0,3",
		5.8:  "5,8",
	}
	for in, want := range cases {
		if got := formatGrams(in); got != want {
			t.Errorf("formatGrams(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatML_switchesToLitresAt1000(t *testing.T) {
	cases := map[float64]string{
		250:  "250 ml",
		1800: "1,8L",
		2000: "2L",
	}
	for in, want := range cases {
		if got := formatML(in); got != want {
			t.Errorf("formatML(%v) = %q, want %q", in, got, want)
		}
	}
}
