package handlers

import (
	"net/http"
	"net/url"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/photos"
	"github.com/puppe1990/vitalis/internal/store"
)

// DiaryHandler renders the day view and owns every mutation that lands back on
// it: logging a portion, deleting an entry and adding water.
type DiaryHandler struct {
	views   *view.Renderer
	store   store.Store
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
	photos  *photos.Library
}

func NewDiaryHandler(views *view.Renderer, s store.Store, site meta.Site, catalog *i18n.Catalog, cfg cais.Config, lib *photos.Library) *DiaryHandler {
	return &DiaryHandler{views: views, store: s, site: site, catalog: catalog, cfg: cfg, photos: lib}
}

func (h *DiaryHandler) t(r *http.Request, key string, args ...any) string {
	return translate(r, h.catalog, key, args...)
}

func (h *DiaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	day := resolveDay(r)

	goal, err := h.store.GetGoal(userID)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	entries, err := h.store.ListEntries(userID, day)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	dayLog, err := h.store.GetDayLog(userID, day)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	dayView := buildDayView(goal, entries, dayLog, day)
	h.fillMealLabels(r, dayView.Meals)
	if err := h.attachPhotos(userID, day, &dayView); err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	writeView(w, r, h.views, h.cfg, "app", "diario", amarraData(r, h.site, map[string]any{
		"Title":         h.t(r, "diario.title"),
		"ActiveNav":     "diario",
		"Day":           dayView,
		"DayISO":        day.Format("2006-01-02"),
		"DayLabel":      h.dayLabel(r, day),
		"PrevURL":       dayPath("/diario", dayView.PreviousDay),
		"NextURL":       dayPath("/diario", dayView.NextDay),
		"TargetLabel":   formatNumber(goal.TargetKcal),
		"WaterLabel":    formatML(dayLog.WaterML),
		"WaterTarget":   formatML(goal.WaterML),
		"StatusKey":     statusKey(dayView.Balance.Status, dayView.MealCount, day, time.Now()),
		"AddWaterURL":   dayPath("/diario/agua", day),
		"RingOffset":    Fmt{}.RingOffset(nutrition.ProgressRatio(dayView.Total.Kcal, goal.TargetKcal)),
		"FoodsURL":      "/alimentos",
		"PhotosEnabled": h.photos.Enabled(),
	}), 0)
}

func (h *DiaryHandler) attachPhotos(userID int64, day time.Time, dayView *DayView) error {
	if !h.photos.Enabled() {
		return nil
	}
	listed, err := h.photos.List(userID, day)
	if err != nil {
		return err
	}
	byMeal := map[nutrition.Meal][]PhotoView{}
	for _, p := range listed {
		byMeal[p.Meal] = append(byMeal[p.Meal], PhotoView{
			ID:        p.ID,
			URL:       "/diario/fotos/" + itoa(p.ID),
			DeleteURL: "/diario/fotos/" + itoa(p.ID) + "/remover",
		})
	}
	for i := range dayView.Meals {
		dayView.Meals[i].Photos = byMeal[dayView.Meals[i].Meal]
	}
	return nil
}

func (h *DiaryHandler) fillMealLabels(r *http.Request, meals []MealView) {
	for i := range meals {
		meals[i].Label = h.t(r, "meal."+string(meals[i].Meal))
		meals[i].AddURL = "/alimentos?meal=" + url.QueryEscape(string(meals[i].Meal))
	}
}

// dayLabel renders "Hoje, 16 de novembro" for today and the written date
// otherwise, because "16/11/2026" repeated on every card is noise.
func (h *DiaryHandler) dayLabel(r *http.Request, day time.Time) string {
	written := longDatePT(day.Day(), int(day.Month()))
	if day.Equal(todayUTC()) {
		return h.t(r, "diario.today", written)
	}
	return written
}

// todayUTC is the diary's definition of "today". Using one function keeps the
// header, the default day and the seeded fixtures on the same calendar day.
func todayUTC() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// AddWaterPost accumulates one glass and returns to the same day.
func (h *DiaryHandler) AddWaterPost(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	day := resolveDay(r)

	if err := h.store.AddWater(userID, day, nutrition.GlassML); err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	http.Redirect(w, r, dayPath("/diario", day), http.StatusSeeOther)
}

// AddEntryPost logs a portion from the detail page. The grams field is parsed
// by the domain so a junk value renders the same error the detail page shows.
func (h *DiaryHandler) AddEntryPost(w http.ResponseWriter, r *http.Request) {
	h.addEntry(w, r, r.PostFormValue("food_id"), "")
}

// QuickAddPost is the one-tap "+" on the browse list. It reads the food id from
// the path and sends the user back to the list, so they can keep logging.
func (h *DiaryHandler) QuickAddPost(w http.ResponseWriter, r *http.Request) {
	h.addEntry(w, r, r.PathValue("id"), "/alimentos")
}

func (h *DiaryHandler) addEntry(w http.ResponseWriter, r *http.Request, rawFoodID, backToList string) {
	userID := currentUserID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulário inválido", http.StatusBadRequest)
		return
	}

	foodID, ok := parseInt64(rawFoodID)
	if !ok {
		http.Redirect(w, r, "/alimentos", http.StatusSeeOther)
		return
	}
	food, err := h.store.FindFood(foodID)
	if err != nil {
		http.Redirect(w, r, "/alimentos", http.StatusSeeOther)
		return
	}

	grams, ok := nutrition.ParseGrams(r.PostFormValue("grams"))
	if !ok {
		http.Redirect(w, r, "/alimento/"+itoa(foodID), http.StatusSeeOther)
		return
	}

	day := resolveDay(r)
	entry := models.Entry{
		UserID:    userID,
		FoodID:    food.ID,
		Meal:      nutrition.NormalizeMeal(r.PostFormValue("meal")),
		LoggedOn:  day,
		Grams:     grams,
		Nutrients: nutrition.Scale(food.Per100g, grams),
	}
	if _, err := h.store.CreateEntry(entry); err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	flash.Set(w, "notice", h.t(r, "diario.entry_added", food.Name), h.cfg.CookieSecure())
	if backToList != "" {
		http.Redirect(w, r, backToList+"?meal="+url.QueryEscape(string(entry.Meal))+"&"+dayQuery(day), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, dayPath("/diario", day), http.StatusSeeOther)
}

// DeleteEntryPost removes an entry from the day the user is looking at.
func (h *DiaryHandler) DeleteEntryPost(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	day := resolveDay(r)

	entryID, ok := parseInt64(r.PathValue("id"))
	if !ok {
		http.Redirect(w, r, dayPath("/diario", day), http.StatusSeeOther)
		return
	}
	if err := h.store.DeleteEntry(userID, entryID); err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	flash.Set(w, "notice", h.t(r, "diario.entry_deleted"), h.cfg.CookieSecure())
	http.Redirect(w, r, dayPath("/diario", day), http.StatusSeeOther)
}
