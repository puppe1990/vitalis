package nutrition

import "time"

// BalanceStatus is the encouraging, non-clinical label the dashboard badge
// shows. It derives from the deficit band, never from a diagnosis.
const (
	StatusDeficitHigh     = "deficit_high"
	StatusDeficitModerate = "deficit_moderate"
	StatusDeficitLight    = "deficit_light"
	StatusOnTarget        = "on_target"
	StatusSurplus         = "surplus"
)

// Deficit bands in kcal/day. Chosen so a 2100 kcal target reads "moderate"
// around 500-700 kcal (a safe 0.4-0.6 kg/week) and "high" only past 900.
const (
	bandDeficitHigh     = 900
	bandDeficitModerate = 400
	bandDeficitLight    = 100
	bandOnTarget        = 100
)

// BalanceInput is what a day's dashboard needs to render its hero card.
type BalanceInput struct {
	TargetKcal   float64
	ConsumedKcal float64
	BurnedKcal   float64
}

// BalanceSummary is the resolved hero card: the signed budget, the net intake
// and the human label for the deficit band.
type BalanceSummary struct {
	TargetKcal    float64 `json:"target_kcal"`
	ConsumedKcal  float64 `json:"consumed_kcal"`
	BurnedKcal    float64 `json:"burned_kcal"`
	NetKcal       float64 `json:"net_kcal"`
	RemainingKcal float64 `json:"remaining_kcal"`
	DeficitKcal   float64 `json:"deficit_kcal"`
	Status        string  `json:"status"`
}

// Balance treats active energy as food the user did not have to spend, so a
// 2000 kcal day with 340 kcal burned leaves the same room as eating 1660.
func Balance(in BalanceInput) BalanceSummary {
	net := in.ConsumedKcal - in.BurnedKcal
	remaining := in.TargetKcal - net
	return BalanceSummary{
		TargetKcal:    in.TargetKcal,
		ConsumedKcal:  in.ConsumedKcal,
		BurnedKcal:    in.BurnedKcal,
		NetKcal:       net,
		RemainingKcal: remaining,
		DeficitKcal:   in.TargetKcal - in.ConsumedKcal,
		Status:        deficitStatus(in.TargetKcal - in.ConsumedKcal),
	}
}

// deficitStatus maps the raw gap to a label. A negative gap is a surplus.
func deficitStatus(deficit float64) string {
	switch {
	case deficit >= bandDeficitHigh:
		return StatusDeficitHigh
	case deficit >= bandDeficitModerate:
		return StatusDeficitModerate
	case deficit >= bandDeficitLight:
		return StatusDeficitLight
	case deficit > -bandOnTarget:
		return StatusOnTarget
	default:
		return StatusSurplus
	}
}

// Display statuses are the display-only buckets that sit on top of the deficit
// bands: a day that has not started, and a day still in progress, are neither.
const (
	DisplayNotStarted = "not_started"
	DisplayInProgress = "in_progress"
)

// DayEndHour is when the app stops hedging and reports the real band. 21h
// leaves room for a light dinner to still close the day.
const DayEndHour = 21

// DisplayStatus is what the badge shows. A deficit is only meaningful once the
// day is over: at 10am with 200 kcal logged the app is 1400 kcal "short", but
// the honest label is "day in progress", not an alarm.
//
// It lives here rather than in a handler so the web page, /api/v1 and the MCP
// server all render the same word for the same state.
func DisplayStatus(band string, entryCount int, day, now time.Time) string {
	if entryCount == 0 {
		return DisplayNotStarted
	}
	if sameDay(day, now) && now.Hour() < DayEndHour {
		return DisplayInProgress
	}
	return band
}

// sameDay compares calendar days, ignoring the clock and the location offset.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// GlassML is the quick-add glass size. 250 ml is the Brazilian household cup
// the Stitch design draws, and keeps the 8-glasses habit legible.
const GlassML = 250

// Water is the hydration card's state.
type Water struct {
	TargetML float64 `json:"target_ml"`
	DrunkML  float64 `json:"drunk_ml"`
}

// Glasses counts full 250 ml glasses already drunk, for the filled icons.
func (w Water) Glasses() int {
	if w.DrunkML <= 0 {
		return 0
	}
	return int(w.DrunkML / GlassML)
}

// GlassesTarget is the number of icons the card draws.
func (w Water) GlassesTarget() int {
	if w.TargetML <= 0 {
		return 0
	}
	return int((w.TargetML + GlassML - 1) / GlassML)
}

// Percent is the hydration ring fill, clamped like every other bar.
func (w Water) Percent() float64 {
	return ProgressRatio(w.DrunkML, w.TargetML)
}

// DayTotal is one bar of the weekly chart.
type DayTotal struct {
	Label  string  `json:"label"`
	Kcal   float64 `json:"kcal"`
	Target float64 `json:"target_kcal"`
}

// Overshoot reports the bar that must be painted as surplus. It uses the same
// 10% tolerance as adherence so the bar colour and the adherence number agree.
func (d DayTotal) Overshoot() bool {
	if d.Target <= 0 {
		return false
	}
	return d.Kcal > d.Target*adherenceToleranceHigh
}

// adherenceTolerance bands: a day counts toward adherence when it lands inside
// ±10% of target, which is the margin nutritionists use before calling a day
// off-plan.
const (
	adherenceToleranceHigh = 1.10
	adherenceToleranceLow  = 0.90
)

// WeeklySummary is the progression page's summary of a week.
type WeeklySummary struct {
	Days             []DayTotal `json:"days"`
	AverageKcal      float64    `json:"average_kcal"`
	TargetKcal       float64    `json:"target_kcal"`
	AdherencePercent float64    `json:"adherence_pct"`
	// BalanceKcal is consumed minus target across the week, so a week under
	// target reads negative exactly like the "Déficit Total −2.450" tile.
	BalanceKcal float64  `json:"balance_kcal"`
	BestDay     DayTotal `json:"best_day"`
	WorstDay    DayTotal `json:"worst_day"`
}

// Weekly averages only the days that have a total. A day with nothing logged
// is absence of data, not a heroic zero-kcal day: counting it would report an
// untouched week as a huge deficit and drag the average to nonsense.
func Weekly(days []DayTotal) WeeklySummary {
	out := WeeklySummary{Days: days}
	if len(days) == 0 {
		return out
	}

	var sumKcal, sumTarget float64
	counted, adherent := 0, 0
	var best, worst DayTotal

	for _, d := range days {
		if d.Kcal <= 0 {
			continue
		}
		if counted == 0 || d.Kcal < best.Kcal {
			best = d
		}
		if counted == 0 || d.Kcal > worst.Kcal {
			worst = d
		}
		counted++
		sumKcal += d.Kcal
		sumTarget += d.Target
		if withinAdherence(d) {
			adherent++
		}
		out.BalanceKcal += d.Kcal - d.Target
	}

	if counted == 0 {
		return out
	}
	out.AverageKcal = sumKcal / float64(counted)
	out.TargetKcal = sumTarget / float64(counted)
	out.AdherencePercent = float64(adherent) / float64(counted) * 100
	out.BestDay = best
	out.WorstDay = worst
	return out
}

func withinAdherence(d DayTotal) bool {
	if d.Target <= 0 {
		return false
	}
	ratio := d.Kcal / d.Target
	return ratio <= adherenceToleranceHigh && ratio >= adherenceToleranceLow
}
