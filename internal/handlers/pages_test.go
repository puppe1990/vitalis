package handlers

import (
	"net/http"
	"net/http/httptest"
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
	h := newPageHandlers(t, env)

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
		{"api_tokens", h.tokens.Show, "/settings/api", `name="label"`},
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
			if strings.Contains(rr.Body.String(), "internal server error") {
				t.Error("page rendered the framework error page")
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
	h := NewDiaryHandler(ptViews, env.store, testSite(), catalog, cais.Config{})

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
	h := newPageHandlers(t, env)

	req := signedIn(t, env, http.MethodGet, "/alimento/1")
	rr := httptest.NewRecorder()
	h.foods.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

type pageHandlers struct {
	diary    *DiaryHandler
	foods    *FoodsHandler
	progress *ProgressHandler
	goals    *GoalsHandler
	tokens   *APITokensPage
}

func newPageHandlers(t *testing.T, env testEnv) pageHandlers {
	t.Helper()
	catalog := appi18n.DefaultCatalog()
	cfg := cais.Config{}
	return pageHandlers{
		diary:    NewDiaryHandler(env.views, env.store, testSite(), catalog, cfg),
		foods:    NewFoodsHandler(env.views, env.store, testSite(), catalog, cfg),
		progress: NewProgressHandler(env.views, env.store, testSite(), catalog, cfg),
		goals:    NewGoalsHandler(env.views, env.store, testSite(), catalog, cfg),
		tokens:   NewAPITokensPage(env.views, env.store, testSite(), catalog, cfg),
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

func firstLines(body string) string {
	lines := strings.SplitN(strings.TrimSpace(body), "\n", 6)
	return strings.Join(lines, "\n")
}
