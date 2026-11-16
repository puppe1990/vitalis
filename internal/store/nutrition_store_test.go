package store

import (
	"errors"
	"testing"
	"time"

	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
	"github.com/puppe1990/vitalis/internal/testdata"
)

func newTestUser(t *testing.T, s Store) int64 {
	t.Helper()
	id, err := s.CreateUser("user@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func chicken() models.Food {
	return models.Food{
		Name:         "Peito de frango grelhado",
		Category:     "Proteínas",
		Per100g:      nutrition.Nutrients{Kcal: 165, Protein: 31, Carb: 0, Fat: 3.6},
		PortionLabel: "1 filé médio",
		PortionGrams: 150,
		Verified:     true,
	}
}

func TestCreateFood_roundTripsEveryField(t *testing.T) {
	s := newTestStore(t)
	f := chicken()

	id, err := s.CreateFood(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.FindFood(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != f.Name || got.Category != f.Category {
		t.Errorf("text fields drifted: %+v", got)
	}
	if got.Per100g.Kcal != 165 || got.Per100g.Protein != 31 || got.Per100g.Fat != 3.6 {
		t.Errorf("nutrients drifted: %+v", got.Per100g)
	}
	if !got.Verified {
		t.Error("verified flag lost")
	}
}

func TestCreateFood_isIdempotentOnName(t *testing.T) {
	s := newTestStore(t)
	first, err := s.CreateFood(chicken())
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateFood(chicken())
	if err != nil {
		t.Fatalf("re-create must not error: %v", err)
	}
	if first != second {
		t.Errorf("re-create produced a new row: %d vs %d", first, second)
	}
}

func TestFindFood_missingReturnsError(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.FindFood(999); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindFood(999) err = %v, want ErrNotFound", err)
	}
}

func TestListFoods_searchAndCategory(t *testing.T) {
	s := newTestStore(t)
	mustFood(t, s, "Arroz branco cozido", "Cereais")
	mustFood(t, s, "Peito de frango grelhado", "Proteínas")
	mustFood(t, s, "Salmão grelhado", "Proteínas")

	all, err := s.ListFoods("", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("ListFoods(all) = %d, want 3", len(all))
	}

	searched, err := s.ListFoods("grelhado", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(searched) != 2 {
		t.Errorf("ListFoods(grelhado) = %d, want 2", len(searched))
	}

	proteins, err := s.ListFoods("", "Proteínas", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(proteins) != 2 {
		t.Errorf("ListFoods(Proteínas) = %d, want 2", len(proteins))
	}
}

func TestListFoods_respectsLimit(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"A", "B", "C", "D"} {
		mustFood(t, s, name, "Cereais")
	}
	got, err := s.ListFoods("", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("limit ignored: got %d", len(got))
	}
}

func TestListCategories_areDistinct(t *testing.T) {
	s := newTestStore(t)
	mustFood(t, s, "A", "Cereais")
	mustFood(t, s, "B", "Cereais")
	mustFood(t, s, "C", "Proteínas")

	got, err := s.ListCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("ListCategories = %v, want 2 distinct", got)
	}
}

func TestUpsertGoal_keepsOneRowPerUser(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)

	if err := s.UpsertGoal(models.Goal{UserID: userID, TargetKcal: 2000, ProteinG: 150}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertGoal(models.Goal{UserID: userID, TargetKcal: 2100, ProteinG: 160}); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetGoal(userID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetKcal != 2100 || got.ProteinG != 160 {
		t.Errorf("upsert kept stale values: %+v", got)
	}
}

func TestGetGoal_missingReturnsDefaults(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)

	got, err := s.GetGoal(userID)
	if err != nil {
		t.Fatalf("missing goal must not error: %v", err)
	}
	if got.TargetKcal != nutrition.DefaultTargetKcal {
		t.Errorf("TargetKcal = %v, want default", got.TargetKcal)
	}
	if got.WaterML <= 0 {
		t.Errorf("WaterML = %v, want a usable default", got.WaterML)
	}
}

func TestCreateEntry_storesScaledNutrients(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)
	foodID, err := s.CreateFood(chicken())
	if err != nil {
		t.Fatal(err)
	}
	day := testdata.Day(2026, 11, 16)

	portion := nutrition.Scale(chicken().Per100g, 150)
	id, err := s.CreateEntry(models.Entry{
		UserID:    userID,
		FoodID:    foodID,
		Meal:      nutrition.MealLunch,
		LoggedOn:  day,
		Grams:     150,
		Nutrients: portion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("CreateEntry returned id 0")
	}

	entries, err := s.ListEntries(userID, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("ListEntries = %d, want 1", len(entries))
	}
	if got := entries[0]; got.Nutrients.Kcal < 247 || got.Nutrients.Kcal > 249 {
		t.Errorf("stored kcal = %v, want ~247.5", got.Nutrients.Kcal)
	}
	if entries[0].FoodName == "" {
		t.Error("FoodName not joined for rendering")
	}
}

func TestListEntries_isScopedToUserAndDay(t *testing.T) {
	s := newTestStore(t)
	userA := newTestUser(t, s)
	userB, err := s.CreateUser("other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	foodID, err := s.CreateFood(chicken())
	if err != nil {
		t.Fatal(err)
	}

	day := testdata.Day(2026, 11, 16)
	mustEntry(t, s, userA, foodID, day)
	mustEntry(t, s, userB, foodID, day)
	mustEntry(t, s, userA, foodID, testdata.Day(2026, 11, 17))

	got, err := s.ListEntries(userA, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("ListEntries leaked across user/day: got %d", len(got))
	}
}

func TestDeleteEntry_removesOnlyOwnRow(t *testing.T) {
	s := newTestStore(t)
	userA := newTestUser(t, s)
	userB, err := s.CreateUser("other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	foodID, err := s.CreateFood(chicken())
	if err != nil {
		t.Fatal(err)
	}
	day := testdata.Day(2026, 11, 16)

	entryID := mustEntry(t, s, userA, foodID, day)

	if err := s.DeleteEntry(userB, entryID); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-user delete err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteEntry(userA, entryID); err != nil {
		t.Fatalf("own delete failed: %v", err)
	}
	if got, _ := s.ListEntries(userA, day); len(got) != 0 {
		t.Errorf("entry survived delete: %+v", got)
	}
}

func TestAddWater_accumulatesAndClamps(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)
	day := testdata.Day(2026, 11, 16)

	if err := s.AddWater(userID, day, 250); err != nil {
		t.Fatal(err)
	}
	if err := s.AddWater(userID, day, 250); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetDayLog(userID, day)
	if err != nil {
		t.Fatal(err)
	}
	if got.WaterML != 500 {
		t.Errorf("WaterML = %v, want 500", got.WaterML)
	}

	// Days are independent.
	other, err := s.GetDayLog(userID, testdata.Day(2026, 11, 17))
	if err != nil {
		t.Fatal(err)
	}
	if other.WaterML != 0 {
		t.Errorf("water leaked into another day: %v", other.WaterML)
	}
}

func TestAddWater_neverGoesNegative(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)
	day := testdata.Day(2026, 11, 16)

	if err := s.AddWater(userID, day, 250); err != nil {
		t.Fatal(err)
	}
	if err := s.AddWater(userID, day, -1000); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetDayLog(userID, day)
	if err != nil {
		t.Fatal(err)
	}
	if got.WaterML < 0 {
		t.Errorf("WaterML = %v, want clamped at 0", got.WaterML)
	}
}

func TestUpsertDayLog_updatesBurnAndWeight(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)
	day := testdata.Day(2026, 11, 16)

	if err := s.SaveDayLog(models.DayLog{UserID: userID, LoggedOn: day, BurnedKcal: 340, WeightKG: 68.4}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetDayLog(userID, day)
	if err != nil {
		t.Fatal(err)
	}
	if got.BurnedKcal != 340 || got.WeightKG != 68.4 {
		t.Errorf("day log = %+v", got)
	}
}

func TestWeight_upsertsSameDay(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)
	day := testdata.Day(2026, 11, 16)

	if err := s.SaveWeight(userID, day, 70.2); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveWeight(userID, day, 69.8); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListWeights(userID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("weights = %d, want 1 (upserted)", len(got))
	}
	if got[0].KG != 69.8 {
		t.Errorf("KG = %v, want latest 69.8", got[0].KG)
	}
}

func TestListWeights_newestFirst(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)

	for _, d := range []struct {
		day int
		kg  float64
	}{{10, 71}, {12, 70}, {14, 69}} {
		if err := s.SaveWeight(userID, testdata.Day(2026, 11, d.day), d.kg); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.ListWeights(userID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("weights = %d, want 3", len(got))
	}
	if got[0].KG != 69 {
		t.Errorf("first row = %v, want newest (69)", got[0].KG)
	}
}

func TestAPIToken_authenticatesRoundTrip(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)

	plain, err := s.CreateAPIToken(userID, "MCP local")
	if err != nil {
		t.Fatal(err)
	}
	if plain == "" {
		t.Fatal("CreateAPIToken returned an empty token")
	}

	gotUser, ok := s.FindUserIDByAPIToken(plain)
	if !ok {
		t.Fatal("token did not authenticate")
	}
	if gotUser != userID {
		t.Errorf("token resolved to user %d, want %d", gotUser, userID)
	}
}

func TestAPIToken_rejectsUnknownAndEmpty(t *testing.T) {
	s := newTestStore(t)

	if _, ok := s.FindUserIDByAPIToken("not-a-real-token"); ok {
		t.Error("unknown token authenticated")
	}
	if _, ok := s.FindUserIDByAPIToken(""); ok {
		t.Error("empty token authenticated")
	}
}

func TestAPIToken_doesNotStorePlaintext(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)

	plain, err := s.CreateAPIToken(userID, "MCP")
	if err != nil {
		t.Fatal(err)
	}

	var count int
	err = s.DB().QueryRow("SELECT COUNT(*) FROM api_tokens WHERE token_hash = ?", plain).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Error("plaintext token stored in the database")
	}
}

func TestAPIToken_listAndRevoke(t *testing.T) {
	s := newTestStore(t)
	userID := newTestUser(t, s)

	if _, err := s.CreateAPIToken(userID, "first"); err != nil {
		t.Fatal(err)
	}
	tokens, err := s.ListAPITokens(userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 {
		t.Fatalf("tokens = %d, want 1", len(tokens))
	}

	if err := s.RevokeAPIToken(userID, tokens[0].ID); err != nil {
		t.Fatal(err)
	}
	after, err := s.ListAPITokens(userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Errorf("token survived revoke: %d", len(after))
	}
}

func TestSeedDemoFoods_isIdempotent(t *testing.T) {
	s := newTestStore(t)

	if err := s.SeedFoods(StarterFoods()); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedFoods(StarterFoods()); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListFoods("", "", 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(StarterFoods()) {
		t.Errorf("items = %d, want %d after two seeds", len(got), len(StarterFoods()))
	}
}

func mustFood(t *testing.T, s Store, name, category string) int64 {
	t.Helper()
	id, err := s.CreateFood(models.Food{
		Name:     name,
		Category: category,
		Per100g:  nutrition.Nutrients{Kcal: 100, Protein: 5, Carb: 10, Fat: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustEntry(t *testing.T, s Store, userID, foodID int64, day time.Time) int64 {
	t.Helper()
	id, err := s.CreateEntry(models.Entry{
		UserID:    userID,
		FoodID:    foodID,
		Meal:      nutrition.MealLunch,
		LoggedOn:  day,
		Grams:     100,
		Nutrients: nutrition.Nutrients{Kcal: 165, Protein: 31, Fat: 3.6},
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
