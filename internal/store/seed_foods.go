package store

import (
	"github.com/puppe1990/vitalis/internal/models"
	"github.com/puppe1990/vitalis/internal/nutrition"
)

// StarterFoods is the starter catalog. Values follow TACO (Tabela Brasileira de
// Composição de Alimentos) so the app ships with figures a nutritionist would
// recognize rather than invented ones. Nutrients are per 100 g.
func StarterFoods() []models.Food {
	return []models.Food{
		{
			Name: "Peito de frango grelhado", Category: "Proteínas",
			Per100g:      nutrition.Nutrients{Kcal: 165, Protein: 31, Carb: 0, Fat: 3.6},
			PortionLabel: "1 filé médio", PortionGrams: 150, Verified: true,
		},
		{
			Name: "Ovo de galinha cozido", Category: "Proteínas",
			Per100g:      nutrition.Nutrients{Kcal: 146, Protein: 13.3, Carb: 0.6, Fat: 9.5},
			PortionLabel: "1 unidade", PortionGrams: 50, Verified: true,
		},
		{
			Name: "Salmão grelhado", Category: "Proteínas",
			Per100g:      nutrition.Nutrients{Kcal: 208, Protein: 22, Carb: 0, Fat: 13},
			PortionLabel: "1 filé", PortionGrams: 130, Verified: true,
		},
		{
			Name: "Arroz branco cozido", Category: "Cereais",
			Per100g:      nutrition.Nutrients{Kcal: 130, Protein: 2.5, Carb: 28, Fat: 0.3},
			PortionLabel: "4 colheres de sopa", PortionGrams: 100, Verified: true,
		},
		{
			Name: "Feijão preto cozido", Category: "Cereais",
			Per100g:      nutrition.Nutrients{Kcal: 77, Protein: 4.5, Carb: 14, Fat: 0.5},
			PortionLabel: "1 concha", PortionGrams: 80, Verified: true,
		},
		{
			Name: "Aveia em flocos finos", Category: "Cereais",
			Per100g:      nutrition.Nutrients{Kcal: 350, Protein: 14, Carb: 57, Fat: 7.3},
			PortionLabel: "2 colheres de sopa", PortionGrams: 30, Verified: true,
		},
		{
			Name: "Pão integral", Category: "Cereais",
			Per100g:      nutrition.Nutrients{Kcal: 253, Protein: 9.4, Carb: 43, Fat: 3.7},
			PortionLabel: "2 fatias", PortionGrams: 50, Verified: true,
		},
		{
			Name: "Batata-doce assada", Category: "Cereais",
			Per100g:      nutrition.Nutrients{Kcal: 90, Protein: 2, Carb: 20.7, Fat: 0.1},
			PortionLabel: "1 unidade média", PortionGrams: 130, Verified: true,
		},
		{
			Name: "Banana prata", Category: "Frutas",
			Per100g:      nutrition.Nutrients{Kcal: 98, Protein: 1.3, Carb: 26, Fat: 0.1},
			PortionLabel: "1 unidade média", PortionGrams: 80, Verified: true,
		},
		{
			Name: "Maçã fuji", Category: "Frutas",
			Per100g:      nutrition.Nutrients{Kcal: 56, Protein: 0.3, Carb: 15.2, Fat: 0.2},
			PortionLabel: "1 unidade", PortionGrams: 130, Verified: true,
		},
		{
			Name: "Mamão papaia", Category: "Frutas",
			Per100g:      nutrition.Nutrients{Kcal: 40, Protein: 0.5, Carb: 10.4, Fat: 0.1},
			PortionLabel: "1 fatia", PortionGrams: 150, Verified: true,
		},
		{
			Name: "Iogurte natural desnatado", Category: "Laticínios",
			Per100g:      nutrition.Nutrients{Kcal: 42, Protein: 4, Carb: 5.4, Fat: 0.3},
			PortionLabel: "1 pote", PortionGrams: 170, Verified: true,
		},
		{
			Name: "Queijo minas frescal", Category: "Laticínios",
			Per100g:      nutrition.Nutrients{Kcal: 264, Protein: 17.4, Carb: 3.2, Fat: 20.2},
			PortionLabel: "1 fatia", PortionGrams: 30, Verified: true,
		},
		{
			Name: "Leite desnatado", Category: "Laticínios",
			Per100g:      nutrition.Nutrients{Kcal: 35, Protein: 3.4, Carb: 5, Fat: 0.2},
			PortionLabel: "1 copo", PortionGrams: 200, Verified: true,
		},
		{
			Name: "Brócolis cozido no vapor", Category: "Vegetais",
			Per100g:      nutrition.Nutrients{Kcal: 25, Protein: 2.1, Carb: 4.4, Fat: 0.5},
			PortionLabel: "1 xícara", PortionGrams: 90, Verified: true,
		},
		{
			Name: "Salada mista de folhas", Category: "Vegetais",
			Per100g:      nutrition.Nutrients{Kcal: 18, Protein: 1.4, Carb: 2.7, Fat: 0.2},
			PortionLabel: "1 prato", PortionGrams: 90, Verified: true,
		},
		{
			Name: "Azeite de oliva extravirgem", Category: "Gorduras",
			Per100g:      nutrition.Nutrients{Kcal: 884, Protein: 0, Carb: 0, Fat: 100},
			PortionLabel: "1 colher de sopa", PortionGrams: 13, Verified: true,
		},
		{
			Name: "Castanha-do-pará", Category: "Gorduras",
			Per100g:      nutrition.Nutrients{Kcal: 643, Protein: 14.5, Carb: 15.1, Fat: 63.4},
			PortionLabel: "2 unidades", PortionGrams: 10, Verified: true,
		},
		{
			Name: "Pasta de amendoim integral", Category: "Gorduras",
			Per100g:      nutrition.Nutrients{Kcal: 588, Protein: 25, Carb: 20, Fat: 50},
			PortionLabel: "1 colher de sopa", PortionGrams: 20, Verified: true,
		},
		{
			Name: "Whey protein concentrado", Category: "Proteínas",
			Per100g:      nutrition.Nutrients{Kcal: 380, Protein: 76, Carb: 8, Fat: 5},
			PortionLabel: "1 scoop", PortionGrams: 30, Verified: true,
		},
	}
}
