package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	appi18n "github.com/puppe1990/vitalis/internal/i18n"
	"github.com/puppe1990/vitalis/internal/store"
)

func testPWAApp(t *testing.T) *App {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := wd
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("go.mod not found")
		}
		root = parent
	}

	s, err := store.NewSQLiteStore(":memory:", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	catalog := appi18n.DefaultCatalog()
	views, err := view.Load(os.DirFS(filepath.Join(root, "web", "templates")), catalog)
	if err != nil {
		t.Fatal(err)
	}

	a, err := New(cais.Config{Env: "test"}, Deps{
		Views:     views,
		Store:     s,
		StaticDir: filepath.Join(root, "web", "static"),
		Site:      meta.Site{AppName: "vitalis", AppURL: "https://cais.example.com"},
		Catalog:   catalog,
		Catalogs: map[string]*i18n.Catalog{
			"en": appi18n.NewCatalog("en"),
			"pt": appi18n.NewCatalog("pt"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestApp_servesRootPWAIcons(t *testing.T) {
	h := testPWAApp(t).Handler()

	manifest := httptest.NewRecorder()
	h.ServeHTTP(manifest, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))
	if manifest.Code != http.StatusOK {
		t.Fatalf("manifest status = %d", manifest.Code)
	}
	if ct := manifest.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/manifest+json") {
		t.Errorf("manifest Content-Type = %q", ct)
	}

	apple := httptest.NewRecorder()
	h.ServeHTTP(apple, httptest.NewRequest(http.MethodGet, "/apple-touch-icon.png", nil))
	if apple.Code != http.StatusOK {
		t.Fatalf("apple-touch-icon status = %d", apple.Code)
	}
	if ct := apple.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Errorf("apple-touch-icon Content-Type = %q", ct)
	}
}
