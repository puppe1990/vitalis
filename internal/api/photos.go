package api

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/photos"
	"github.com/puppe1990/vitalis/internal/store"
)

type photoJSON struct {
	ID          int64  `json:"id"`
	Meal        string `json:"meal"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
	Date        string `json:"date"`
}

func toPhotoJSON(p models.MealPhoto) photoJSON {
	return photoJSON{
		ID:          p.ID,
		Meal:        string(p.Meal),
		ContentType: p.ContentType,
		ByteSize:    p.ByteSize,
		Date:        p.LoggedOn.Format(time.DateOnly),
	}
}

func (h *Handler) uploadPhoto(w http.ResponseWriter, r *http.Request) {
	if !h.photos.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "photo storage is not configured")
		return
	}
	day, err := resolveDay(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, photos.MaxBytes+1<<20)
	if err := r.ParseMultipartForm(photos.MaxBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "image exceeds 8 MB")
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "photo file is required")
		return
	}
	defer func() { _ = file.Close() }()

	body, err := io.ReadAll(io.LimitReader(file, photos.MaxBytes+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	saved, err := h.photos.Save(r.Context(), photos.SaveInput{
		UserID:   userIDFrom(r),
		Meal:     nutrition.NormalizeMeal(r.FormValue("meal")),
		LoggedOn: day,
		Body:     body,
	})
	if err != nil {
		writePhotoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"photo": toPhotoJSON(saved)})
}

func (h *Handler) getPhoto(w http.ResponseWriter, r *http.Request) {
	if !h.photos.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "photo storage is not configured")
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	obj, err := h.photos.Open(r.Context(), userIDFrom(r), id)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, photos.ErrNotFound) {
		writeError(w, http.StatusNotFound, "photo not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer func() { _ = obj.Body.Close() }()
	if obj.ContentType != "" {
		w.Header().Set("Content-Type", obj.ContentType)
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, obj.Body)
}

func (h *Handler) deletePhoto(w http.ResponseWriter, r *http.Request) {
	if !h.photos.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "photo storage is not configured")
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.photos.Delete(r.Context(), userIDFrom(r), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "photo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writePhotoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, photos.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "image exceeds 8 MB")
	case errors.Is(err, photos.ErrTooMany):
		writeError(w, http.StatusUnprocessableEntity, "meal already has the maximum number of photos")
	case errors.Is(err, photos.ErrUnsupportedType):
		writeError(w, http.StatusUnprocessableEntity, "upload a JPEG, PNG, WebP or HEIC image")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
