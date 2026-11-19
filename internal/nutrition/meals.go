package nutrition

import (
	"strconv"
	"strings"
)

// Meal is the diary slot a food entry belongs to. The values double as the
// SQL column and the JSON field so no mapping table is needed at the boundary.
type Meal string

const (
	MealBreakfast Meal = "cafe_da_manha"
	MealLunch     Meal = "almoco"
	MealSnack     Meal = "lanche"
	MealDinner    Meal = "jantar"
	MealSupper    Meal = "ceia"
)

// mealAliases lets a form, an MCP client and an accented Portuguese string all
// resolve to the same slot instead of silently dropping an entry.
var mealAliases = map[string]Meal{
	"cafe_da_manha": MealBreakfast,
	"cafedamanha":   MealBreakfast,
	"café da manhã": MealBreakfast,
	"cafe da manha": MealBreakfast,
	"breakfast":     MealBreakfast,
	"almoco":        MealLunch,
	"almoço":        MealLunch,
	"lunch":         MealLunch,
	"lanche":        MealSnack,
	"snack":         MealSnack,
	"jantar":        MealDinner,
	"dinner":        MealDinner,
	"ceia":          MealSupper,
	"supper":        MealSupper,
}

// MealOrder is the diary's render order, matching the day's chronology.
func MealOrder() []Meal {
	return []Meal{MealBreakfast, MealLunch, MealSnack, MealDinner, MealSupper}
}

// Valid reports whether m is one of the five known slots.
func (m Meal) Valid() bool {
	switch m {
	case MealBreakfast, MealLunch, MealSnack, MealDinner, MealSupper:
		return true
	}
	return false
}

// NormalizeMeal resolves free text (form value, API body, accented label) to a
// slot. Anything unrecognized becomes a snack: an entry filed in the wrong slot
// still counts toward the day, which beats rejecting the user's food.
func NormalizeMeal(raw string) Meal {
	key := strings.ToLower(strings.TrimSpace(raw))
	if m, ok := mealAliases[key]; ok {
		return m
	}
	if m := Meal(key); m.Valid() {
		return m
	}
	return MealSnack
}

// Grams bounds. A 1 g portion is a rounding artifact; past 5 kg the user most
// likely typed grams into the calorie field.
const (
	// MinPortionGrams and MaxPortionGrams bound every portion the app accepts,
	// in the browser and in the API, so both entry points share one rule.
	MinPortionGrams = 1.0
	MaxPortionGrams = 5000.0
)

// ParseGrams accepts "150", "150,5" and " 80 g " — the three shapes a Brazilian
// numeric keyboard plus a pasted label produce.
func ParseGrams(raw string) (float64, bool) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimSuffix(strings.ToLower(clean), "g")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return 0, false
	}
	clean = strings.ReplaceAll(clean, ",", ".")
	value, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, false
	}
	if value < MinPortionGrams || value > MaxPortionGrams {
		return 0, false
	}
	return value, true
}

// Serving is a one-tap portion shortcut on the food detail page.
type Serving struct {
	Label string
	Grams float64
}

// ServingPresets are the household measures the Stitch detail screen offers.
func ServingPresets() []Serving {
	return []Serving{
		{Label: "Porção padrão", Grams: 120},
		{Label: "Filé pequeno", Grams: 80},
		{Label: "Xícara picada", Grams: 140},
		{Label: "Prato cheio", Grams: 250},
	}
}
