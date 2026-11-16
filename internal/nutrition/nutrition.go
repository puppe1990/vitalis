// Package nutrition holds Vitalis's calorie/macro arithmetic.
//
// It is pure: no database, no HTTP. Handlers and the JSON API both call it so
// the numbers on the dashboard and the numbers an MCP client reads cannot drift.
package nutrition

// KcalPerGram is the Atwater factor set. Labels in Brazil (and every
// macro-tracking app) round with 4/4/9, so Vitalis does too instead of using
// the more precise 4.2/3.75/9 that would make sums disagree with the packaging.
const (
	KcalPerProteinGram = 4.0
	KcalPerCarbGram    = 4.0
	KcalPerFatGram     = 9.0
)

// Nutrients is one portion's worth of energy and the three macros.
type Nutrients struct {
	Kcal    float64 `json:"kcal"`
	Protein float64 `json:"protein_g"`
	Carb    float64 `json:"carb_g"`
	Fat     float64 `json:"fat_g"`
}

// MacroPercent is the share of energy each macro contributes, summing to 100.
type MacroPercent struct {
	Protein float64 `json:"protein_pct"`
	Carb    float64 `json:"carb_pct"`
	Fat     float64 `json:"fat_pct"`
}

// Scale converts a per-100g nutrient profile into an arbitrary portion.
// A non-positive portion has no nutrients, so it returns zero instead of
// letting a stray minus sign become negative calories on the dashboard.
func Scale(per100g Nutrients, grams float64) Nutrients {
	if grams <= 0 {
		return Nutrients{}
	}
	factor := grams / 100
	return Nutrients{
		Kcal:    per100g.Kcal * factor,
		Protein: per100g.Protein * factor,
		Carb:    per100g.Carb * factor,
		Fat:     per100g.Fat * factor,
	}
}

// Sum adds every entry of a day. Variadic so callers can pass a slice with
// `Sum(entries...)` and skip building an intermediate accumulator.
func Sum(parts ...Nutrients) Nutrients {
	var total Nutrients
	for _, p := range parts {
		total.Kcal += p.Kcal
		total.Protein += p.Protein
		total.Carb += p.Carb
		total.Fat += p.Fat
	}
	return total
}

// Remaining is the signed budget left. It goes negative on purpose: the UI and
// the API must be able to say "300 kcal above target" instead of clamping to 0.
func Remaining(target, consumed float64) float64 {
	return target - consumed
}

// MacroShare normalizes each macro's energy contribution to a percentage.
// An empty day has no share: returning 0 avoids the NaN that 0/0 would print.
func MacroShare(n Nutrients) MacroPercent {
	proteinKcal := n.Protein * KcalPerProteinGram
	carbKcal := n.Carb * KcalPerCarbGram
	fatKcal := n.Fat * KcalPerFatGram
	total := proteinKcal + carbKcal + fatKcal
	if total <= 0 {
		return MacroPercent{}
	}
	return MacroPercent{
		Protein: proteinKcal / total * 100,
		Carb:    carbKcal / total * 100,
		Fat:     fatKcal / total * 100,
	}
}

// ProgressRatio is the bar/ring fill percentage, clamped at 100 so an
// overshoot does not draw a ring that wraps past its own start.
func ProgressRatio(value, target float64) float64 {
	if target <= 0 || value <= 0 {
		return 0
	}
	if value >= target {
		return 100
	}
	return value / target * 100
}
