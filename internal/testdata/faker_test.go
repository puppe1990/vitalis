package testdata

import (
	"testing"

	"github.com/puppe1990/vitalis/internal/nutrition"
)

func TestNew_isDeterministic(t *testing.T) {
	a, b := New(), New()
	if got, want := FoodName(a), FoodName(b); got != want {
		t.Errorf("two seeded fakers diverged: %q vs %q", got, want)
	}
}

func TestMacroProfile_kcalReconcilesWithMacros(t *testing.T) {
	f := New()
	for i := 0; i < 20; i++ {
		n := MacroProfile(f)
		want := n.Protein*nutrition.KcalPerProteinGram +
			n.Carb*nutrition.KcalPerCarbGram +
			n.Fat*nutrition.KcalPerFatGram
		if diff := n.Kcal - want; diff > 0.3 || diff < -0.3 {
			t.Fatalf("fixture kcal %v does not match macros (want %v)", n.Kcal, want)
		}
	}
}

func TestMealFor_alwaysReturnsAValidSlot(t *testing.T) {
	f := New()
	for i := 0; i < 50; i++ {
		if m := MealFor(f); !m.Valid() {
			t.Fatalf("MealFor returned invalid slot %q", m)
		}
	}
}

func TestWeek_hasSevenRowsAroundTarget(t *testing.T) {
	f := New()
	days := Week(f, Day(2026, 11, 16), 1950)
	if len(days) != 7 {
		t.Fatalf("Week len = %d, want 7", len(days))
	}
	for _, d := range days {
		if d.Kcal <= 0 || d.Target != 1950 {
			t.Errorf("bad row %+v", d)
		}
	}
}

func TestPortionGrams_isWithinParseBounds(t *testing.T) {
	f := New()
	for i := 0; i < 50; i++ {
		g := PortionGrams(f)
		if g < 1 || g > 5000 {
			t.Fatalf("PortionGrams = %v, outside parser bounds", g)
		}
	}
}
