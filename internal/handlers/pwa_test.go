package handlers

import (
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func testPWAAssets(t *testing.T) PWAAssets {
	t.Helper()
	return PWAAssets{Dir: filepath.Join(projectRoot(t), "web", "static")}
}

func TestPWAAssets_manifestIsManifestJSON(t *testing.T) {
	h := testPWAAssets(t)
	req := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
	rr := httptest.NewRecorder()
	h.Manifest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/manifest+json") {
		t.Errorf("Content-Type = %q, want application/manifest+json", ct)
	}
	if !strings.Contains(rr.Body.String(), `"name": "Vitalis"`) {
		t.Error("body is not the Vitalis manifest")
	}
}

func TestPWAAssets_appleTouchIsThe180Drop(t *testing.T) {
	h := testPWAAssets(t)
	req := httptest.NewRequest(http.MethodGet, "/apple-touch-icon.png", nil)
	rr := httptest.NewRecorder()
	h.AppleTouch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "image/png") {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	img, err := png.Decode(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != 180 || b.Dy() != 180 {
		t.Fatalf("size = %dx%d, want 180x180", b.Dx(), b.Dy())
	}
}
