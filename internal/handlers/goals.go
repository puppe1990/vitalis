package handlers

import (
	"fmt"
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
		// ManualTarget keeps the typed number in the form after a validation
		// error; TargetSource labels the card as pinned instead of calculated.
		"ManualTarget": goal.TargetOverride,
		"TargetSource": nutrition.TargetSource(goal.TargetOverride),
	}), status)
}

// Save recomputes the budget from the profile, unless the user pinned a target.
// A pinned number is stored as an override and the macros are split from it, so
// the UI, the API and the MCP server all read one value.
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

	override := parseFloatDefault(r.PostFormValue("target_kcal"), 0)
	errs := validateProfile(profile, override)
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

	// A pinned target wins over the formula. An empty field means "derive",
	// which is what the form sends when the user has not typed a number.
	goals := nutrition.GoalsFor(profile)
	if override > 0 {
		goals = nutrition.GoalsFromTarget(override, profile.Goal, goals.WaterML)
	}

	userID := currentUserID(r)
	goal := models.Goal{
		TargetOverride: override,
		UserID:         userID,
		TargetKcal:     goals.TargetKcal,
		ProteinG:       goals.ProteinG,
		CarbG:          goals.CarbG,
		FatG:           goals.FatG,
		WaterML:        goals.WaterML,
		Sex:            profile.Sex,
		AgeYears:       profile.AgeYears,
		HeightCM:       profile.HeightCM,
		WeightKG:       profile.WeightKG,
		Activity:       profile.Activity,
		Direction:      profile.Goal,
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
func validateProfile(p nutrition.Profile, override float64) map[string]string {
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
	if !p.Activity.Valid() {
		errs["activity"] = "Escolha um nível de atividade."
	}
	if !p.Goal.Valid() {
		errs["direction"] = "Escolha um objetivo."
	}
	if override != 0 && (override < nutrition.MinManualTargetKcal || override > nutrition.MaxManualTargetKcal) {
		errs["target_kcal"] = fmt.Sprintf("A meta deve ficar entre %d e %d kcal.",
			int(nutrition.MinManualTargetKcal), int(nutrition.MaxManualTargetKcal))
	}
	return errs
}

// activityOptions is the pill-group payload. The labels are i18n keys resolved
// in the template so a locale switch does not need a second map.
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

// Option is one radio pill of the goal form.
type Option struct {
	Value    string
	LabelKey string
}
