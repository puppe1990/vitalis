package handlers

import (
	"net/http"
	"path/filepath"
)

// PWAAssets serves the files the OS looks up at the site root. Cleat's Python
// static layer owns /static/ and sends .webmanifest as text/plain; iOS also
// requests /apple-touch-icon.png without reading the HTML link.
type PWAAssets struct {
	Dir string
}

func (p PWAAssets) Manifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	http.ServeFile(w, r, filepath.Join(p.Dir, "manifest.webmanifest"))
}

func (p PWAAssets) AppleTouch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(p.Dir, "icons", "icon.png"))
}

func (p PWAAssets) FaviconICO(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(p.Dir, "icons", "icon-192.png"))
}
