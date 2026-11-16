package nutrition

import "math"

// Sex drives the Mifflin-St Jeor constant, the only place Vitalis needs it.
type Sex string

const (
	SexFemale Sex = "female"
	SexMale   Sex = "male"
)

// Activity is the multiplier applied to BMR.
type Activity string

const (
	ActivitySedentary Activity = "sedentary"
	ActivityLight     Activity = "light"
	ActivityModerate  Activity = "moderate"
	ActivityActive    Activity = "active"
)

// Goal is the direction of the calorie budget.
type Goal string

const (
	GoalLose     Goal = "lose"
	GoalMaintain Goal = "maintain"
	GoalGain     Goal = "gain"
)

// activityFactor maps the four choices to the standard Harris-Benedict bands.
// Values are deliberately the conservative end of each band so the app does not
// hand out a budget the user cannot eat.
var activityFactor = map[Activity]float64{
	ActivitySedentary: 1.2,
	ActivityLight:     1.375,
	ActivityModerate:  1.55,
	ActivityActive:    1.725,
}

// goalDelta is the daily kcal adjustment. 500 kcal/day ≈ 0.45 kg/week.
const goalDelta = 500

// Macro split by goal, as share of the calorie target. Higher protein when
// cutting preserves lean mass; maintenance and gain spread wider.
var goalMacroSplit = map[Goal]MacroPercent{
	GoalLose:     {Protein: 35, Carb: 35, Fat: 30},
	GoalMaintain: {Protein: 30, Carb: 40, Fat: 30},
	GoalGain:     {Protein: 25, Carb: 45, Fat: 30},
}

// MinTargetKcal is the intake floor. Below it the app would be recommending a
// deficit no adult should follow, so GoalsFor clamps and the UI says so.
const MinTargetKcal = 1200

// DefaultTargetKcal is what an incomplete profile gets, so the dashboard still
// renders before the user fills in the goal form.
const DefaultTargetKcal = 2000

// Profile is the subset of the user's goal form the calculation needs.
type Profile struct {
	Sex      Sex      `json:"sex"`
	AgeYears int      `json:"age_years"`
	HeightCM float64  `json:"height_cm"`
	WeightKG float64  `json:"weight_kg"`
	Activity Activity `json:"activity"`
	Goal     Goal     `json:"goal"`
}

// Goals is everything the app derives from a profile: the calorie target, the
// three macro targets in grams and the hydration target.
type Goals struct {
	TargetKcal float64 `json:"target_kcal"`
	ProteinG   float64 `json:"protein_g"`
	CarbG      float64 `json:"carb_g"`
	FatG       float64 `json:"fat_g"`
	WaterML    float64 `json:"water_ml"`
	BMR        float64 `json:"bmr"`
}

// GoalsFor runs Mifflin-St Jeor, applies the activity factor and the goal
// delta, then splits the result into macros.
//
// Water follows 35 ml/kg because that is the figure Brazilian nutrition
// guidance uses, clamped so an outlier weight does not produce an absurd target.
func GoalsFor(p Profile) Goals {
	if !p.complete() {
		return defaultGoals()
	}

	bmr := mifflinStJeor(p)
	factor, ok := activityFactor[p.Activity]
	if !ok {
		factor = activityFactor[ActivityModerate]
	}

	target := bmr * factor
	switch p.Goal {
	case GoalLose:
		target -= goalDelta
	case GoalGain:
		target += goalDelta
	}
	if target < MinTargetKcal {
		target = MinTargetKcal
	}

	split, ok := goalMacroSplit[p.Goal]
	if !ok {
		split = goalMacroSplit[GoalMaintain]
	}

	return Goals{
		TargetKcal: math.Round(target),
		ProteinG:   math.Round(target * split.Protein / 100 / KcalPerProteinGram),
		CarbG:      math.Round(target * split.Carb / 100 / KcalPerCarbGram),
		FatG:       math.Round(target * split.Fat / 100 / KcalPerFatGram),
		WaterML:    math.Round(clamp(p.WeightKG*35, 1500, 4000)),
		BMR:        math.Round(bmr),
	}
}

// complete guards the formula: Mifflin-St Jeor leaves its valid range with a
// missing height or a zero weight, and would otherwise return nonsense.
func (p Profile) complete() bool {
	return p.AgeYears > 0 && p.HeightCM > 0 && p.WeightKG > 0 &&
		(p.Sex == SexFemale || p.Sex == SexMale)
}

// mifflinStJeor is 10w + 6.25h - 5a + s, where s is +5 for men and -161 for
// women. The female constant is why the two branches cannot be one expression.
func mifflinStJeor(p Profile) float64 {
	base := 10*p.WeightKG + 6.25*p.HeightCM - 5*float64(p.AgeYears)
	if p.Sex == SexMale {
		return base + 5
	}
	return base - 161
}

func defaultGoals() Goals {
	split := goalMacroSplit[GoalMaintain]
	return Goals{
		TargetKcal: DefaultTargetKcal,
		ProteinG:   math.Round(DefaultTargetKcal * split.Protein / 100 / KcalPerProteinGram),
		CarbG:      math.Round(DefaultTargetKcal * split.Carb / 100 / KcalPerCarbGram),
		FatG:       math.Round(DefaultTargetKcal * split.Fat / 100 / KcalPerFatGram),
		WaterML:    2500,
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func withActivity(p Profile, a Activity) Profile {
	p.Activity = a
	return p
}
