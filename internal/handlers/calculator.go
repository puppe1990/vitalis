package handlers

import (
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/vitalis/internal/nutrition"
)

// CalculatorHandler is the public Mifflin-St Jeor form: weight, height, age and
// sex in, daily calories out. It never writes a goal — /metas is what saves.
type CalculatorHandler struct {
	views   *view.Renderer
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
}

func NewCalculatorHandler(views *view.Renderer, site meta.Site, catalog *i18n.Catalog, cfg cais.Config) *CalculatorHandler {
	return &CalculatorHandler{views: views, site: site, catalog: catalog, cfg: cfg}
}

func (h *CalculatorHandler) t(r *http.Request, key string, args ...any) string {
	return translate(r, h.catalog, key, args...)
}

func (h *CalculatorHandler) Show(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, nutrition.Profile{}, nil, false, 0)
}

func (h *CalculatorHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulário inválido", http.StatusBadRequest)
		return
	}

	profile := calculatorProfile(r)
	errs := validateProfile(profile, 0)
	if len(errs) > 0 {
		h.render(w, r, profile, errs, false, http.StatusUnprocessableEntity)
		return
	}
	h.render(w, r, profile, nil, true, 0)
}

func (h *CalculatorHandler) render(w http.ResponseWriter, r *http.Request, profile nutrition.Profile, errs map[string]string, hasResult bool, status int) {
	data := map[string]any{
		"Title":     h.t(r, "calc.title"),
		"ActiveNav": "calculadora",
		"Profile":   profile,
		"Errors":    errs,
		"HasResult": hasResult,
	}
	if hasResult {
		goals := nutrition.GoalsFor(profile)
		data["TargetLabel"] = formatNumber(goals.TargetKcal)
		data["BMRLabel"] = formatNumber(goals.BMR)
		data["WaterLabel"] = formatML(goals.WaterML)
		data["ProteinLabel"] = formatGrams(goals.ProteinG)
		data["CarbLabel"] = formatGrams(goals.CarbG)
		data["FatLabel"] = formatGrams(goals.FatG)
		data["Bands"] = calculatorBands(profile)
	}
	writeView(w, r, h.views, h.cfg, "app", "calculadora", amarraData(r, h.site, data), status)
}

// calculatorProfile fills the four public fields and pins moderate / maintain
// so GoalsFor returns TDEE. Activity and direction stay off the form.
func calculatorProfile(r *http.Request) nutrition.Profile {
	return nutrition.Profile{
		Sex:      nutrition.Sex(r.PostFormValue("sex")),
		AgeYears: atoiDefault(r.PostFormValue("age_years"), 0),
		HeightCM: parseFloatDefault(r.PostFormValue("height_cm"), 0),
		WeightKG: parseFloatDefault(r.PostFormValue("weight_kg"), 0),
		Activity: nutrition.ActivityModerate,
		Goal:     nutrition.GoalMaintain,
	}
}

type calcBand struct {
	Value     string
	LabelKey  string
	KcalLabel string
	Highlight bool
}

func calculatorBands(p nutrition.Profile) []calcBand {
	out := make([]calcBand, 0, 4)
	for _, opt := range activityOptions() {
		q := p
		q.Activity = nutrition.Activity(opt.Value)
		g := nutrition.GoalsFor(q)
		out = append(out, calcBand{
			Value:     opt.Value,
			LabelKey:  opt.LabelKey,
			KcalLabel: formatNumber(g.TargetKcal),
			Highlight: opt.Value == string(nutrition.ActivityModerate),
		})
	}
	return out
}
