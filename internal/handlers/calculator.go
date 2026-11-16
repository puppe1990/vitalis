package handlers

import (
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/store"
)

// CalculatorHandler is the public form: body + secar/manter/aumentar in, a
// daily calorie goal out. Guests only see the number; a signed-in submit
// writes the same budget /metas would have derived.
type CalculatorHandler struct {
	views   *view.Renderer
	store   store.Store
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
}

func NewCalculatorHandler(views *view.Renderer, s store.Store, site meta.Site, catalog *i18n.Catalog, cfg cais.Config) *CalculatorHandler {
	return &CalculatorHandler{views: views, store: s, site: site, catalog: catalog, cfg: cfg}
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

	goals := nutrition.GoalsFor(profile)
	if userID, ok := session.UserID(r); ok {
		if err := h.saveGoal(r, userID, profile, goals); err != nil {
			httpxServerError(w, r, err, h.cfg)
			return
		}
		flash.Set(w, "notice", h.t(r, "goals.saved"), h.cfg.CookieSecure())
	}
	h.render(w, r, profile, nil, true, 0)
}

func (h *CalculatorHandler) saveGoal(r *http.Request, userID int64, profile nutrition.Profile, goals nutrition.Goals) error {
	goal := models.Goal{
		UserID:     userID,
		TargetKcal: goals.TargetKcal,
		ProteinG:   goals.ProteinG,
		CarbG:      goals.CarbG,
		FatG:       goals.FatG,
		WaterML:    goals.WaterML,
		Sex:        profile.Sex,
		AgeYears:   profile.AgeYears,
		HeightCM:   profile.HeightCM,
		WeightKG:   profile.WeightKG,
		Activity:   profile.Activity,
		Direction:  profile.Goal,
	}
	if err := h.store.UpsertGoal(goal); err != nil {
		return err
	}
	return h.store.SaveWeight(userID, resolveDay(r), profile.WeightKG)
}

func (h *CalculatorHandler) render(w http.ResponseWriter, r *http.Request, profile nutrition.Profile, errs map[string]string, hasResult bool, status int) {
	data := map[string]any{
		"Title":      h.t(r, "calc.title"),
		"ActiveNav":  "calculadora",
		"Profile":    profile,
		"Errors":     errs,
		"HasResult":  hasResult,
		"Directions": calculatorDirections(),
	}
	if hasResult {
		goals := nutrition.GoalsFor(profile)
		data["TargetLabel"] = formatNumber(goals.TargetKcal)
		data["BMRLabel"] = formatNumber(goals.BMR)
		data["WaterLabel"] = formatML(goals.WaterML)
		data["ProteinLabel"] = formatGrams(goals.ProteinG)
		data["CarbLabel"] = formatGrams(goals.CarbG)
		data["FatLabel"] = formatGrams(goals.FatG)
		data["GoalLabelKey"] = calculatorGoalLabel(profile.Goal)
	}
	writeView(w, r, h.views, h.cfg, "app", "calculadora", amarraData(r, h.site, data), status)
}

// calculatorProfile reads the public fields and pins moderate activity so
// GoalsFor can run. Direction comes from the form.
func calculatorProfile(r *http.Request) nutrition.Profile {
	return nutrition.Profile{
		Sex:      nutrition.Sex(r.PostFormValue("sex")),
		AgeYears: atoiDefault(r.PostFormValue("age_years"), 0),
		HeightCM: parseFloatDefault(r.PostFormValue("height_cm"), 0),
		WeightKG: parseFloatDefault(r.PostFormValue("weight_kg"), 0),
		Activity: nutrition.ActivityModerate,
		Goal:     nutrition.Goal(r.PostFormValue("direction")),
	}
}

func calculatorDirections() []Option {
	return []Option{
		{Value: string(nutrition.GoalLose), LabelKey: "calc.lose"},
		{Value: string(nutrition.GoalMaintain), LabelKey: "calc.maintain"},
		{Value: string(nutrition.GoalGain), LabelKey: "calc.gain"},
	}
}

func calculatorGoalLabel(g nutrition.Goal) string {
	switch g {
	case nutrition.GoalLose:
		return "calc.goal_lose"
	case nutrition.GoalGain:
		return "calc.goal_gain"
	default:
		return "calc.goal_maintain"
	}
}
