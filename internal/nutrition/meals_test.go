package nutrition

import "testing"

func TestMealOrder_isStableAndComplete(t *testing.T) {
	want := []Meal{MealBreakfast, MealLunch, MealSnack, MealDinner}
	got := MealOrder()
	if len(got) != len(want) {
		t.Fatalf("MealOrder() has %d meals, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("MealOrder()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMeal_validRejectsUnknown(t *testing.T) {
	if !MealLunch.Valid() {
		t.Error("MealLunch should be valid")
	}
	if Meal("brunch").Valid() {
		t.Error("unknown meal must be invalid")
	}
}

func TestMeal_normalizeFallsBackToSnack(t *testing.T) {
	if got := NormalizeMeal("brunch"); got != MealSnack {
		t.Errorf("NormalizeMeal(brunch) = %q, want %q", got, MealSnack)
	}
	if got := NormalizeMeal("Almoço"); got != MealLunch {
		t.Errorf("NormalizeMeal(Almoço) = %q, want %q", got, MealLunch)
	}
}

func TestParseGrams_rejectsGarbage(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"150", 150, true},
		{"150,5", 150.5, true},
		{" 80 g ", 80, true},
		{"", 0, false},
		{"abc", 0, false},
		{"-10", 0, false},
		{"0", 0, false},
		{"6000", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := ParseGrams(tc.in)
			if ok != tc.ok {
				t.Fatalf("ParseGrams(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("ParseGrams(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestServingPresets_arePositive(t *testing.T) {
	presets := ServingPresets()
	if len(presets) == 0 {
		t.Fatal("ServingPresets() is empty")
	}
	for _, p := range presets {
		if p.Grams <= 0 {
			t.Errorf("preset %q has grams %v", p.Label, p.Grams)
		}
	}
}
