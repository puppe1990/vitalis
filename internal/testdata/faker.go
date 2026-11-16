// Package testdata builds fake domain values for tests and dev seeds.
//
// Every value derives from one seeded gofakeit Faker, so a failing test
// reproduces with identical data on every run.
package testdata

import (
	"time"

	"github.com/brianvoe/gofakeit/v7"

	"github.com/puppe1990/vitalis/internal/nutrition"
)

// Seed pins the fixture sequence.
const Seed = uint64(20261116)

func New() *gofakeit.Faker {
	return gofakeit.New(Seed)
}

// Day is a UTC midnight, which is how the diary keys a day.
func Day(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// MacroProfile is a per-100g nutrient profile with plausible calories for its
// macros: a real food's label always reconciles, so a fixture that does not
// would let a Scale bug hide behind fake numbers.
func MacroProfile(f *gofakeit.Faker) nutrition.Nutrients {
	// Round the macros first, then derive kcal from the rounded grams — a label
	// that rounds the two independently never sums to itself.
	protein := round1(f.Float64Range(2, 30))
	carb := round1(f.Float64Range(0, 60))
	fat := round1(f.Float64Range(0.5, 25))
	kcal := protein*nutrition.KcalPerProteinGram +
		carb*nutrition.KcalPerCarbGram +
		fat*nutrition.KcalPerFatGram
	return nutrition.Nutrients{
		Kcal:    round1(kcal),
		Protein: protein,
		Carb:    carb,
		Fat:     fat,
	}
}

// FoodName returns a Portuguese food noun so the seeded UI reads like a real
// diary instead of "Food 1".
func FoodName(f *gofakeit.Faker) string {
	return f.RandomString([]string{
		"Peito de frango grelhado",
		"Arroz branco cozido",
		"Feijão preto cozido",
		"Ovo de galinha cozido",
		"Banana prata",
		"Aveia em flocos finos",
		"Iogurte natural desnatado",
		"Batata-doce assada",
		"Brócolis cozido no vapor",
		"Salmão grelhado",
		"Azeite de oliva extravirgem",
		"Queijo minas frescal",
		"Pão integral",
		"Leite desnatado",
		"Whey protein concentrado",
	})
}

// Category is the food's shelf in the browse page.
func Category(f *gofakeit.Faker) string {
	return f.RandomString([]string{
		"Proteínas", "Cereais", "Frutas", "Laticínios", "Vegetais", "Gorduras",
	})
}

// PortionGrams returns a portion a person actually serves.
func PortionGrams(f *gofakeit.Faker) float64 {
	return float64(f.IntRange(1, 12)) * 10
}

// MealFor picks a slot weighted toward lunch and dinner, where most entries land.
func MealFor(f *gofakeit.Faker) nutrition.Meal {
	roll := f.IntRange(1, 100)
	switch {
	case roll <= 20:
		return nutrition.MealBreakfast
	case roll <= 55:
		return nutrition.MealLunch
	case roll <= 75:
		return nutrition.MealSnack
	default:
		return nutrition.MealDinner
	}
}

// Week returns seven DayTotal rows ending on end, oldest first, with kcal
// scattered around the target so best/worst/adherence all have real data.
func Week(f *gofakeit.Faker, end time.Time, target float64) []nutrition.DayTotal {
	labels := []string{"Seg", "Ter", "Qua", "Qui", "Sex", "Sáb", "Dom"}
	out := make([]nutrition.DayTotal, 7)
	for i := range out {
		out[i] = nutrition.DayTotal{
			Label:  labels[i],
			Kcal:   round1(target * f.Float64Range(0.82, 1.12)),
			Target: target,
		}
	}
	_ = end
	return out
}

// WeightKG returns a body weight around a believable adult figure.
func WeightKG(f *gofakeit.Faker) float64 {
	return round1(f.Float64Range(55, 95))
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
