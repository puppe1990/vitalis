package handlers

// Fmt exposes the Portuguese number formatters to templates. `view.Load` builds
// its funcMap at boot from the framework only (no app hook), so the app hands
// templates a value with methods instead of fighting the renderer — and the
// same code stays unit-tested in Go.
type Fmt struct{}

// Number renders calories and counts with Brazilian thousands separators.
func (Fmt) Number(v float64) string { return formatNumber(v) }

// Grams renders one decimal with a comma, dropping a trailing ",0".
func (Fmt) Grams(v float64) string { return formatGrams(v) }

// ML renders hydration, switching to litres past 1000 ml.
func (Fmt) ML(v float64) string { return formatML(v) }

// Pct renders a percentage with no decimals, for the aria-valuenow labels.
func (Fmt) Pct(v float64) string { return formatNumber(v) }

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
func (Fmt) RingOffset(percent float64) string {
	const circumference = 301.6
	offset := circumference * (1 - percent/100)
	if offset < 0 {
		offset = 0
	}
	return trimFloat(offset)
}
