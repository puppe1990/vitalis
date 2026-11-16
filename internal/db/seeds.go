package db

import (
	"log"

	"github.com/puppe1990/vitalis/internal/store"
)

// RunSeeds populates demo data. Safe to run multiple times.
func RunSeeds(s store.Store) error {
	// cais:recurring-seeds
	// cais:seeds
	return nil
}

// RunBootSeeds runs the catalog seed on every boot. The starter food catalog is
// content, not demo data: every user needs it, and SeedFoods upserts on name so
// re-running it cannot duplicate rows.
func RunBootSeeds(s store.Store) {
	if err := s.SeedFoods(store.StarterFoods()); err != nil {
		log.Printf("boot seeds: food catalog: %v", err)
	}
}
