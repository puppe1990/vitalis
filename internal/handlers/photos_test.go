package handlers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	appi18n "github.com/puppe1990/vitalis/internal/i18n"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/photos"
)

func jpegBody() []byte {
	return []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
}

func TestDiary_hidesPhotoFormWhenDisabled(t *testing.T) {
	env := newTestEnv(t)
	h := NewDiaryHandler(env.views, env.store, testSite(), appi18n.NewCatalog("pt"), cais.Config{}, nil)

	req := signedIn(t, env, http.MethodGet, "/diario")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if strings.Contains(rr.Body.String(), `name="photo"`) {
		t.Error("photo form shown without B2")
	}
}

func TestDiary_showsPhotoFormWhenEnabled(t *testing.T) {
	env := newTestEnv(t)
	lib := photos.New(env.store, photos.NewMemoryBucket())
	h := NewDiaryHandler(env.views, env.store, testSite(), appi18n.NewCatalog("pt"), cais.Config{}, lib)

	req := signedIn(t, env, http.MethodGet, "/diario")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `name="photo"`) {
		t.Error("photo form missing")
	}
	if !strings.Contains(rr.Body.String(), `multiple`) {
		t.Error("photo input missing multiple")
	}
	if !strings.Contains(rr.Body.String(), `enctype="multipart/form-data"`) {
		t.Error("multipart form missing")
	}
}

func TestPhotos_uploadJPEGThenServe(t *testing.T) {
	env := newTestEnv(t)
	lib := photos.New(env.store, photos.NewMemoryBucket())
	h := NewPhotosHandler(lib, appi18n.NewCatalog("pt"), cais.Config{})

	rr := httptest.NewRecorder()
	h.UploadPost(rr, photoUploadRequest(t, env, "almoco", jpegBody(), "prato.jpg"))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("upload status = %d, want 303. body: %s", rr.Code, rr.Body.String())
	}

	listed, err := lib.List(env.userID, resolveDay(httptest.NewRequest(http.MethodGet, "/", nil)))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("saved photos = %d, want 1", len(listed))
	}
	if listed[0].Meal != nutrition.MealLunch {
		t.Errorf("meal = %q", listed[0].Meal)
	}

	req := signedIn(t, env, http.MethodGet, "/diario/fotos/"+itoa(listed[0].ID))
	req.SetPathValue("id", itoa(listed[0].ID))
	serve := httptest.NewRecorder()
	h.Serve(serve, req)
	if serve.Code != http.StatusOK {
		t.Fatalf("serve status = %d", serve.Code)
	}
	if serve.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Content-Type = %q", serve.Header().Get("Content-Type"))
	}
	if !bytes.Equal(serve.Body.Bytes(), jpegBody()) {
		t.Error("served bytes differ")
	}
}

func TestPhotos_uploadTwoJPEGsInOnePost(t *testing.T) {
	env := newTestEnv(t)
	lib := photos.New(env.store, photos.NewMemoryBucket())
	h := NewPhotosHandler(lib, appi18n.NewCatalog("pt"), cais.Config{})

	rr := httptest.NewRecorder()
	h.UploadPost(rr, photoUploadRequestFiles(t, env, "almoco", []struct {
		body     []byte
		filename string
	}{
		{jpegBody(), "prato1.jpg"},
		{jpegBody(), "prato2.jpg"},
	}))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("upload status = %d, want 303. body: %s", rr.Code, rr.Body.String())
	}

	listed, err := lib.List(env.userID, resolveDay(httptest.NewRequest(http.MethodGet, "/", nil)))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("saved photos = %d, want 2", len(listed))
	}
}

func TestPhotos_rejectsPDF(t *testing.T) {
	env := newTestEnv(t)
	lib := photos.New(env.store, photos.NewMemoryBucket())
	h := NewPhotosHandler(lib, appi18n.NewCatalog("pt"), cais.Config{})

	rr := httptest.NewRecorder()
	h.UploadPost(rr, photoUploadRequest(t, env, "almoco", []byte("%PDF-1.4"), "doc.pdf"))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	listed, err := lib.List(env.userID, resolveDay(httptest.NewRequest(http.MethodGet, "/", nil)))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Errorf("pdf was stored: %d photos", len(listed))
	}
}

func TestPhotos_otherUserCannotRead(t *testing.T) {
	env := newTestEnv(t)
	lib := photos.New(env.store, photos.NewMemoryBucket())
	_, err := lib.Save(context.Background(), photos.SaveInput{
		UserID:   env.userID,
		Meal:     nutrition.MealLunch,
		LoggedOn: resolveDay(httptest.NewRequest(http.MethodGet, "/", nil)),
		Body:     jpegBody(),
	})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := lib.List(env.userID, resolveDay(httptest.NewRequest(http.MethodGet, "/", nil)))
	if err != nil {
		t.Fatal(err)
	}

	stranger, err := env.store.CreateUser("stranger@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	h := NewPhotosHandler(lib, appi18n.NewCatalog("pt"), cais.Config{})
	req := httptest.NewRequest(http.MethodGet, "/diario/fotos/"+itoa(listed[0].ID), nil)
	req = session.WithUserID(req, stranger)
	req.SetPathValue("id", itoa(listed[0].ID))
	rr := httptest.NewRecorder()
	h.Serve(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestPhotos_deleteRemovesFile(t *testing.T) {
	env := newTestEnv(t)
	lib := photos.New(env.store, photos.NewMemoryBucket())
	day := resolveDay(httptest.NewRequest(http.MethodGet, "/", nil))
	photo, err := lib.Save(context.Background(), photos.SaveInput{
		UserID: env.userID, Meal: nutrition.MealLunch, LoggedOn: day, Body: jpegBody(),
	})
	if err != nil {
		t.Fatal(err)
	}

	h := NewPhotosHandler(lib, appi18n.NewCatalog("pt"), cais.Config{})
	req := signedIn(t, env, http.MethodPost, "/diario/fotos/"+itoa(photo.ID)+"/remover")
	req.SetPathValue("id", itoa(photo.ID))
	rr := httptest.NewRecorder()
	h.DeletePost(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	listed, err := lib.List(env.userID, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Errorf("photo still listed after delete")
	}
}

func TestDiary_rendersSavedPhoto(t *testing.T) {
	env := newTestEnv(t)
	lib := photos.New(env.store, photos.NewMemoryBucket())
	day := resolveDay(httptest.NewRequest(http.MethodGet, "/", nil))
	photo, err := lib.Save(context.Background(), photos.SaveInput{
		UserID: env.userID, Meal: nutrition.MealLunch, LoggedOn: day, Body: jpegBody(),
	})
	if err != nil {
		t.Fatal(err)
	}

	h := NewDiaryHandler(env.views, env.store, testSite(), appi18n.NewCatalog("pt"), cais.Config{}, lib)
	req := signedIn(t, env, http.MethodGet, "/diario")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	want := `/diario/fotos/` + itoa(photo.ID)
	if !strings.Contains(rr.Body.String(), want) {
		t.Errorf("diary missing photo url %s", want)
	}
}

func photoUploadRequest(t *testing.T, env testEnv, meal string, body []byte, filename string) *http.Request {
	t.Helper()
	return photoUploadRequestFiles(t, env, meal, []struct {
		body     []byte
		filename string
	}{{body, filename}})
}

func photoUploadRequestFiles(t *testing.T, env testEnv, meal string, files []struct {
	body     []byte
	filename string
}) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("meal", meal); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		part, err := mw.CreateFormFile("photo", f.filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(part, bytes.NewReader(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/diario/fotos", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return session.WithUserID(req, env.userID)
}
