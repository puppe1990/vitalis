package handlers

import (
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/vitalis/internal/store"
)

// APITokensPage is the settings page where a user mints the bearer token the
// MCP server uses. The plaintext appears exactly once, on the POST response.
type APITokensPage struct {
	views   *view.Renderer
	store   store.Store
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
}

func NewAPITokensPage(views *view.Renderer, s store.Store, site meta.Site, catalog *i18n.Catalog, cfg cais.Config) *APITokensPage {
	return &APITokensPage{views: views, store: s, site: site, catalog: catalog, cfg: cfg}
}

func (h *APITokensPage) t(r *http.Request, key string, args ...any) string {
	return translate(r, h.catalog, key, args...)
}

func (h *APITokensPage) Show(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "", "")
}

// Create mints a token and re-renders the page with the plaintext visible.
// It is deliberately not a redirect: a flash cookie would put the secret in a
// cookie, and a 303 would lose it before the user can copy it.
func (h *APITokensPage) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulário inválido", http.StatusBadRequest)
		return
	}
	label := r.PostFormValue("label")
	if label == "" {
		label = h.t(r, "settings.api_default_label")
	}

	plain, err := h.store.CreateAPIToken(currentUserID(r), label)
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	h.render(w, r, plain, h.t(r, "settings.api_created"))
}

func (h *APITokensPage) Revoke(w http.ResponseWriter, r *http.Request) {
	id, ok := parseInt64(r.PathValue("id"))
	if !ok {
		http.Redirect(w, r, "/settings/api", http.StatusSeeOther)
		return
	}
	if err := h.store.RevokeAPIToken(currentUserID(r), id); err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	flash.Set(w, "notice", h.t(r, "settings.api_revoked"), h.cfg.CookieSecure())
	http.Redirect(w, r, "/settings/api", http.StatusSeeOther)
}

func (h *APITokensPage) render(w http.ResponseWriter, r *http.Request, plain, notice string) {
	tokens, err := h.store.ListAPITokens(currentUserID(r))
	if err != nil {
		httpxServerError(w, r, err, h.cfg)
		return
	}
	writeView(w, r, h.views, h.cfg, "app", "api_tokens", amarraData(r, h.site, map[string]any{
		"Title":     h.t(r, "settings.api_title"),
		"ActiveNav": "metas",
		"Tokens":    tokens,
		"Plain":     plain,
		"Notice":    notice,
		"Empty":     len(tokens) == 0,
	}), 0)
}
