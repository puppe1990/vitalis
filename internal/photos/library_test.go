package photos

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/store"
	"github.com/puppe1990/vitalis/internal/testdata"
)

func jpegBytes() []byte {
	return []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
}

func TestLibrary_EnabledWhenBucketIsSet(t *testing.T) {
	if New(nil, nil).Enabled() {
		t.Error("nil bucket must disable the feature")
	}
	lib := New(nil, NewMemoryBucket())
	if !lib.Enabled() {
		t.Error("memory bucket must enable the feature")
	}
}

func TestLibrary_SavePutsJPEGAndRecordsRow(t *testing.T) {
	s := newPhotoStore(t)
	userID := newPhotoUser(t, s)
	bucket := NewMemoryBucket()
	lib := New(s, bucket)
	day := testdata.Day(2026, 11, 16)

	photo, err := lib.Save(context.Background(), SaveInput{
		UserID:   userID,
		Meal:     nutrition.MealLunch,
		LoggedOn: day,
		Body:     jpegBytes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if photo.ID == 0 {
		t.Fatal("Save returned id 0")
	}
	if photo.ContentType != "image/jpeg" {
		t.Errorf("ContentType = %q", photo.ContentType)
	}
	wantPrefix := "users/1/2026-11-16/almoco/"
	if !strings.HasPrefix(photo.B2Key, wantPrefix) || !strings.HasSuffix(photo.B2Key, ".jpg") {
		t.Errorf("B2Key = %q, want %s<uuid>.jpg", photo.B2Key, wantPrefix)
	}

	got, err := bucket.Open(context.Background(), photo.B2Key)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = got.Body.Close() }()
	body, _ := io.ReadAll(got.Body)
	if !bytes.Equal(body, jpegBytes()) {
		t.Errorf("stored bytes = %v", body)
	}
	if got.ContentType != "image/jpeg" {
		t.Errorf("stored type = %q", got.ContentType)
	}
}

func TestLibrary_SaveRejectsPDF(t *testing.T) {
	s := newPhotoStore(t)
	lib := New(s, NewMemoryBucket())

	_, err := lib.Save(context.Background(), SaveInput{
		UserID:   newPhotoUser(t, s),
		Meal:     nutrition.MealLunch,
		LoggedOn: testdata.Day(2026, 11, 16),
		Body:     []byte("%PDF-1.4"),
	})
	if !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("err = %v, want ErrUnsupportedType", err)
	}
}

func TestLibrary_OpenAndDeleteRoundTrip(t *testing.T) {
	s := newPhotoStore(t)
	userID := newPhotoUser(t, s)
	bucket := NewMemoryBucket()
	lib := New(s, bucket)
	day := testdata.Day(2026, 11, 16)

	photo, err := lib.Save(context.Background(), SaveInput{
		UserID: userID, Meal: nutrition.MealLunch, LoggedOn: day, Body: jpegBytes(),
	})
	if err != nil {
		t.Fatal(err)
	}

	opened, err := lib.Open(context.Background(), userID, photo.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Body.Close() }()
	body, _ := io.ReadAll(opened.Body)
	if !bytes.Equal(body, jpegBytes()) {
		t.Error("Open returned different bytes")
	}

	stranger, err := s.CreateUser("stranger@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Open(context.Background(), stranger, photo.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Open(stranger) err = %v, want ErrNotFound", err)
	}

	if err := lib.Delete(context.Background(), userID, photo.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := bucket.Open(context.Background(), photo.B2Key); !errors.Is(err, ErrNotFound) {
		t.Errorf("bucket after delete err = %v, want ErrNotFound", err)
	}
	listed, err := lib.List(userID, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Errorf("List after delete = %d", len(listed))
	}
}

func TestLibrary_ListGroupsByMeal(t *testing.T) {
	s := newPhotoStore(t)
	userID := newPhotoUser(t, s)
	lib := New(s, NewMemoryBucket())
	day := testdata.Day(2026, 11, 16)

	if _, err := lib.Save(context.Background(), SaveInput{
		UserID: userID, Meal: nutrition.MealLunch, LoggedOn: day, Body: jpegBytes(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Save(context.Background(), SaveInput{
		UserID: userID, Meal: nutrition.MealBreakfast, LoggedOn: day, Body: jpegBytes(),
	}); err != nil {
		t.Fatal(err)
	}

	listed, err := lib.List(userID, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("List = %d, want 2", len(listed))
	}
}

func TestLibrary_SaveCapsPerMeal(t *testing.T) {
	s := newPhotoStore(t)
	userID := newPhotoUser(t, s)
	lib := New(s, NewMemoryBucket())
	day := testdata.Day(2026, 11, 16)
	in := SaveInput{UserID: userID, Meal: nutrition.MealLunch, LoggedOn: day, Body: jpegBytes()}

	for i := 0; i < MaxPhotosPerMeal; i++ {
		if _, err := lib.Save(context.Background(), in); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	_, err := lib.Save(context.Background(), in)
	if !errors.Is(err, ErrTooMany) {
		t.Errorf("err = %v, want ErrTooMany", err)
	}
}

func newPhotoStore(t *testing.T) store.Store {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newPhotoUser(t *testing.T, s store.Store) int64 {
	t.Helper()
	id, err := s.CreateUser("photos@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	return id
}
