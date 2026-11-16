package app

import (
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/amarra/live"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/middleware"

	"github.com/puppe1990/vitalis/internal/api"
	"github.com/puppe1990/vitalis/internal/handlers"
)

func registerRoutes(r *cais.Router, deps Deps, cfg cais.Config) {
	home := handlers.NewHomeHandler(deps.Views, deps.Site, deps.Catalog, cfg)
	calc := handlers.NewCalculatorHandler(deps.Views, deps.Site, deps.Catalog, cfg)
	auth := handlers.NewAuthHandler(deps.Views, deps.Store, deps.Site, deps.Store.Sessions(), cfg, deps.Catalog)
	diary := handlers.NewDiaryHandler(deps.Views, deps.Store, deps.Site, deps.Catalog, cfg, deps.Photos)
	mealPhotos := handlers.NewPhotosHandler(deps.Photos, deps.Catalog, cfg)
	foods := handlers.NewFoodsHandler(deps.Views, deps.Store, deps.Site, deps.Catalog, cfg)
	progress := handlers.NewProgressHandler(deps.Views, deps.Store, deps.Site, deps.Catalog, cfg)
	goals := handlers.NewGoalsHandler(deps.Views, deps.Store, deps.Site, deps.Catalog, cfg)
	tokens := handlers.NewAPITokensPage(deps.Views, deps.Store, deps.Site, deps.Catalog, cfg)

	loginLimit := middleware.NewRateLimiter(10, cfg)
	resetLimit := middleware.NewRateLimiter(10, cfg)

	pwa := handlers.PWAAssets{Dir: deps.StaticDir}
	r.Get("/manifest.webmanifest", pwa.Manifest)
	r.Get("/apple-touch-icon.png", pwa.AppleTouch)
	r.Get("/apple-touch-icon-precomposed.png", pwa.AppleTouch)
	r.Get("/favicon.ico", pwa.FaviconICO)

	r.Get("/", home.ServeHTTP)
	r.Get("/calculadora", calc.Show)
	r.Post("/calculadora", calc.Calculate)
	r.Get("/login", auth.Login)
	r.Post("/login", loginLimit.Middleware(http.HandlerFunc(auth.LoginPost)).ServeHTTP)
	r.Get("/signup", auth.SignUp)
	r.Post("/signup", loginLimit.Middleware(http.HandlerFunc(auth.SignUpPost)).ServeHTTP)
	r.Get("/forgot-password", auth.ForgotPassword)
	r.Post("/forgot-password", resetLimit.Middleware(http.HandlerFunc(auth.ForgotPasswordPost)).ServeHTTP)
	r.Get("/reset-password", auth.ResetPassword)
	r.Post("/reset-password", resetLimit.Middleware(http.HandlerFunc(auth.ResetPasswordPost)).ServeHTTP)
	r.Post("/logout", auth.LogoutPost)
	r.Post("/locale", handlers.PostLocale(cfg))
	r.Get("/settings", middleware.RequireAuthFunc("/login", auth.Settings))
	r.Post("/settings", resetLimit.Middleware(http.HandlerFunc(auth.SettingsPost)).ServeHTTP)

	// Diary is the app's home once signed in.
	r.Get("/diario", middleware.RequireAuthFunc("/login", diary.ServeHTTP))
	r.Post("/diario/agua", middleware.RequireAuthFunc("/login", diary.AddWaterPost))
	r.Post("/diario/entradas", middleware.RequireAuthFunc("/login", diary.AddEntryPost))
	r.Post("/alimentos/{id}/registrar", middleware.RequireAuthFunc("/login", diary.QuickAddPost))
	r.Post("/diario/entradas/{id}/remover", middleware.RequireAuthFunc("/login", diary.DeleteEntryPost))
	r.Post("/diario/fotos", middleware.RequireAuthFunc("/login", mealPhotos.UploadPost))
	r.Get("/diario/fotos/{id}", middleware.RequireAuthFunc("/login", mealPhotos.Serve))
	r.Post("/diario/fotos/{id}/remover", middleware.RequireAuthFunc("/login", mealPhotos.DeletePost))

	r.Get("/alimentos", middleware.RequireAuthFunc("/login", foods.Browse))
	r.Get("/alimentos/novo", middleware.RequireAuthFunc("/login", foods.New))
	r.Post("/alimentos", middleware.RequireAuthFunc("/login", foods.Create))
	r.Get("/alimento/{id}", middleware.RequireAuthFunc("/login", foods.Show))

	r.Get("/progresso", middleware.RequireAuthFunc("/login", progress.ServeHTTP))
	r.Get("/metas", middleware.RequireAuthFunc("/login", goals.Show))
	r.Post("/metas", middleware.RequireAuthFunc("/login", goals.Save))

	r.Get("/settings/api", middleware.RequireAuthFunc("/login", tokens.Show))
	r.Post("/settings/api", middleware.RequireAuthFunc("/login", tokens.Create))
	r.Post("/settings/api/{id}/revogar", middleware.RequireAuthFunc("/login", tokens.Revoke))

	// The MCP server's transport: bearer-token auth, no CSRF cookie. The token
	// is validated by the API handler itself, so the prefix is registered as a
	// bearer API for CSRF exemption rather than skipping the check globally.
	apiHandler := api.NewHandler(deps.Store, deps.Photos)
	r.Handle("GET "+api.Prefix+"/", apiHandler)
	r.Handle("POST "+api.Prefix+"/", apiHandler)
	r.Handle("DELETE "+api.Prefix+"/", apiHandler)
}

func registerLiveViews(hub *live.Hub, deps Deps) {
	_ = hub
	_ = deps
	// cais:live-views
}
