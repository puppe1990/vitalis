package handlers

import "fmt"

// Fmt exposes the Portuguese number formatters to templates. `view.Load` builds
// its funcMap at boot from the framework only (no app hook), so the app hands
// templates a value with methods instead of fighting the renderer — and the
// same code stays unit-tested in Go.
type Fmt struct{}

// Number renders calories and counts with Brazilian thousands separators.
//
// It takes `any` on purpose: a template argument's type is whatever the view
// data holds, and an int reaching a float64-only parameter is a runtime render
// error the compiler cannot catch. Accepting both numeric kinds turns that
// whole class of 500 into a correct string.
func (Fmt) Number(v any) string {
	return formatNumber(toFloat(v))
}

// Grams renders one decimal with a comma, dropping a trailing ",0".
func (Fmt) Grams(v any) string { return formatGrams(toFloat(v)) }

// ML renders hydration, switching to litres past 1000 ml.
func (Fmt) ML(v any) string { return formatML(toFloat(v)) }

// Pct renders a percentage with no decimals, for the aria-valuenow labels.
func (Fmt) Pct(v any) string { return formatNumber(toFloat(v)) }

// Sequence is the 0..n-1 range templates need for the hydration glass icons;
// `seq` is not a framework func and Go templates cannot count without one.
func (Fmt) Sequence(n int) []int {
	if n <= 0 {
		return nil
	}
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// RingOffset converts a fill percentage into a stroke-dashoffset for the 301.6
// circumference of the calorie ring (r=48, as the Stitch source draws it).
func (Fmt) RingOffset(percent any) string {
	const circumference = 301.6
	offset := circumference * (1 - toFloat(percent)/100)
	if offset < 0 {
		offset = 0
	}
	return trimFloat(offset)
}

// toFloat converts the numeric kinds a template can hand over. An unknown type
// is a programming error, so it panics with the type name instead of rendering
// a silent zero that would look like real data.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case uint:
		return float64(n)
	case uint64:
		return float64(n)
	default:
		panic(fmt.Sprintf("handlers.Fmt: %T is not numeric", v))
	}
}
