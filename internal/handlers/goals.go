package handlers

import (
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/store"
)

// GoalsHandler owns the profile form that drives the calorie budget.
type GoalsHandler struct {
	views   *view.Renderer
	store   store.Store
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
}

func NewGoalsHandler(views *view.Renderer, s store.Store, site meta.Site, catalog *i18n.Catalog, cfg cais.Config) *GoalsHandler {
	return &GoalsHandler{views: views, store: s, site: site, catalog: catalog, cfg: cfg}
}

func (h *GoalsHandler) t(r *http.Request, key string, args ...any) string {
	return translate(r, h.catalog, key, args...)
}

func (h *GoalsHandler) Show(w http.ResponseWriter, r *http.Request) {
	goal, err := h.store.GetGoal(currentUserID(r))
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	h.render(w, r, goal, nil, 0)
}

func (h *GoalsHandler) render(w http.ResponseWriter, r *http.Request, goal models.Goal, errs map[string]string, status int) {
	writeView(w, r, h.views, h.cfg, "app", "metas", amarraData(r, h.site, map[string]any{
		"Title":        h.t(r, "goals.title"),
		"ActiveNav":    "metas",
		"Goal":         goal,
		"Errors":       errs,
		"TargetLabel":  formatNumber(goal.TargetKcal),
		"WaterLabel":   formatML(goal.WaterML),
		"ProteinLabel": formatGrams(goal.ProteinG),
		"CarbLabel":    formatGrams(goal.CarbG),
		"FatLabel":     formatGrams(goal.FatG),
		"FloorWarned":  goal.TargetKcal <= nutrition.MinTargetKcal,
		"Activities":   activityOptions(),
		"Directions":   directionOptions(),
	}), status)
}

// Save recomputes the whole budget from the profile rather than trusting a
// posted target_kcal: the derived number is the product's promise, and letting
// a form field override it would let the UI and the MCP server disagree.
func (h *GoalsHandler) Save(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulário inválido", http.StatusBadRequest)
		return
	}

	profile := nutrition.Profile{
		Sex:      nutrition.Sex(r.PostFormValue("sex")),
		AgeYears: atoiDefault(r.PostFormValue("age_years"), 0),
		HeightCM: parseFloatDefault(r.PostFormValue("height_cm"), 0),
		WeightKG: parseFloatDefault(r.PostFormValue("weight_kg"), 0),
		Activity: nutrition.Activity(r.PostFormValue("activity")),
		Goal:     nutrition.Goal(r.PostFormValue("direction")),
	}

	errs := validateProfile(profile)
	if len(errs) > 0 {
		h.render(w, r, models.Goal{
			UserID:    currentUserID(r),
			Sex:       profile.Sex,
			AgeYears:  profile.AgeYears,
			HeightCM:  profile.HeightCM,
			WeightKG:  profile.WeightKG,
			Activity:  profile.Activity,
			Direction: profile.Goal,
		}, errs, http.StatusUnprocessableEntity)
		return
	}

	goals := nutrition.GoalsFor(profile)
	userID := currentUserID(r)
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
		httpxServerError(w, r, err, h.cfg)
		return
	}

	// A weigh-in on the goals form is also the progression chart's data point,
	// so one form keeps both views current.
	if err := h.store.SaveWeight(userID, resolveDay(r), profile.WeightKG); err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	flash.Set(w, "notice", h.t(r, "goals.saved"), h.cfg.CookieSecure())
	http.Redirect(w, r, "/diario", http.StatusSeeOther)
}

// validateProfile reports every problem at once so the user fixes the form in
// one pass instead of submitting four times.
func validateProfile(p nutrition.Profile) map[string]string {
	errs := map[string]string{}
	if p.Sex != nutrition.SexFemale && p.Sex != nutrition.SexMale {
		errs["sex"] = "Informe o sexo biológico."
	}
	if p.AgeYears < 14 || p.AgeYears > 100 {
		errs["age_years"] = "Informe uma idade entre 14 e 100 anos."
	}
	// Height and weight bounds are the plausible-adult range: outside it
	// Mifflin-St Jeor degenerates and the budget would be meaningless.
	if p.HeightCM < 120 || p.HeightCM > 230 {
		errs["height_cm"] = "Informe uma altura entre 120 e 230 cm."
	}
	if p.WeightKG < 35 || p.WeightKG > 300 {
		errs["weight_kg"] = "Informe um peso entre 35 e 300 kg."
	}
	if _, ok := nutritionActivityFactor(p.Activity); !ok {
		errs["activity"] = "Escolha um nível de atividade."
	}
	if _, ok := nutritionGoalOptions()[p.Goal]; !ok {
		errs["direction"] = "Escolha um objetivo."
	}
	return errs
}

// activityOptions is the select payload. The labels are i18n keys resolved in
// the template so a locale switch does not need a second map.
func activityOptions() []Option {
	return []Option{
		{Value: string(nutrition.ActivitySedentary), LabelKey: "goals.sedentary"},
		{Value: string(nutrition.ActivityLight), LabelKey: "goals.light"},
		{Value: string(nutrition.ActivityModerate), LabelKey: "goals.moderate"},
		{Value: string(nutrition.ActivityActive), LabelKey: "goals.active"},
	}
}

func directionOptions() []Option {
	return []Option{
		{Value: string(nutrition.GoalLose), LabelKey: "goals.lose"},
		{Value: string(nutrition.GoalMaintain), LabelKey: "goals.maintain"},
		{Value: string(nutrition.GoalGain), LabelKey: "goals.gain"},
	}
}

// Option is one <option> of a goal form select.
type Option struct {
	Value    string
	LabelKey string
}

func nutritionActivityFactor(a nutrition.Activity) (float64, bool) {
	for _, opt := range activityOptions() {
		if opt.Value == string(a) {
			return 1, true
		}
	}
	return 0, false
}

func nutritionGoalOptions() map[nutrition.Goal]struct{} {
	out := map[nutrition.Goal]struct{}{}
	for _, opt := range directionOptions() {
		out[nutrition.Goal(opt.Value)] = struct{}{}
	}
	return out
}
