package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
)

// ErrNotFound is the single not-found sentinel shared by the store and the
// handlers, so a missing row maps to 404 instead of a 500.
var ErrNotFound = errors.New("not found")

// dayLayout is the storage format for a DATE column. SQLite has no date type,
// so the app owns the format and every query binds this layout.
const dayLayout = "2006-01-02"

func formatDay(t time.Time) string {
	return t.UTC().Format(dayLayout)
}

func parseDay(raw string) time.Time {
	// A DATE column can come back as "2026-11-16" or with a trailing time when
	// a row was written by a driver that ignored the column affinity.
	for _, layout := range []string{dayLayout, dayLayout + " 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func (s *SQLiteStore) CreateFood(f models.Food) (int64, error) {
	// The catalog is upserted on name so a seed re-run and a repeated barcode
	// scan converge on one row instead of duplicating the food.
	result, err := s.db.Exec(`
		INSERT INTO foods (name, brand, category, barcode, kcal_100g, protein_100g,
			carb_100g, fat_100g, portion_label, portion_grams, verified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			brand = excluded.brand,
			category = excluded.category,
			barcode = excluded.barcode,
			kcal_100g = excluded.kcal_100g,
			protein_100g = excluded.protein_100g,
			carb_100g = excluded.carb_100g,
			fat_100g = excluded.fat_100g,
			portion_label = excluded.portion_label,
			portion_grams = excluded.portion_grams,
			verified = excluded.verified`,
		f.Name, f.Brand, f.Category, f.Barcode,
		f.Per100g.Kcal, f.Per100g.Protein, f.Per100g.Carb, f.Per100g.Fat,
		f.PortionLabel, f.PortionGrams, boolInt(f.Verified),
	)
	if err != nil {
		return 0, fmt.Errorf("create food %q: %w", f.Name, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create food id %q: %w", f.Name, err)
	}
	// ON CONFLICT DO UPDATE leaves LastInsertId at 0 on the update path, so
	// resolve the row by its unique name instead of returning a useless id.
	if id == 0 {
		var existing int64
		if err := s.db.QueryRow("SELECT id FROM foods WHERE name = ?", f.Name).Scan(&existing); err != nil {
			return 0, fmt.Errorf("resolve food id %q: %w", f.Name, err)
		}
		return existing, nil
	}
	return id, nil
}

func (s *SQLiteStore) FindFood(id int64) (models.Food, error) {
	row := s.db.QueryRow(`
		SELECT id, name, brand, category, barcode, kcal_100g, protein_100g, carb_100g,
			fat_100g, portion_label, portion_grams, verified, created_at
		FROM foods WHERE id = ?`, id)

	f, err := scanFood(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Food{}, fmt.Errorf("find food %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return models.Food{}, fmt.Errorf("find food %d: %w", id, err)
	}
	return f, nil
}

func (s *SQLiteStore) ListFoods(search, category string, limit int) ([]models.Food, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, name, brand, category, barcode, kcal_100g, protein_100g, carb_100g,
			fat_100g, portion_label, portion_grams, verified, created_at
		FROM foods
		WHERE (? = '' OR name LIKE '%' || ? || '%' OR brand LIKE '%' || ? || '%')
		  AND (? = '' OR category = ?)
		ORDER BY (category = '') ASC, name COLLATE NOCASE ASC
		LIMIT ?`,
		search, search, search, category, category, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list foods: %w", err)
	}
	defer func() { _ = rows.Close() }()

	foods := make([]models.Food, 0, limit)
	for rows.Next() {
		f, err := scanFood(rows)
		if err != nil {
			return nil, fmt.Errorf("scan food: %w", err)
		}
		foods = append(foods, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list foods rows: %w", err)
	}
	return foods, nil
}

func (s *SQLiteStore) ListCategories() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT category FROM foods WHERE category <> '' ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SeedFoods(foods []models.Food) error {
	for _, f := range foods {
		if _, err := s.CreateFood(f); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) UpsertGoal(g models.Goal) error {
	if g.WaterML <= 0 {
		g.WaterML = 2500
	}
	_, err := s.db.Exec(`
		INSERT INTO goals (user_id, target_kcal, protein_g, carb_g, fat_g, water_ml,
			sex, age_years, height_cm, weight_kg, activity, direction, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			target_kcal = excluded.target_kcal,
			protein_g = excluded.protein_g,
			carb_g = excluded.carb_g,
			fat_g = excluded.fat_g,
			water_ml = excluded.water_ml,
			sex = excluded.sex,
			age_years = excluded.age_years,
			height_cm = excluded.height_cm,
			weight_kg = excluded.weight_kg,
			activity = excluded.activity,
			direction = excluded.direction,
			updated_at = CURRENT_TIMESTAMP`,
		g.UserID, g.TargetKcal, g.ProteinG, g.CarbG, g.FatG, g.WaterML,
		string(g.Sex), g.AgeYears, g.HeightCM, g.WeightKG,
		string(g.Activity), string(g.Direction),
	)
	if err != nil {
		return fmt.Errorf("upsert goal for user %d: %w", g.UserID, err)
	}
	return nil
}

func (s *SQLiteStore) GetGoal(userID int64) (models.Goal, error) {
	var g models.Goal
	var sex, activity, direction string
	err := s.db.QueryRow(`
		SELECT user_id, target_kcal, protein_g, carb_g, fat_g, water_ml,
			sex, age_years, height_cm, weight_kg, activity, direction, updated_at
		FROM goals WHERE user_id = ?`, userID,
	).Scan(&g.UserID, &g.TargetKcal, &g.ProteinG, &g.CarbG, &g.FatG, &g.WaterML,
		&sex, &g.AgeYears, &g.HeightCM, &g.WeightKG, &activity, &direction, &g.UpdatedAt)

	// A user who has not opened the goal form yet still needs a dashboard, so a
	// missing row resolves to derived defaults instead of an error page.
	if errors.Is(err, sql.ErrNoRows) {
		return defaultGoal(userID), nil
	}
	if err != nil {
		return models.Goal{}, fmt.Errorf("get goal for user %d: %w", userID, err)
	}

	g.Sex = nutrition.Sex(sex)
	g.Activity = nutrition.Activity(activity)
	g.Direction = nutrition.Goal(direction)
	return g, nil
}

func defaultGoal(userID int64) models.Goal {
	goals := nutrition.GoalsFor(nutrition.Profile{})
	return models.Goal{
		UserID:     userID,
		TargetKcal: goals.TargetKcal,
		ProteinG:   goals.ProteinG,
		CarbG:      goals.CarbG,
		FatG:       goals.FatG,
		WaterML:    goals.WaterML,
		Activity:   nutrition.ActivityModerate,
		Direction:  nutrition.GoalMaintain,
	}
}

func (s *SQLiteStore) CreateEntry(e models.Entry) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO entries (user_id, food_id, meal, logged_on, grams, kcal, protein_g, carb_g, fat_g)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.UserID, e.FoodID, string(e.Meal), formatDay(e.LoggedOn), e.Grams,
		e.Nutrients.Kcal, e.Nutrients.Protein, e.Nutrients.Carb, e.Nutrients.Fat,
	)
	if err != nil {
		return 0, fmt.Errorf("create entry for user %d: %w", e.UserID, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create entry id: %w", err)
	}
	return id, nil
}

func (s *SQLiteStore) ListEntries(userID int64, day time.Time) ([]models.Entry, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.user_id, e.food_id, e.meal, e.logged_on, e.grams,
			e.kcal, e.protein_g, e.carb_g, e.fat_g, e.created_at,
			f.name, f.category
		FROM entries e
		JOIN foods f ON f.id = e.food_id
		WHERE e.user_id = ? AND e.logged_on = ?
		ORDER BY e.created_at ASC, e.id ASC`,
		userID, formatDay(day),
	)
	if err != nil {
		return nil, fmt.Errorf("list entries for user %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.Entry
	for rows.Next() {
		var e models.Entry
		var meal, loggedOn string
		if err := rows.Scan(&e.ID, &e.UserID, &e.FoodID, &meal, &loggedOn, &e.Grams,
			&e.Nutrients.Kcal, &e.Nutrients.Protein, &e.Nutrients.Carb, &e.Nutrients.Fat,
			&e.CreatedAt, &e.FoodName, &e.FoodCategory); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		e.Meal = nutrition.Meal(meal)
		e.LoggedOn = parseDay(loggedOn)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) DeleteEntry(userID, entryID int64) error {
	// user_id in the WHERE is the authorization check: an entry id from another
	// account must not be deletable, and it must read as "not found", not 403.
	result, err := s.db.Exec("DELETE FROM entries WHERE id = ? AND user_id = ?", entryID, userID)
	if err != nil {
		return fmt.Errorf("delete entry %d: %w", entryID, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete entry rows %d: %w", entryID, err)
	}
	if n == 0 {
		return fmt.Errorf("delete entry %d: %w", entryID, ErrNotFound)
	}
	return nil
}

func (s *SQLiteStore) AddWater(userID int64, day time.Time, deltaML float64) error {
	// MAX(0, …) keeps a stray negative delta from making the glass counter
	// negative; the UI only ever adds, but the API accepts a signed delta.
	_, err := s.db.Exec(`
		INSERT INTO day_logs (user_id, logged_on, water_ml)
		VALUES (?, ?, MAX(0, ?))
		ON CONFLICT(user_id, logged_on) DO UPDATE SET
			water_ml = MAX(0, day_logs.water_ml + ?)`,
		userID, formatDay(day), deltaML, deltaML,
	)
	if err != nil {
		return fmt.Errorf("add water for user %d: %w", userID, err)
	}
	return nil
}

func (s *SQLiteStore) SaveDayLog(d models.DayLog) error {
	_, err := s.db.Exec(`
		INSERT INTO day_logs (user_id, logged_on, water_ml, burned_kcal, weight_kg)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, logged_on) DO UPDATE SET
			water_ml = excluded.water_ml,
			burned_kcal = excluded.burned_kcal,
			weight_kg = excluded.weight_kg`,
		d.UserID, formatDay(d.LoggedOn), d.WaterML, d.BurnedKcal, d.WeightKG,
	)
	if err != nil {
		return fmt.Errorf("save day log for user %d: %w", d.UserID, err)
	}
	return nil
}

func (s *SQLiteStore) GetDayLog(userID int64, day time.Time) (models.DayLog, error) {
	var d models.DayLog
	var loggedOn string
	err := s.db.QueryRow(`
		SELECT user_id, logged_on, water_ml, burned_kcal, weight_kg
		FROM day_logs WHERE user_id = ? AND logged_on = ?`,
		userID, formatDay(day),
	).Scan(&d.UserID, &loggedOn, &d.WaterML, &d.BurnedKcal, &d.WeightKG)

	if errors.Is(err, sql.ErrNoRows) {
		return models.DayLog{UserID: userID, LoggedOn: day.UTC()}, nil
	}
	if err != nil {
		return models.DayLog{}, fmt.Errorf("get day log for user %d: %w", userID, err)
	}
	d.LoggedOn = parseDay(loggedOn)
	return d, nil
}

func (s *SQLiteStore) ListDayLogs(userID int64, since time.Time) ([]models.DayLog, error) {
	rows, err := s.db.Query(`
		SELECT user_id, logged_on, water_ml, burned_kcal, weight_kg
		FROM day_logs WHERE user_id = ? AND logged_on >= ?
		ORDER BY logged_on ASC`,
		userID, formatDay(since),
	)
	if err != nil {
		return nil, fmt.Errorf("list day logs for user %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.DayLog
	for rows.Next() {
		var d models.DayLog
		var loggedOn string
		if err := rows.Scan(&d.UserID, &loggedOn, &d.WaterML, &d.BurnedKcal, &d.WeightKG); err != nil {
			return nil, fmt.Errorf("scan day log: %w", err)
		}
		d.LoggedOn = parseDay(loggedOn)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SaveWeight(userID int64, day time.Time, kg float64) error {
	_, err := s.db.Exec(`
		INSERT INTO weights (user_id, logged_on, kg) VALUES (?, ?, ?)
		ON CONFLICT(user_id, logged_on) DO UPDATE SET kg = excluded.kg`,
		userID, formatDay(day), kg,
	)
	if err != nil {
		return fmt.Errorf("save weight for user %d: %w", userID, err)
	}
	return nil
}

func (s *SQLiteStore) ListWeights(userID int64, limit int) ([]models.Weight, error) {
	if limit <= 0 || limit > 365 {
		limit = 30
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, logged_on, kg FROM weights
		WHERE user_id = ? ORDER BY logged_on DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list weights for user %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.Weight
	for rows.Next() {
		var w models.Weight
		var loggedOn string
		if err := rows.Scan(&w.ID, &w.UserID, &loggedOn, &w.KG); err != nil {
			return nil, fmt.Errorf("scan weight: %w", err)
		}
		w.LoggedOn = parseDay(loggedOn)
		out = append(out, w)
	}
	return out, rows.Err()
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
