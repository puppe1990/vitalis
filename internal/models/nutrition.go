package models

import (
	"time"

	"github.com/puppe1990/vitalis/internal/nutrition"
)

// Food is a catalog item. Nutrients are stored per 100 g so a portion is a
// multiplication, never a re-entry of the label.
type Food struct {
	ID           int64
	Name         string
	Brand        string
	Category     string
	Barcode      string
	Per100g      nutrition.Nutrients
	PortionLabel string
	PortionGrams float64
	Verified     bool
	CreatedAt    time.Time
}

// Entry is one logged portion on a given day and meal.
type Entry struct {
	ID        int64
	UserID    int64
	FoodID    int64
	Meal      nutrition.Meal
	LoggedOn  time.Time
	Grams     float64
	Nutrients nutrition.Nutrients
	CreatedAt time.Time

	// FoodName and FoodCategory are joined for list rendering so a page does
	// not need a second query per row.
	FoodName     string
	FoodCategory string
}

// Goal is the user's daily budget. Stored as a row per user (upserted) because
// every page and the API need exactly one current value.
type Goal struct {
	UserID int64
	// TargetOverride is a calorie target the user pinned by hand. 0 means the
	// target came from the profile formula; a positive value wins over it.
	TargetOverride float64
	TargetKcal     float64
	ProteinG       float64
	CarbG          float64
	FatG           float64
	WaterML        float64
	Sex            nutrition.Sex
	AgeYears       int
	HeightCM       float64
	WeightKG       float64
	Activity       nutrition.Activity
	Direction      nutrition.Goal
	UpdatedAt      time.Time
}

// DayLog is the per-day aggregate the dashboard and weekly chart read.
type DayLog struct {
	UserID     int64
	LoggedOn   time.Time
	WaterML    float64
	BurnedKcal float64
	WeightKG   float64
}

// Profile is the subset the goal form edits.
func (g Goal) Profile() nutrition.Profile {
	return nutrition.Profile{
		Sex:      g.Sex,
		AgeYears: g.AgeYears,
		HeightCM: g.HeightCM,
		WeightKG: g.WeightKG,
		Activity: g.Activity,
		Goal:     g.Direction,
	}
}

// APIToken authenticates the MCP server. Only the SHA-256 hash is stored so a
// leaked DB or backup cannot be replayed against the API.
type APIToken struct {
	ID        int64
	UserID    int64
	Label     string
	CreatedAt time.Time
}

// Weight is one weigh-in on the progression chart.
type Weight struct {
	ID       int64
	UserID   int64
	LoggedOn time.Time
	KG       float64
}

// MealPhoto is one plate photo attached to a meal on a day. The file lives in
// Backblaze B2 under B2Key; SQLite only stores the index the diary needs to
// list and authorize a download.
type MealPhoto struct {
	ID          int64
	UserID      int64
	Meal        nutrition.Meal
	LoggedOn    time.Time
	B2Key       string
	ContentType string
	ByteSize    int64
	CreatedAt   time.Time
}
