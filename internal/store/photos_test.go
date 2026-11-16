package store

import (
	"errors"
	"testing"
	"time"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/testdata"
)

func TestCreateMealPhoto_roundTrips(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)
	day := testdata.Day(2026, 11, 16)

	id, err := s.CreateMealPhoto(models.MealPhoto{
		UserID:      userID,
		Meal:        nutrition.MealLunch,
		LoggedOn:    day,
		B2Key:       "users/1/2026-11-16/almoco/abc.jpg",
		ContentType: "image/jpeg",
		ByteSize:    1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("CreateMealPhoto returned id 0")
	}

	got, err := s.FindMealPhoto(userID, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meal != nutrition.MealLunch {
		t.Errorf("Meal = %q", got.Meal)
	}
	if got.B2Key != "users/1/2026-11-16/almoco/abc.jpg" {
		t.Errorf("B2Key = %q", got.B2Key)
	}
	if got.ContentType != "image/jpeg" || got.ByteSize != 1200 {
		t.Errorf("meta = %+v", got)
	}
	if got.LoggedOn != day {
		t.Errorf("LoggedOn = %v, want %v", got.LoggedOn, day)
	}
}

func TestListMealPhotos_isScopedToUserAndDay(t *testing.T) {
	s := newTestStore(t)
	userA := newTestUser(t, s)
	userB, err := s.CreateUser("other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	day := testdata.Day(2026, 11, 16)

	mustPhoto(t, s, userA, nutrition.MealLunch, day, "a.jpg")
	mustPhoto(t, s, userB, nutrition.MealLunch, day, "b.jpg")
	mustPhoto(t, s, userA, nutrition.MealBreakfast, testdata.Day(2026, 11, 17), "c.jpg")
	mustPhoto(t, s, userA, nutrition.MealDinner, day, "d.jpg")

	got, err := s.ListMealPhotos(userA, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("ListMealPhotos = %d, want 2", len(got))
	}
	if got[0].Meal != nutrition.MealLunch || got[1].Meal != nutrition.MealDinner {
		t.Errorf("order = %q then %q", got[0].Meal, got[1].Meal)
	}
}

func TestFindMealPhoto_otherUserIsNotFound(t *testing.T) {
	s := newTestStore(t)
	owner := newTestUser(t, s)
	stranger, err := s.CreateUser("stranger@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	id := mustPhoto(t, s, owner, nutrition.MealLunch, testdata.Day(2026, 11, 16), "a.jpg")

	if _, err := s.FindMealPhoto(stranger, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindMealPhoto(stranger) err = %v, want ErrNotFound", err)
	}
}

func TestDeleteMealPhoto_isScopedToUser(t *testing.T) {
	s := newTestStore(t)
	owner := newTestUser(t, s)
	stranger, err := s.CreateUser("stranger@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	day := testdata.Day(2026, 11, 16)
	id := mustPhoto(t, s, owner, nutrition.MealLunch, day, "a.jpg")

	if err := s.DeleteMealPhoto(stranger, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteMealPhoto(stranger) err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteMealPhoto(owner, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FindMealPhoto(owner, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find after delete err = %v, want ErrNotFound", err)
	}
}

func mustPhoto(t *testing.T, s Store, userID int64, meal nutrition.Meal, day time.Time, key string) int64 {
	t.Helper()
	id, err := s.CreateMealPhoto(models.MealPhoto{
		UserID:      userID,
		Meal:        meal,
		LoggedOn:    day,
		B2Key:       key,
		ContentType: "image/jpeg",
		ByteSize:    100,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
