package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	appi18n "github.com/puppe1990/vitalis/internal/i18n"
)

func newHomeHandler(t *testing.T) *HomeHandler {
	t.Helper()
	return NewHomeHandler(setupTestViews(t), testSite(), appi18n.DefaultCatalog(), cais.Config{})
}

func TestHomeHandler_Returns200(t *testing.T) {
	h := newHomeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHomeHandler_RendersHTML(t *testing.T) {
	h := newHomeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `id="amarra-main"`) {
		t.Errorf("body missing #amarra-main, got: %s", body)
	}
	if !strings.Contains(body, "Vitalis") {
		t.Errorf("body missing brand, got: %s", body)
	}
	if strings.Contains(body, `data-amarra-outlet=`) {
		t.Errorf("home must not emit outlets; the handler opts in (#397), got: %s", body)
	}
}

// The landing page must show the product's own idea (the calorie ring), not the
// scaffold's "app is running" placeholder.
func TestHomeHandler_showsCalorieRingHero(t *testing.T) {
	h := newHomeHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `stroke-dasharray="301.6"`) {
		t.Error("landing page is missing the calorie ring")
	}
	if !strings.Contains(body, `data-testid="amarra-ready"`) {
		t.Error("missing amarra-ready marker")
	}
	if strings.Contains(body, "amarra-cais g resource") {
		t.Error("landing page still shows the scaffold generator snippet")
	}
}

func TestHomeHandler_ContentType(t *testing.T) {
	h := newHomeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// A signed-in user has no use for the landing page.
func TestHomeHandler_loggedInRedirectsToDiary(t *testing.T) {
	h := newHomeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = session.WithUserID(req, 1)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != afterAuthPath {
		t.Errorf("Location = %q, want %s", got, afterAuthPath)
	}
}

func TestHomeHandler_pwaIconLinksReachTheOS(t *testing.T) {
	h := newHomeHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	body := rr.Body.String()

	if !strings.Contains(body, `href="/apple-touch-icon.png"`) {
		t.Error("apple-touch-icon must be at /apple-touch-icon.png; iOS does not look under /static/")
	}
	if !strings.Contains(body, `href="/manifest.webmanifest"`) {
		t.Error("manifest must be at /manifest.webmanifest so Go can set application/manifest+json")
	}
	if strings.Contains(body, `href="/static/favicon.svg"`) && !strings.Contains(body, `favicon.svg?`) {
		t.Error("favicon URL must be cache-busted; /static/favicon.svg is cached for a year")
	}
}

// #208: the guest page must not offer chrome that bounces off the login gate.
func TestHomeHandler_anonymousHidesAppChrome(t *testing.T) {
	h := newHomeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, `action="/logout"`) {
		t.Error("guest page must not offer sign-out")
	}
	if strings.Contains(body, `href="/diario"`) {
		t.Error("guest page must not link to the diary")
	}
}
