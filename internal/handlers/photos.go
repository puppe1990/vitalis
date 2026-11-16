package handlers

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"

	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/photos"
	"github.com/puppe1990/vitalis/internal/store"
)

// PhotosHandler uploads, serves and deletes meal plate photos. The diary
// handler only lists them; this type owns the multipart and the B2 round-trip.
type PhotosHandler struct {
	lib     *photos.Library
	catalog *i18n.Catalog
	cfg     cais.Config
}

func NewPhotosHandler(lib *photos.Library, catalog *i18n.Catalog, cfg cais.Config) *PhotosHandler {
	return &PhotosHandler{lib: lib, catalog: catalog, cfg: cfg}
}

func (h *PhotosHandler) t(r *http.Request, key string, args ...any) string {
	return translate(r, h.catalog, key, args...)
}

func (h *PhotosHandler) UploadPost(w http.ResponseWriter, r *http.Request) {
	day := resolveDay(r)
	back := dayPath("/diario", day)
	if !h.lib.Enabled() {
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, photos.MaxRequestBytes())
	if err := r.ParseMultipartForm(photos.MaxRequestBytes()); err != nil {
		flash.Set(w, "notice", h.t(r, "diario.photo_too_large"), h.cfg.CookieSecure())
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	var headers []*multipart.FileHeader
	if r.MultipartForm != nil {
		headers = r.MultipartForm.File["photo"]
	}
	if len(headers) == 0 {
		flash.Set(w, "notice", h.t(r, "diario.photo_invalid"), h.cfg.CookieSecure())
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	bodies, err := photos.ReadFileHeaders(headers)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}

	meal := nutrition.NormalizeMeal(r.FormValue("meal"))
	saved := 0
	var firstErr error
	for _, body := range bodies {
		_, err := h.lib.Save(r.Context(), photos.SaveInput{
			UserID:   currentUserID(r),
			Meal:     meal,
			LoggedOn: day,
			Body:     body,
		})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		saved++
	}
	if saved == 0 {
		msg := h.t(r, "diario.photo_invalid")
		if firstErr != nil {
			msg = h.photoError(r, firstErr)
		}
		flash.Set(w, "notice", msg, h.cfg.CookieSecure())
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	notice := h.t(r, "diario.photo_added")
	if saved > 1 {
		notice = h.t(r, "diario.photos_added", saved)
	}
	flash.Set(w, "notice", notice, h.cfg.CookieSecure())
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (h *PhotosHandler) Serve(w http.ResponseWriter, r *http.Request) {
	if !h.lib.Enabled() {
		http.NotFound(w, r)
		return
	}
	id, ok := parseInt64(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	obj, err := h.lib.Open(r.Context(), currentUserID(r), id)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, photos.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	defer func() { _ = obj.Body.Close() }()

	if obj.ContentType != "" {
		w.Header().Set("Content-Type", obj.ContentType)
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, obj.Body)
}

func (h *PhotosHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	day := resolveDay(r)
	back := dayPath("/diario", day)
	id, ok := parseInt64(r.PathValue("id"))
	if !ok {
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	if err := h.lib.Delete(r.Context(), currentUserID(r), id); err != nil && !errors.Is(err, store.ErrNotFound) {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	flash.Set(w, "notice", h.t(r, "diario.photo_deleted"), h.cfg.CookieSecure())
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (h *PhotosHandler) photoError(r *http.Request, err error) string {
	switch {
	case errors.Is(err, photos.ErrTooLarge):
		return h.t(r, "diario.photo_too_large")
	case errors.Is(err, photos.ErrTooMany):
		return h.t(r, "diario.photo_too_many")
	case errors.Is(err, photos.ErrUnsupportedType):
		return h.t(r, "diario.photo_invalid")
	default:
		return h.t(r, "diario.photo_invalid")
	}
}
