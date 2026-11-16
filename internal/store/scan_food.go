package store

import (
	"database/sql"

	"github.com/puppe1990/vitalis/internal/models"
)

// rowScanner is the shared shape of *sql.Row and *sql.Rows, so FindFood and
// ListFoods scan a food through one function.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanFood(row rowScanner) (models.Food, error) {
	var f models.Food
	var verified int
	err := row.Scan(
		&f.ID, &f.Name, &f.Brand, &f.Category, &f.Barcode,
		&f.Per100g.Kcal, &f.Per100g.Protein, &f.Per100g.Carb, &f.Per100g.Fat,
		&f.PortionLabel, &f.PortionGrams, &verified, &f.CreatedAt,
	)
	if err != nil {
		return models.Food{}, err
	}
	f.Verified = verified == 1
	return f, nil
}

var _ rowScanner = (*sql.Row)(nil)
var _ rowScanner = (*sql.Rows)(nil)
