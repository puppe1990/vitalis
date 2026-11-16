package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puppe1990/vitalis/internal/photos"
)

func jpegAPIBody() []byte {
	return []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
}

func newTestAPIWithPhotos(t *testing.T) (http.Handler, string, int64, *photos.Library) {
	t.Helper()
	_, token, userID, s := newTestAPI(t)
	lib := photos.New(s, photos.NewMemoryBucket())
	return NewHandler(s, lib), token, userID, lib
}

func TestHandler_uploadAndFetchPhoto(t *testing.T) {
	h, token, _, _ := newTestAPIWithPhotos(t)

	rr := doMultipart(t, h, "/api/v1/photos?d=2026-11-16", token, "almoco", jpegAPIBody(), "prato.jpg")
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var created struct {
		Photo struct {
			ID          int64  `json:"id"`
			Meal        string `json:"meal"`
			ContentType string `json:"content_type"`
		} `json:"photo"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Photo.ID == 0 || created.Photo.Meal != "almoco" {
		t.Errorf("photo = %+v", created.Photo)
	}

	get := doJSON(t, h, http.MethodGet, "/api/v1/photos/"+itoa(created.Photo.ID), token, nil)
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d", get.Code)
	}
	if get.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Content-Type = %q", get.Header().Get("Content-Type"))
	}
	if !bytes.Equal(get.Body.Bytes(), jpegAPIBody()) {
		t.Error("fetched bytes differ")
	}

	diary := doJSON(t, h, http.MethodGet, "/api/v1/diary?d=2026-11-16", token, nil)
	if diary.Code != http.StatusOK {
		t.Fatalf("diary status = %d", diary.Code)
	}
	if !bytes.Contains(diary.Body.Bytes(), []byte(`"id":`+itoa(created.Photo.ID))) {
		t.Errorf("diary missing photo id: %s", diary.Body.String())
	}
}

func TestHandler_uploadPhotoRequiresStorage(t *testing.T) {
	h, token, _, _ := newTestAPI(t)
	rr := doMultipart(t, h, "/api/v1/photos", token, "almoco", jpegAPIBody(), "prato.jpg")
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
}

func TestHandler_deletePhoto(t *testing.T) {
	h, token, _, _ := newTestAPIWithPhotos(t)
	created := doMultipart(t, h, "/api/v1/photos?d=2026-11-16", token, "almoco", jpegAPIBody(), "prato.jpg")
	var payload struct {
		Photo struct {
			ID int64 `json:"id"`
		} `json:"photo"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}

	del := doJSON(t, h, http.MethodDelete, "/api/v1/photos/"+itoa(payload.Photo.ID), token, nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", del.Code, del.Body.String())
	}

	get := doJSON(t, h, http.MethodGet, "/api/v1/photos/"+itoa(payload.Photo.ID), token, nil)
	if get.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", get.Code)
	}
}

func doMultipart(t *testing.T, h http.Handler, path, token, meal string, body []byte, filename string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("meal", meal); err != nil {
		t.Fatal(err)
	}
	part, err := mw.CreateFormFile("photo", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}
