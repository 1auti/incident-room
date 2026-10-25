package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newDist builds a fake frontend build next to a file that must never be served.
func newDist(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "dist")
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(dir, "index.html"):       "INDEX",
		filepath.Join(dir, "assets", "app.js"): "APPJS",
		filepath.Join(dir, "robots.txt"):       "ROBOTS",
		filepath.Join(root, "secret.txt"):      "SECRET",
	}
	for p, body := range files {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestStatic(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "api: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	})
	h := Static(newDist(t), api)

	const noCache, immutable = "no-cache", "public, max-age=31536000, immutable"
	tests := []struct {
		name, method, path string
		wantStatus         int
		wantBody           string // always asserted, "" means an empty body
		wantCache          string // always asserted, "" means no Cache-Control
	}{
		{"existing asset", http.MethodGet, "/assets/app.js", 200, "APPJS", immutable},
		{"head existing asset", http.MethodHead, "/assets/app.js", 200, "", immutable},
		{"existing non-asset file", http.MethodGet, "/robots.txt", 200, "ROBOTS", ""},
		{"unknown path falls back to index", http.MethodGet, "/incidents/42", 200, "INDEX", noCache},
		{"head on fallback", http.MethodHead, "/incidents/42", 200, "", noCache},
		{"root", http.MethodGet, "/", 200, "INDEX", noCache},
		{"index direct, no redirect", http.MethodGet, "/index.html", 200, "INDEX", noCache},
		{"directory falls back to index", http.MethodGet, "/assets", 200, "INDEX", noCache},
		{"missing asset with extension is 404", http.MethodGet, "/assets/missing.js", 404, "404 page not found\n", ""},
		{"missing favicon is 404", http.MethodGet, "/favicon.ico", 404, "404 page not found\n", ""},
		{"unknown api path goes to api", http.MethodGet, "/api/nope", 404, "api: GET /api/nope\n", ""},
		{"bare api goes to api", http.MethodGet, "/api", 404, "api: GET /api\n", ""},
		{"post to api goes to api", http.MethodPost, "/api/nope", 404, "api: POST /api/nope\n", ""},
		{"encoded api prefix goes to api", http.MethodGet, "/%61pi/x", 404, "api: GET /api/x\n", ""},
		{"similar prefix is not api", http.MethodGet, "/apiary", 200, "INDEX", noCache},
		{"apix is not api", http.MethodGet, "/apix", 200, "INDEX", noCache},
		{"uppercase API is not api", http.MethodGet, "/API", 200, "INDEX", noCache},
		{"traversal never leaves dir", http.MethodGet, "/../secret.txt", 404, "404 page not found\n", ""},
		{"encoded traversal never leaves dir", http.MethodGet, "/%2e%2e/secret.txt", 404, "404 page not found\n", ""},
		{"post outside api", http.MethodPost, "/anything", 405, "Method Not Allowed\n", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if rec.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
			if got := rec.Header().Get("Cache-Control"); got != tc.wantCache {
				t.Fatalf("Cache-Control = %q, want %q", got, tc.wantCache)
			}
			if tc.wantStatus == 405 && rec.Header().Get("Allow") != "GET, HEAD" {
				t.Fatalf("Allow = %q, want %q", rec.Header().Get("Allow"), "GET, HEAD")
			}
		})
	}
}

func TestStaticIndexDirect(t *testing.T) {
	h := Static(newDist(t), http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("unexpected redirect to %q", loc)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
}

func TestStaticMissingIndex(t *testing.T) {
	h := Static(t.TempDir(), http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/incidents/42", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
