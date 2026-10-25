package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Static serves the compiled frontend from dir in front of api: /api and
// /api/... always go to api untouched, an existing regular file is served as
// is, and any other GET/HEAD falls back to index.html (SPA routing).
func Static(dir string, api http.Handler) http.Handler {
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		// Rooting the path before Clean drops any ".." so it cannot leave dir.
		clean := path.Clean("/" + r.URL.Path)
		if serveFile(w, r, filepath.Join(dir, filepath.FromSlash(clean)), clean) {
			return
		}
		// A missing file with an extension is a broken asset, not an SPA route.
		if path.Ext(clean) != "" {
			http.NotFound(w, r)
			return
		}
		if !serveFile(w, r, index, "/index.html") {
			http.Error(w, "frontend index.html not found", http.StatusInternalServerError)
		}
	})
}

// serveFile serves name when it is a regular file. It uses ServeContent, not
// ServeFile, which would redirect /index.html to ./. urlPath picks the
// Cache-Control: index.html must always revalidate, while Vite hashes the
// names under /assets/, so those never change.
// os.Open follows symlinks: dist is trusted build output (no os.OpenRoot).
func serveFile(w http.ResponseWriter, r *http.Request, name, urlPath string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	switch {
	case urlPath == "/index.html":
		w.Header().Set("Cache-Control", "no-cache")
	case strings.HasPrefix(urlPath, "/assets/"):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
	return true
}
