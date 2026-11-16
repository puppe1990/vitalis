package nutrition

import (
	"testing"

	"github.com/brianvoe/gofakeit/v7"
)

// TestGoals_fromProfileDerivesMacroTargets pins the Mifflin-St Jeor path: an
// adult with real inputs must land in a plausible band, not an arbitrary one.
func TestGoals_fromProfileDerivesMacroTargets(t *testing.T) {
	f := gofakeit.New(20261116)
	p := Profile{
		Sex:      SexFemale,
		AgeYears: f.IntRange(25, 45),
		HeightCM: f.Float64Range(158, 175),
		WeightKG: f.Float64Range(55, 80),
		Activity: ActivityModerate,
		Goal:     GoalLose,
	}

	got := GoalsFor(p)

	if got.TargetKcal < 1200 || got.TargetKcal > 3200 {
		t.Errorf("TargetKcal = %v, want within [1200, 3200]", got.TargetKcal)
	}
	// 4/4/9 kcal per gram must reconcile with the calorie target.
	sum := got.ProteinG*KcalPerProteinGram + got.CarbG*KcalPerCarbGram + got.FatG*KcalPerFatGram
	if diff := sum - got.TargetKcal; diff > got.TargetKcal*0.02 || diff < -got.TargetKcal*0.02 {
		t.Errorf("macro kcal %.1f drifts from target %.1f", sum, got.TargetKcal)
	}
	if got.WaterML <= 0 {
		t.Errorf("WaterML = %v, want positive", got.WaterML)
	}
}

func TestGoals_maintenanceIsAboveLose(t *testing.T) {
	p := Profile{
		Sex: SexMale, AgeYears: 30, HeightCM: 180, WeightKG: 82,
		Activity: ActivityModerate, Goal: GoalLose,
	}
	lose := GoalsFor(p)
	p.Goal = GoalMaintain
	maintain := GoalsFor(p)
	p.Goal = GoalGain
	gain := GoalsFor(p)

	if !(maintain.TargetKcal > lose.TargetKcal) {
		t.Errorf("maintain %v <= lose %v", maintain.TargetKcal, lose.TargetKcal)
	}
	if !(gain.TargetKcal > maintain.TargetKcal) {
		t.Errorf("gain %v <= maintain %v", gain.TargetKcal, maintain.TargetKcal)
	}
}

func TestGoals_neverGoesBelowSafetyFloor(t *testing.T) {
	p := Profile{
		Sex: SexFemale, AgeYears: 70, HeightCM: 150, WeightKG: 42,
		Activity: ActivitySedentary, Goal: GoalLose,
	}
	if got := GoalsFor(p); got.TargetKcal < MinTargetKcal {
		t.Errorf("TargetKcal = %v, want >= floor %v", got.TargetKcal, MinTargetKcal)
	}
}

func TestGoals_incompleteProfileFallsBackToDefaults(t *testing.T) {
	got := GoalsFor(Profile{})
	if got.TargetKcal != DefaultTargetKcal {
		t.Errorf("TargetKcal = %v, want default %v", got.TargetKcal, DefaultTargetKcal)
	}
	if got.ProteinG <= 0 || got.CarbG <= 0 || got.FatG <= 0 {
		t.Errorf("macros = %+v, want all positive", got)
	}
}

func TestGoals_activityRaisesTarget(t *testing.T) {
	base := Profile{Sex: SexMale, AgeYears: 30, HeightCM: 180, WeightKG: 80, Goal: GoalMaintain}
	sedentary := GoalsFor(withActivity(base, ActivitySedentary))
	active := GoalsFor(withActivity(base, ActivityActive))

	if !(active.TargetKcal > sedentary.TargetKcal) {
		t.Errorf("active %v <= sedentary %v", active.TargetKcal, sedentary.TargetKcal)
	}
}

// The handlers and the JSON API both validate with these, so a drift between
// the accepted list and the calculation's own table would be a real bug.
func TestActivity_ValidMatchesTheFactorTable(t *testing.T) {
	for _, a := range []Activity{ActivitySedentary, ActivityLight, ActivityModerate, ActivityActive} {
		if !a.Valid() {
			t.Errorf("%q should be valid", a)
		}
	}
	for _, a := range []Activity{"", "extreme", "ACTIVE"} {
		if Activity(a).Valid() {
			t.Errorf("%q should be invalid", a)
		}
	}
}

func TestGoal_ValidMatchesTheSplitTable(t *testing.T) {
	for _, g := range []Goal{GoalLose, GoalMaintain, GoalGain} {
		if !g.Valid() {
			t.Errorf("%q should be valid", g)
		}
	}
	for _, g := range []Goal{"", "bulk", "LOSE"} {
		if Goal(g).Valid() {
			t.Errorf("%q should be invalid", g)
		}
	}
}
