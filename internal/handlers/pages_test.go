package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	appi18n "github.com/puppe1990/vitalis/internal/i18n"
	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/store"
)

// testEnv is one wired app: store, views and a signed-in user.
type testEnv struct {
	store  store.Store
	views  *view.Renderer
	userID int64
	foodID int64
}

// newTestEnv seeds exactly what the pages need: one food, one entry on today
// and one weigh-in, so every conditional branch in the templates is exercised.
func newTestEnv(t *testing.T) testEnv {
	t.Helper()

	s := setupTestStore(t)
	views := setupTestViews(t)

	userID, err := s.CreateUser("pages@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SeedFoods(store.StarterFoods()); err != nil {
		t.Fatal(err)
	}
	foods, err := s.ListFoods("", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(foods) == 0 {
		t.Fatal("starter catalog is empty")
	}
	food := foods[0]

	today := resolveDay(httptest.NewRequest(http.MethodGet, "/", nil))
	if _, err := s.CreateEntry(models.Entry{
		UserID:    userID,
		FoodID:    food.ID,
		Meal:      nutrition.MealLunch,
		LoggedOn:  today,
		Grams:     150,
		Nutrients: nutrition.Scale(food.Per100g, 150),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddWater(userID, today, 500); err != nil {
		t.Fatal(err)
	}
	// A weigh-in matters: it is what made `.WeightDays` (an int) reach a
	// float64-only formatter and 500 the progress page in production.
	if err := s.SaveWeight(userID, today, 68.4); err != nil {
		t.Fatal(err)
	}

	return testEnv{store: s, views: views, userID: userID, foodID: food.ID}
}

// TestPages_rendersEveryPageWithRealData is the net that the unit tests left
// open: a view-data type mismatch only fails at render time, so the only test
// that catches it is one that renders the page.
func TestPages_rendersEveryPageWithRealData(t *testing.T) {
	env := newTestEnv(t)

	// Both catalogs run: the test suite boots English, production boots pt, and
	// a key that only exists in one of them would otherwise ship unchecked.
	for _, locale := range []string{"en", "pt"} {
		t.Run(locale, func(t *testing.T) {
			renderEveryPage(t, env, locale)
		})
	}
}

func renderEveryPage(t *testing.T, env testEnv, locale string) {
	t.Helper()
	h := newPageHandlers(t, env, locale)

	cases := []struct {
		name   string
		handle http.HandlerFunc
		path   string
		want   string
	}{
		// The marker is a form field or class the template always emits, so the
		// assertion does not depend on which locale catalog the test boots.
		{"diario", h.diary.ServeHTTP, "/diario", "vitalis-bar"},
		{"alimentos", h.foods.Browse, "/alimentos", `id="food-search"`},
		{"alimento", h.foods.Show, "/alimento/1", `name="grams"`},
		{"alimento_novo", h.foods.New, "/alimentos/novo", `name="kcal_100g"`},
		{"progresso", h.progress.ServeHTTP, "/progresso", "weight-fill"},
		{"metas", h.goals.Show, "/metas", `name="age_years"`},
		{"calculadora", h.calc.Show, "/calculadora", `name="weight_kg"`},
		{"api_tokens", h.tokens.Show, "/settings/api", `name="label"`},
		{"settings", h.auth.Settings, "/settings", `action="/logout"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := signedIn(t, env, http.MethodGet, tc.path)
			rr := httptest.NewRecorder()
			tc.handle(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200. body: %s", rr.Code, firstLines(rr.Body.String()))
			}
			if !strings.Contains(rr.Body.String(), tc.want) {
				t.Errorf("body missing %q", tc.want)
			}
			body := rr.Body.String()
			if strings.Contains(body, "internal server error") {
				t.Error("page rendered the framework error page")
			}
			// A bad format verb renders `%!d(string=…)` with no error and no
			// 500 — the page looks fine until someone reads that line.
			if strings.Contains(body, "%!") {
				t.Errorf("page has a format-verb mismatch: %s", formatMismatch(body))
			}
		})
	}
}

// Production boots with LOCALE=pt, so the Portuguese catalog must resolve on a
// page whose copy is entirely in the catalog.
func TestPages_rendersPortugueseCopy(t *testing.T) {
	env := newTestEnv(t)
	catalog := appi18n.NewCatalog("pt")
	// The renderer picks the catalog from the request locale; without loading
	// the pt set it would fall back to the English boot catalog and the
	// Portuguese copy would never be exercised.
	ptViews := setupTestViewsFor(t, "pt")
	h := NewDiaryHandler(ptViews, env.store, testSite(), catalog, cais.Config{}, nil)

	req := signedIn(t, env, http.MethodGet, "/diario")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"Ritmo Calórico", "Macronutrientes", "Hidratação", "Refeições do Dia"} {
		if !strings.Contains(body, want) {
			t.Errorf("pt body missing %q", want)
		}
	}
}

// The detail page reads the id from the path pattern.
func TestPages_alimentoUsesPathValue(t *testing.T) {
	env := newTestEnv(t)
	h := newPageHandlers(t, env, "pt")

	req := signedIn(t, env, http.MethodGet, "/alimento/1")
	rr := httptest.NewRecorder()
	h.foods.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

type pageHandlers struct {
	auth     *AuthHandler
	diary    *DiaryHandler
	foods    *FoodsHandler
	progress *ProgressHandler
	goals    *GoalsHandler
	calc     *CalculatorHandler
	tokens   *APITokensPage
}

func newPageHandlers(t *testing.T, env testEnv, locale string) pageHandlers {
	t.Helper()
	catalog := appi18n.NewCatalog(locale)
	views := setupTestViewsFor(t, locale)
	cfg := cais.Config{}
	return pageHandlers{
		auth:     NewAuthHandler(views, env.store, testSite(), env.store.Sessions(), cfg, catalog),
		diary:    NewDiaryHandler(views, env.store, testSite(), catalog, cfg, nil),
		foods:    NewFoodsHandler(views, env.store, testSite(), catalog, cfg),
		progress: NewProgressHandler(views, env.store, testSite(), catalog, cfg),
		goals:    NewGoalsHandler(views, env.store, testSite(), catalog, cfg),
		calc:     NewCalculatorHandler(views, testSite(), catalog, cfg),
		tokens:   NewAPITokensPage(views, env.store, testSite(), catalog, cfg),
	}
}

// signedIn builds a request carrying the session user and the path values the
// router would normally have set.
func signedIn(t *testing.T, env testEnv, method, target string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	req = session.WithUserID(req, env.userID)
	if strings.HasPrefix(target, "/alimento/") && !strings.HasSuffix(target, "/novo") {
		req.SetPathValue("id", "1")
	}
	return req
}

// setupTestViewsFor loads the template set with locale as the boot catalog plus
// both shipped catalogs, mirroring what cmd/server does.
func setupTestViewsFor(t *testing.T, locale string) *view.Renderer {
	t.Helper()
	root := projectRoot(t)
	fsys := os.DirFS(filepath.Join(root, "web", "templates"))
	r, err := view.Load(fsys,
		appi18n.NewCatalog(locale),
		appi18n.NewCatalog("en"),
		appi18n.NewCatalog("pt"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// formatMismatch extracts the first `%!…` fragment so a failure names the verb
// instead of dumping the whole page.
func formatMismatch(body string) string {
	i := strings.Index(body, "%!")
	if i < 0 {
		return ""
	}
	end := i + 40
	if end > len(body) {
		end = len(body)
	}
	return body[i:end]
}

func firstLines(body string) string {
	lines := strings.SplitN(strings.TrimSpace(body), "\n", 6)
	return strings.Join(lines, "\n")
}

// A user must be able to leave the app. The layout has no sign-out control, so
// the account page is the only place that offers it — and it did not before.
func TestSettings_offersSignOut(t *testing.T) {
	env := newTestEnv(t)
	h := newPageHandlers(t, env, "pt")

	req := signedIn(t, env, http.MethodGet, "/settings")
	rr := httptest.NewRecorder()
	h.auth.Settings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `action="/logout"`) {
		t.Error("settings page has no sign-out form")
	}
	if !strings.Contains(body, `method="post"`) {
		t.Error("sign out must be a POST")
	}
}

// The password form lives on /settings; it has to be one tap from the tab bar,
// not buried behind the avatar → metas path.
func TestLayout_footerLinksToSettingsAndCentersFAB(t *testing.T) {
	env := newTestEnv(t)
	h := newPageHandlers(t, env, "pt")

	req := signedIn(t, env, http.MethodGet, "/diario")
	rr := httptest.NewRecorder()
	h.diary.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}

	navAt := strings.Index(rr.Body.String(), `amarra-hook="nav"`)
	if navAt < 0 {
		t.Fatal("footer nav missing")
	}
	nav := rr.Body.String()[navAt:]
	if !strings.Contains(nav, `href="/settings"`) {
		t.Error("footer missing /settings")
	}
	if !strings.Contains(nav, "Ajustes") {
		t.Error("footer missing settings label")
	}

	fab := strings.Index(nav, `aria-label="Registrar"`)
	if fab < 0 {
		t.Fatal("registrar FAB missing")
	}
	open := strings.LastIndex(nav[:fab], "<a ")
	if open < 0 {
		t.Fatal("registrar <a> missing")
	}
	wrap := strings.LastIndex(nav[:open], "<div")
	if wrap < 0 {
		t.Fatal("FAB wrapper missing")
	}
	slot := nav[wrap:open]
	end := strings.Index(slot, ">")
	if end < 0 {
		t.Fatal("FAB wrapper unclosed")
	}
	class := slot[:end]
	if !strings.Contains(class, "flex-1") || !strings.Contains(class, "justify-center") {
		t.Error("registrar FAB is not in a centered flex-1 slot")
	}
}

// The account page must be reachable from the goal page, otherwise a user can
// never reach the password form or the sign-out button.
func TestMetas_linksToAccountPage(t *testing.T) {
	env := newTestEnv(t)
	h := newPageHandlers(t, env, "pt")

	req := signedIn(t, env, http.MethodGet, "/metas")
	rr := httptest.NewRecorder()
	h.goals.Show(rr, req)

	if !strings.Contains(rr.Body.String(), `href="/settings"`) {
		t.Error("metas page does not link to /settings")
	}
}

// An untouched week has no peak day; the card must say so instead of printing
// "Pico da semana: (0 kcal)" with an empty label.
func TestProgresso_emptyWeekHasNoPeakRow(t *testing.T) {
	env := newTestEnv(t)
	h := newPageHandlers(t, env, "pt")

	// Clear the seeded entry so the week has nothing.
	today := resolveDay(httptest.NewRequest(http.MethodGet, "/", nil))
	entries, err := env.store.ListEntries(env.userID, today)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if err := env.store.DeleteEntry(env.userID, e.ID); err != nil {
			t.Fatal(err)
		}
	}

	req := signedIn(t, env, http.MethodGet, "/progresso")
	rr := httptest.NewRecorder()
	h.progress.ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, "Pico da semana: (") {
		t.Error("empty week still prints an empty peak label")
	}
	if !strings.Contains(body, "Nenhuma refeição registrada nesta semana") {
		t.Error("empty week has no explanatory copy")
	}
}

// The goal form must offer the pinned target, and the saved value must win
// over the formula when the user posts it.
func TestGoals_manualTargetWinsOverTheFormula(t *testing.T) {
	env := newTestEnv(t)
	h := newPageHandlers(t, env, "pt")

	form := url.Values{
		"sex":         {"male"},
		"age_years":   {"34"},
		"height_cm":   {"180"},
		"weight_kg":   {"93.2"},
		"activity":    {"moderate"},
		"direction":   {"lose"},
		"target_kcal": {"2500"},
	}
	req := httptest.NewRequest(http.MethodPost, "/metas", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = session.WithUserID(req, env.userID)
	rr := httptest.NewRecorder()
	h.goals.Save(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body %s)", rr.Code, firstLines(rr.Body.String()))
	}

	goal, err := env.store.GetGoal(env.userID)
	if err != nil {
		t.Fatal(err)
	}
	if goal.TargetKcal != 2500 {
		t.Errorf("TargetKcal = %v, want the pinned 2500", goal.TargetKcal)
	}
	if goal.TargetOverride != 2500 {
		t.Errorf("TargetOverride = %v, want 2500", goal.TargetOverride)
	}
}

func TestGoals_formRendersTargetField(t *testing.T) {
	env := newTestEnv(t)
	h := newPageHandlers(t, env, "pt")

	req := signedIn(t, env, http.MethodGet, "/metas")
	rr := httptest.NewRecorder()
	h.goals.Show(rr, req)

	if !strings.Contains(rr.Body.String(), `name="target_kcal"`) {
		t.Error("goal form has no pinned-target field")
	}
}
