package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
)

// dateQueryParam is how every page carries "which day am I looking at".
// A diary that cannot be navigated backwards is a scoreboard, not a log.
const dateQueryParam = "d"

// resolveDay reads ?d=YYYY-MM-DD and falls back to today in the server's zone.
// A malformed value is ignored rather than rejected: a stale bookmark must not
// break the diary.
func resolveDay(r *http.Request) time.Time {
	raw := r.URL.Query().Get(dateQueryParam)
	if raw != "" {
		if t, err := time.Parse(time.DateOnly, raw); err == nil {
			return t.UTC()
		}
	}
	return todayUTC()
}

// dayPath builds the URL for a given day on a page, preserving the page path.
func dayPath(base string, day time.Time) string {
	return fmt.Sprintf("%s?%s=%s", base, dateQueryParam, day.Format(time.DateOnly))
}

// currentUserID is the auth boundary for every handler. The routes already sit
// behind RequireAuthFunc, so a missing id here is a programming error, not a
// user-facing case.
func currentUserID(r *http.Request) int64 {
	id, _ := session.UserID(r)
	return id
}

// formatNumber renders calories and grams the Brazilian way (1.890 / 68,4).
// The templates cannot call a locale-aware formatter, and shipping "1890" next
// to "68.4" in a Portuguese UI looks like a bug.
func formatNumber(v float64) string {
	rounded := int64(math.Round(v))
	sign := ""
	if rounded < 0 {
		sign = "-"
		rounded = -rounded
	}

	digits := strconv.FormatInt(rounded, 10)
	var grouped []byte
	for i, ch := range digits {
		// A separator precedes every third digit counted from the right.
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped = append(grouped, '.')
		}
		grouped = append(grouped, byte(ch))
	}
	return sign + string(grouped)
}

// formatGrams renders one decimal with a comma, dropping a trailing ",0".
func formatGrams(v float64) string {
	r := math.Round(v*10) / 10
	if r == math.Trunc(r) {
		return formatNumber(r)
	}
	return fmt.Sprintf("%s,%d", formatNumber(math.Trunc(r)), int(math.Abs(math.Round((r-math.Trunc(r))*10))))
}

// formatML renders hydration in litres when it crosses 1 L, which is how the
// Stitch card writes "1.8L".
func formatML(v float64) string {
	if v < 1000 {
		return fmt.Sprintf("%s ml", formatNumber(v))
	}
	litres := math.Round(v/100) / 10
	return formatGrams(litres) + "L"
}

// MealView is one meal card on the diary.
type MealView struct {
	Meal     nutrition.Meal
	Label    string
	Icon     string
	Entries  []models.Entry
	Kcal     float64
	HasItems bool
	AddURL   string
}

// MacroView is one linear macro bar.
type MacroView struct {
	Key     string
	Label   string
	Color   string
	Current float64
	Target  float64
	Percent float64
	LeftPct float64
	Unit    string
}

// DayView is everything the diary page renders for one day.
type DayView struct {
	Day         time.Time
	Total       nutrition.Nutrients
	Target      nutrition.Nutrients
	Balance     nutrition.BalanceSummary
	Macros      []MacroView
	Water       nutrition.Water
	Meals       []MealView
	MealCount   int
	WaterSlots  int
	PreviousDay time.Time
	NextDay     time.Time
}

// buildDayView assembles the diary payload. It lives in the handlers package
// (not the templates) so the same numbers can be asserted in Go tests.
func buildDayView(goal models.Goal, entries []models.Entry, dayLog models.DayLog, day time.Time) DayView {
	total := nutrition.Sum(nutrientsOf(entries)...)
	target := nutrition.Nutrients{
		Kcal:    goal.TargetKcal,
		Protein: goal.ProteinG,
		Carb:    goal.CarbG,
		Fat:     goal.FatG,
	}

	byMeal := map[nutrition.Meal][]models.Entry{}
	for _, e := range entries {
		byMeal[e.Meal] = append(byMeal[e.Meal], e)
	}

	meals := make([]MealView, 0, len(nutrition.MealOrder()))
	for _, m := range nutrition.MealOrder() {
		items := byMeal[m]
		meals = append(meals, MealView{
			Meal:     m,
			Icon:     mealIcon(m),
			Entries:  items,
			Kcal:     nutrition.Sum(nutrientsOf(items)...).Kcal,
			HasItems: len(items) > 0,
		})
	}

	share := nutrition.MacroShare(total)
	return DayView{
		Day:         day,
		Total:       total,
		Target:      target,
		Balance:     nutrition.Balance(nutrition.BalanceInput{TargetKcal: target.Kcal, ConsumedKcal: total.Kcal, BurnedKcal: dayLog.BurnedKcal}),
		Macros:      macroViews(total, target, share),
		Water:       nutrition.Water{TargetML: goal.WaterML, DrunkML: dayLog.WaterML},
		WaterSlots:  waterSlots(goal.WaterML),
		Meals:       meals,
		MealCount:   len(entries),
		PreviousDay: day.AddDate(0, 0, -1),
		NextDay:     day.AddDate(0, 0, 1),
	}
}

func macroViews(total, target nutrition.Nutrients, share nutrition.MacroPercent) []MacroView {
	return []MacroView{
		{
			Key: "carb", Label: "macro.carb", Color: "carb",
			Current: total.Carb, Target: target.Carb,
			Percent: nutrition.ProgressRatio(total.Carb, target.Carb),
			LeftPct: 100 - nutrition.ProgressRatio(total.Carb, target.Carb),
			Unit:    "g",
		},
		{
			Key: "protein", Label: "macro.protein", Color: "protein",
			Current: total.Protein, Target: target.Protein,
			Percent: nutrition.ProgressRatio(total.Protein, target.Protein),
			LeftPct: 100 - nutrition.ProgressRatio(total.Protein, target.Protein),
			Unit:    "g",
		},
		{
			Key: "fat", Label: "macro.fat", Color: "fat",
			Current: total.Fat, Target: target.Fat,
			Percent: nutrition.ProgressRatio(total.Fat, target.Fat),
			LeftPct: 100 - nutrition.ProgressRatio(total.Fat, target.Fat),
			Unit:    "g",
		},
	}
}

func nutrientsOf(entries []models.Entry) []nutrition.Nutrients {
	out := make([]nutrition.Nutrients, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Nutrients)
	}
	return out
}

// waterSlots is the number of glass icons the card draws. It is computed here
// rather than in the template because the template would emit an invalid
// `repeat(0, …)` for a goal with no hydration target.
func waterSlots(targetML float64) int {
	slots := nutrition.Water{TargetML: targetML}.GlassesTarget()
	if slots > maxWaterSlots {
		return maxWaterSlots
	}
	if slots < 1 {
		return 1
	}
	return slots
}

// maxWaterSlots keeps a 4 L goal from turning the card into a wall of cups.
const maxWaterSlots = 10

// statusKey maps the shared display status to the i18n key of the badge.
func statusKey(status string, entryCount int, day time.Time, now time.Time) string {
	return "diario.status." + nutrition.DisplayStatus(status, entryCount, day, now)
}

// mealIcon maps a slot to the icon key the template renders.
func mealIcon(m nutrition.Meal) string {
	switch m {
	case nutrition.MealBreakfast:
		return "sunrise"
	case nutrition.MealLunch:
		return "sun"
	case nutrition.MealSnack:
		return "coffee"
	default:
		return "moon"
	}
}

// t is the per-request translator. Holding it on a small embedded helper keeps
// every handler from repeating the CatalogOr fallback dance (#211).
func translate(r *http.Request, fallback *i18n.Catalog, key string, args ...any) string {
	return i18n.CatalogOr(r, fallback).T(key, args...)
}
