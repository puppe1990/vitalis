package handlers

import (
	"net/http"
	"strconv"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"
)

// httpxServerError renders the framework's error page with the request's
// config. Wrapping it keeps handlers from repeating the call shape.
func httpxServerError(w http.ResponseWriter, r *http.Request, err error, cfg cais.Config) {
	httpx.ServerError(w, err, cfg)
}

// parseInt64 parses a form/path id. Returning ok keeps callers on the early
// return path instead of branching on a sentinel error.
func parseInt64(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func atoiDefault(raw string, fallback int) int {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func parseFloatDefault(raw string, fallback float64) float64 {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}
