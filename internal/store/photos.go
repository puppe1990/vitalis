package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
)

func (s *SQLiteStore) CreateMealPhoto(p models.MealPhoto) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO meal_photos (user_id, meal, logged_on, b2_key, content_type, byte_size)
		VALUES (?, ?, ?, ?, ?, ?)`,
		p.UserID, string(p.Meal), formatDay(p.LoggedOn), p.B2Key, p.ContentType, p.ByteSize,
	)
	if err != nil {
		return 0, fmt.Errorf("create meal photo for user %d: %w", p.UserID, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create meal photo id: %w", err)
	}
	return id, nil
}

func (s *SQLiteStore) ListMealPhotos(userID int64, day time.Time) ([]models.MealPhoto, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, meal, logged_on, b2_key, content_type, byte_size, created_at
		FROM meal_photos
		WHERE user_id = ? AND logged_on = ?
		ORDER BY created_at ASC, id ASC`,
		userID, formatDay(day),
	)
	if err != nil {
		return nil, fmt.Errorf("list meal photos for user %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.MealPhoto
	for rows.Next() {
		p, err := scanMealPhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) FindMealPhoto(userID, id int64) (models.MealPhoto, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, meal, logged_on, b2_key, content_type, byte_size, created_at
		FROM meal_photos
		WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	p, err := scanMealPhoto(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.MealPhoto{}, fmt.Errorf("find meal photo %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return models.MealPhoto{}, fmt.Errorf("find meal photo %d: %w", id, err)
	}
	return p, nil
}

func (s *SQLiteStore) DeleteMealPhoto(userID, id int64) error {
	result, err := s.db.Exec("DELETE FROM meal_photos WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete meal photo %d: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete meal photo rows %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("delete meal photo %d: %w", id, ErrNotFound)
	}
	return nil
}

type mealPhotoScanner interface {
	Scan(dest ...any) error
}

func scanMealPhoto(row mealPhotoScanner) (models.MealPhoto, error) {
	var p models.MealPhoto
	var meal, loggedOn string
	if err := row.Scan(&p.ID, &p.UserID, &meal, &loggedOn, &p.B2Key, &p.ContentType, &p.ByteSize, &p.CreatedAt); err != nil {
		return models.MealPhoto{}, fmt.Errorf("scan meal photo: %w", err)
	}
	p.Meal = nutrition.Meal(meal)
	p.LoggedOn = parseDay(loggedOn)
	return p, nil
}
