package photos

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/store"
)

// MaxPhotosPerMeal keeps one plate from turning into an album. Four photos
// cover a typical lunch (meat bowl + two plates); eight is the hard cap.
const MaxPhotosPerMeal = 8

var ErrTooMany = errors.New("meal already has the maximum number of photos")
var ErrNotFound = errors.New("photo not found")
var ErrDisabled = errors.New("photo storage is not configured")

// Object is one downloaded file from the bucket.
type Object struct {
	Body        io.ReadCloser
	ContentType string
}

// Bucket is the object store. Production talks to Backblaze B2; tests use
// MemoryBucket so the rest of the stack never needs credentials.
type Bucket interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string) (Object, error)
	Remove(ctx context.Context, key string) error
}

// Library is the diary's photo use case: sniff, put, index, serve, delete.
type Library struct {
	store  store.Store
	bucket Bucket
}

func New(s store.Store, bucket Bucket) *Library {
	return &Library{store: s, bucket: bucket}
}

func (l *Library) Enabled() bool {
	return l != nil && l.bucket != nil
}

// SaveInput is one uploaded plate photo.
type SaveInput struct {
	UserID   int64
	Meal     nutrition.Meal
	LoggedOn time.Time
	Body     []byte
}

func (l *Library) Save(ctx context.Context, in SaveInput) (models.MealPhoto, error) {
	if !l.Enabled() {
		return models.MealPhoto{}, ErrDisabled
	}
	kind, err := Sniff(in.Body)
	if err != nil {
		return models.MealPhoto{}, err
	}
	meal := nutrition.NormalizeMeal(string(in.Meal))
	existing, err := l.store.ListMealPhotos(in.UserID, in.LoggedOn)
	if err != nil {
		return models.MealPhoto{}, err
	}
	count := 0
	for _, p := range existing {
		if p.Meal == meal {
			count++
		}
	}
	if count >= MaxPhotosPerMeal {
		return models.MealPhoto{}, fmt.Errorf("%w: %d", ErrTooMany, MaxPhotosPerMeal)
	}

	key, err := objectKey(in.UserID, in.LoggedOn, meal, kind.Ext)
	if err != nil {
		return models.MealPhoto{}, err
	}
	if err := l.bucket.Put(ctx, key, bytes.NewReader(in.Body), int64(len(in.Body)), kind.ContentType); err != nil {
		return models.MealPhoto{}, fmt.Errorf("upload photo: %w", err)
	}

	id, err := l.store.CreateMealPhoto(models.MealPhoto{
		UserID:      in.UserID,
		Meal:        meal,
		LoggedOn:    in.LoggedOn,
		B2Key:       key,
		ContentType: kind.ContentType,
		ByteSize:    int64(len(in.Body)),
	})
	if err != nil {
		_ = l.bucket.Remove(ctx, key)
		return models.MealPhoto{}, err
	}
	return l.store.FindMealPhoto(in.UserID, id)
}

func (l *Library) Open(ctx context.Context, userID, id int64) (Object, error) {
	if !l.Enabled() {
		return Object{}, ErrDisabled
	}
	photo, err := l.store.FindMealPhoto(userID, id)
	if err != nil {
		return Object{}, err
	}
	return l.bucket.Open(ctx, photo.B2Key)
}

func (l *Library) Delete(ctx context.Context, userID, id int64) error {
	if !l.Enabled() {
		return ErrDisabled
	}
	photo, err := l.store.FindMealPhoto(userID, id)
	if err != nil {
		return err
	}
	if err := l.store.DeleteMealPhoto(userID, id); err != nil {
		return err
	}
	if err := l.bucket.Remove(ctx, photo.B2Key); err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("remove photo object: %w", err)
	}
	return nil
}

func (l *Library) List(userID int64, day time.Time) ([]models.MealPhoto, error) {
	if l == nil || l.store == nil {
		return nil, nil
	}
	return l.store.ListMealPhotos(userID, day)
}

func objectKey(userID int64, day time.Time, meal nutrition.Meal, ext string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("photo id: %w", err)
	}
	return fmt.Sprintf("users/%d/%s/%s/%s.%s",
		userID, day.UTC().Format(time.DateOnly), meal, hex.EncodeToString(raw[:]), ext), nil
}
