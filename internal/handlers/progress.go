package handlers

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/store"
)

// progressDays is the window the progression page summarises. One week matches
// the Stitch tabs' default and keeps the query bounded.
const progressDays = 7

// ProgressHandler renders the weekly statistics page.
type ProgressHandler struct {
	views   *view.Renderer
	store   store.Store
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
}

func NewProgressHandler(views *view.Renderer, s store.Store, site meta.Site, catalog *i18n.Catalog, cfg cais.Config) *ProgressHandler {
	return &ProgressHandler{views: views, store: s, site: site, catalog: catalog, cfg: cfg}
}

func (h *ProgressHandler) t(r *http.Request, key string, args ...any) string {
	return translate(r, h.catalog, key, args...)
}

func (h *ProgressHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	goal, err := h.store.GetGoal(userID)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	end := resolveDay(r)
	days, err := h.weekTotals(userID, goal.TargetKcal, end)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	summary := nutrition.Weekly(days)

	weights, err := h.store.ListWeights(userID, 30)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	share, err := h.weekMacroShare(userID, end)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	writeView(w, r, h.views, h.cfg, "app", "progresso", amarraData(r, h.site, map[string]any{
		"Title":          h.t(r, "progresso.title"),
		"ActiveNav":      "progresso",
		"Summary":        summary,
		"Bars":           barViews(summary.Days, goal.TargetKcal),
		"MacroShare":     share,
		"Weights":        weights,
		"WeightPoints":   weightPolyline(weights),
		"LatestWeight":   latestWeightLabel(weights),
		"WeekLabel":      weekRangeLabel(end),
		"TargetLinePct":  targetLinePct(summary.Days, goal.TargetKcal),
		"AverageLabel":   formatNumber(summary.AverageKcal),
		"TargetLabel":    formatNumber(goal.TargetKcal),
		"DeficitLabel":   signedNumber(summary.BalanceKcal),
		"AdherenceLabel": formatNumber(summary.AdherencePercent) + "%",
		"LowestLabel":    formatNumber(summary.WorstDay.Kcal) + " kcal",
		"WeightDays":     len(weights),
		"HasWeight":      len(weights) > 0,
	}), 0)
}

// weekTotals collects one row per day. Every day appears even with no entries:
// a gap in the chart would read as missing data rather than a zero-kcal day.
func (h *ProgressHandler) weekTotals(userID int64, target float64, end time.Time) ([]nutrition.DayTotal, error) {
	labels := []string{"Seg", "Ter", "Qua", "Qui", "Sex", "Sáb", "Dom"}
	days := make([]nutrition.DayTotal, 0, progressDays)

	start := end.AddDate(0, 0, -(progressDays - 1))
	for i := 0; i < progressDays; i++ {
		day := start.AddDate(0, 0, i)
		entries, err := h.store.ListEntries(userID, day)
		if err != nil {
			return nil, err
		}
		days = append(days, nutrition.DayTotal{
			Label:  labels[int(day.Weekday()+6)%7],
			Kcal:   nutrition.Sum(nutrientsOf(entries)...).Kcal,
			Target: target,
		})
	}
	return days, nil
}

func (h *ProgressHandler) weekMacroShare(userID int64, end time.Time) (nutrition.MacroPercent, error) {
	start := end.AddDate(0, 0, -(progressDays - 1))
	var all []models.Entry
	for i := 0; i < progressDays; i++ {
		entries, err := h.store.ListEntries(userID, start.AddDate(0, 0, i))
		if err != nil {
			return nutrition.MacroPercent{}, err
		}
		all = append(all, entries...)
	}
	return nutrition.MacroShare(nutrition.Sum(nutrientsOf(all)...)), nil
}

// targetLinePct places the dashed goal line in the same 0-100 space the bars
// use, so the line and the columns share one scale.
func targetLinePct(days []nutrition.DayTotal, target float64) float64 {
	scale := target
	for _, d := range days {
		if d.Kcal > scale {
			scale = d.Kcal
		}
	}
	if scale <= 0 {
		return 0
	}
	pct := target / scale * 100
	if pct > 100 {
		return 100
	}
	return pct
}

// BarView is one column of the intake chart. HeightPct is relative to the
// tallest bar (not the target) so a week of light days is still readable.
type BarView struct {
	Label     string
	HeightPct float64
	Kcal      float64
	KcalLabel string
	Over      bool
}

func barViews(days []nutrition.DayTotal, target float64) []BarView {
	// The chart caps the scale at the larger of the target and the peak so the
	// goal line stays inside the plot even on a week of overshoots.
	scale := target
	for _, d := range days {
		if d.Kcal > scale {
			scale = d.Kcal
		}
	}
	if scale <= 0 {
		scale = 1
	}

	out := make([]BarView, 0, len(days))
	for _, d := range days {
		height := d.Kcal / scale * 100
		// A minimum stub keeps every day visible, including the ones with
		// nothing logged: a hole in the chart reads as missing data, not zero.
		if height < 4 {
			height = 4
		}
		out = append(out, BarView{
			Label:     d.Label,
			HeightPct: height,
			Kcal:      d.Kcal,
			KcalLabel: formatNumber(d.Kcal),
			Over:      d.Overshoot(),
		})
	}
	return out
}

// weightPolyline maps weigh-ins to SVG points inside a 0-100 viewBox, newest
// last. The chart is deliberately not zero-based: body weight moves by grams,
// and a zero-based axis would render a flat line.
func weightPolyline(weights []models.Weight) string {
	if len(weights) < 2 {
		return ""
	}

	minKG, maxKG := weights[0].KG, weights[0].KG
	for _, w := range weights {
		if w.KG < minKG {
			minKG = w.KG
		}
		if w.KG > maxKG {
			maxKG = w.KG
		}
	}
	span := maxKG - minKG
	if span == 0 {
		span = 1
	}

	points := make([]byte, 0, len(weights)*12)
	for i := len(weights) - 1; i >= 0; i-- {
		x := float64(len(weights)-1-i) / float64(len(weights)-1) * 100
		// 10-90 leaves room for the stroke so the line is not clipped.
		y := 90 - (weights[i].KG-minKG)/span*80
		if len(points) > 0 {
			points = append(points, ' ')
		}
		points = append(points, []byte(formatPoint(x, y))...)
	}
	return string(points)
}

func formatPoint(x, y float64) string {
	return trimFloat(x) + "," + trimFloat(y)
}

// trimFloat keeps SVG coordinates short. It must use a dot: an SVG point with
// a decimal comma is invalid markup, unlike the Portuguese copy elsewhere.
func trimFloat(v float64) string {
	rounded := math.Round(v*10) / 10
	if rounded == math.Trunc(rounded) {
		return itoa(int64(rounded))
	}
	return strconv.FormatFloat(rounded, 'f', 1, 64)
}

func latestWeightLabel(weights []models.Weight) string {
	if len(weights) == 0 {
		return "—"
	}
	return formatGrams(weights[0].KG) + " kg"
}

// weekRangeLabel renders "10 nov — 16 nov" for the report header.
func weekRangeLabel(end time.Time) string {
	start := end.AddDate(0, 0, -(progressDays - 1))
	return shortDatePT(start.Day(), int(start.Month())) + " — " + shortDatePT(end.Day(), int(end.Month()))
}

// signedNumber always shows the sign, matching the Stitch "−2.450" tile.
func signedNumber(v float64) string {
	if v > 0 {
		return "+" + formatNumber(v)
	}
	return formatNumber(v)
}
