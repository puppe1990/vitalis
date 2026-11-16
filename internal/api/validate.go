package api

import (
	"fmt"
	"strconv"

	"github.com/puppe1990/vitalis/internal/nutrition"
)

func parseID(raw string) (int64, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("id %q must be a positive integer: %w", raw, err)
	}
	return v, nil
}

func parsePositive(raw string) (int, error) {
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("limit %q must be a positive integer: %w", raw, err)
	}
	return v, nil
}

// validateProfile mirrors the browser form's server-side checks. The MCP client
// hits the same rules so a bad payload cannot create a nonsensical target.
func validateProfile(p nutrition.Profile) error {
	if p.Sex != nutrition.SexFemale && p.Sex != nutrition.SexMale {
		return fmt.Errorf("sex %q must be female or male", p.Sex)
	}
	if p.AgeYears < 14 || p.AgeYears > 100 {
		return fmt.Errorf("age_years %d must be between 14 and 100", p.AgeYears)
	}
	if p.HeightCM < 120 || p.HeightCM > 230 {
		return fmt.Errorf("height_cm %v must be between 120 and 230", p.HeightCM)
	}
	if p.WeightKG < 35 || p.WeightKG > 300 {
		return fmt.Errorf("weight_kg %v must be between 35 and 300", p.WeightKG)
	}
	switch p.Activity {
	case nutrition.ActivitySedentary, nutrition.ActivityLight,
		nutrition.ActivityModerate, nutrition.ActivityActive:
	default:
		return fmt.Errorf("activity %q must be sedentary, light, moderate or active", p.Activity)
	}
	switch p.Goal {
	case nutrition.GoalLose, nutrition.GoalMaintain, nutrition.GoalGain:
	default:
		return fmt.Errorf("direction %q must be lose, maintain or gain", p.Goal)
	}
	return nil
}
