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
	"github.com/puppe1990/vitalis/internal/store"
)

// foodSearchLimit caps the browse list. Without a cap the page would try to
// render the whole catalog after a one-letter search.
const foodSearchLimit = 40

// FoodsHandler renders the browse page and the create-manually form.
type FoodsHandler struct {
	views   *view.Renderer
	store   store.Store
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
}

func NewFoodsHandler(views *view.Renderer, s store.Store, site meta.Site, catalog *i18n.Catalog, cfg cais.Config) *FoodsHandler {
	return &FoodsHandler{views: views, store: s, site: site, catalog: catalog, cfg: cfg}
}

func (h *FoodsHandler) t(r *http.Request, key string, args ...any) string {
	return translate(r, h.catalog, key, args...)
}

// Browse is the searchable catalog. The destination meal rides along in the
// query so tapping a food keeps the slot the user started from.
func (h *FoodsHandler) Browse(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	search := query.Get("q")
	meal := nutrition.NormalizeMeal(query.Get("meal"))

	foods, err := h.store.ListFoods(search, query.Get("category"), foodSearchLimit)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	categories, err := h.store.ListCategories()
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	day := resolveDay(r)
	goal, err := h.store.GetGoal(currentUserID(r))
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	writeView(w, r, h.views, h.cfg, "app", "alimentos", amarraData(r, h.site, map[string]any{
		"Title":        h.t(r, "alimentos.title"),
		"ActiveNav":    "alimentos",
		"Foods":        foods,
		"Categories":   categories,
		"Search":       search,
		"Category":     query.Get("category"),
		"Meal":         meal,
		"MealLabel":    h.t(r, "meal."+string(meal)),
		"GoalLabel":    formatNumber(goal.TargetKcal),
		"DayISO":       day.Format("2006-01-02"),
		"Empty":        len(foods) == 0,
		"NoCategories": len(categories) == 0,
	}), 0)
}

// Show renders the detail page with the portion adjuster.
func (h *FoodsHandler) Show(w http.ResponseWriter, r *http.Request) {
	id, ok := parseInt64(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	food, err := h.store.FindFood(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	query := r.URL.Query()
	meal := nutrition.NormalizeMeal(query.Get("meal"))
	grams := food.PortionGrams
	if grams <= 0 {
		grams = 100
	}
	// ?g= lets the adjuster reload the page with a server-rendered portion
	// instead of duplicating the scaling math in JavaScript.
	if raw := query.Get("g"); raw != "" {
		if parsed, ok := nutrition.ParseGrams(raw); ok {
			grams = parsed
		}
	}

	portion := nutrition.Scale(food.Per100g, grams)
	day := resolveDay(r)

	writeView(w, r, h.views, h.cfg, "app", "alimento", amarraData(r, h.site, map[string]any{
		"Title":       food.Name,
		"ActiveNav":   "alimentos",
		"Food":        food,
		"Grams":       grams,
		"PrevGrams":   clampGrams(grams - portionStepGrams),
		"NextGrams":   clampGrams(grams + portionStepGrams),
		"GramsLabel":  formatGrams(grams),
		"Portion":     portion,
		"KcalLabel":   formatNumber(portion.Kcal),
		"Share":       nutrition.MacroShare(portion),
		"Meal":        meal,
		"MealLabel":   h.t(r, "meal."+string(meal)),
		"Meals":       mealOptions(h, r),
		"Presets":     nutrition.ServingPresets(),
		"DayISO":      day.Format("2006-01-02"),
		"AddURL":      "/diario/entradas?" + dayQuery(day),
		"DetailURL":   "/alimento/" + itoa(food.ID),
		"KcalPerGram": kcalPerGram(food),
	}), 0)
}

// New renders the manual food form.
func (h *FoodsHandler) New(w http.ResponseWriter, r *http.Request) {
	writeView(w, r, h.views, h.cfg, "app", "alimento_novo", amarraData(r, h.site, map[string]any{
		"Title":     h.t(r, "alimentos.manual"),
		"ActiveNav": "alimentos",
		"Meal":      nutrition.NormalizeMeal(r.URL.Query().Get("meal")),
		// A typed empty form: `index .Errors "name"` panics on an untyped nil,
		// and the template always reads `.Errors`.
		"Form":   models.Food{},
		"Errors": map[string]string{},
	}), 0)
}

// Create stores a manual food and sends the user to its detail page so they
// can pick a portion before logging it.
func (h *FoodsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulário inválido", http.StatusBadRequest)
		return
	}

	form := models.Food{
		Name:     r.PostFormValue("name"),
		Brand:    r.PostFormValue("brand"),
		Category: r.PostFormValue("category"),
		Per100g: nutrition.Nutrients{
			Kcal:    parseFloatDefault(r.PostFormValue("kcal_100g"), 0),
			Protein: parseFloatDefault(r.PostFormValue("protein_100g"), 0),
			Carb:    parseFloatDefault(r.PostFormValue("carb_100g"), 0),
			Fat:     parseFloatDefault(r.PostFormValue("fat_100g"), 0),
		},
		PortionLabel: r.PostFormValue("portion_label"),
		PortionGrams: parseFloatDefault(r.PostFormValue("portion_grams"), 100),
	}

	if form.Name == "" || form.Per100g.Kcal <= 0 {
		writeView(w, r, h.views, h.cfg, "app", "alimento_novo", amarraData(r, h.site, map[string]any{
			"Title":     h.t(r, "alimentos.manual"),
			"ActiveNav": "alimentos",
			"Form":      form,
			"Errors": map[string]string{
				"name":      h.t(r, "alimentos.err_name"),
				"kcal_100g": h.t(r, "alimentos.err_kcal"),
			},
		}), http.StatusUnprocessableEntity)
		return
	}

	id, err := h.store.CreateFood(form)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	flash.Set(w, "notice", h.t(r, "alimentos.created", form.Name), h.cfg.CookieSecure())
	http.Redirect(w, r, "/alimento/"+itoa(id), http.StatusSeeOther)
}

func dayQuery(day time.Time) string {
	return dateQueryParam + "=" + url.QueryEscape(day.Format("2006-01-02"))
}

// portionStepGrams is the +/- increment on the detail page. 25 g maps to the
// household measures the Stitch screen offers (a slice, half a scoop).
const portionStepGrams = 25

// clampGrams keeps an adjusted portion inside the parser's accepted range so
// the plus link cannot walk past 5000 g into a URL the server rejects.
func clampGrams(v float64) float64 {
	if v < nutrition.MinPortionGrams {
		return nutrition.MinPortionGrams
	}
	if v > nutrition.MaxPortionGrams {
		return nutrition.MaxPortionGrams
	}
	return v
}

// mealOptions is the destination picker payload, one entry per diary slot.
func mealOptions(h *FoodsHandler, r *http.Request) []MealOption {
	out := make([]MealOption, 0, len(nutrition.MealOrder()))
	for _, m := range nutrition.MealOrder() {
		out = append(out, MealOption{Value: string(m), Label: h.t(r, "meal."+string(m))})
	}
	return out
}

// MealOption is one radio of the destination picker.
type MealOption struct {
	Value string
	Label string
}

// kcalPerGram is the "1.65 kcal / g" chip on the detail page.
func kcalPerGram(f models.Food) string {
	if f.Per100g.Kcal <= 0 {
		return "—"
	}
	return formatGrams(f.Per100g.Kcal/100) + " kcal / g"
}
