package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHandlerIndexAndFallback(t *testing.T) {
	h := Handler()
	for _, path := range []string{"/", "/index.html", "/login", "/zones"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		h.ServeHTTP(rec, req)
		res := rec.Result()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d want 200", path, res.StatusCode)
		}
		if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Errorf("%s Content-Type=%q want text/html", path, ct)
		}
		if got := res.Header.Get("Cache-Control"); got != cacheIndex {
			t.Errorf("%s Cache-Control=%q want %q", path, got, cacheIndex)
		}
		assertSecurityHeaders(t, path, res)
		if !strings.Contains(string(body), "LabDNS") {
			t.Errorf("%s body missing LabDNS placeholder", path)
		}
	}
}

func TestHandlerHeadIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	Handler().ServeHTTP(rec, req)
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("HEAD / status=%d want 200", res.StatusCode)
	}
	if got := res.Header.Get("Cache-Control"); got != cacheIndex {
		t.Errorf("Cache-Control=%q want %q", got, cacheIndex)
	}
	assertSecurityHeaders(t, "HEAD /", res)
}

func TestHandlerAssetPathsAre404(t *testing.T) {
	h := Handler()
	for _, path := range []string{"/assets", "/assets/", "/assets/missing-app.js"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		h.ServeHTTP(rec, req)
		res := rec.Result()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("%s status=%d want 404", path, res.StatusCode)
		}
		if got := res.Header.Get("Cache-Control"); got == cacheHashed {
			t.Fatalf("%s must not be marked immutable", path)
		}
		if got := res.Header.Get("Cache-Control"); got == cacheIndex {
			t.Fatalf("%s must not SPA-fallback to index.html", path)
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "operator console stub") {
			t.Fatalf("%s must not serve index.html", path)
		}
		assertSecurityHeaders(t, path, res)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); !strings.Contains(got, http.MethodGet) {
		t.Errorf("Allow=%q", got)
	}
	assertSecurityHeaders(t, "POST /", rec.Result())
}

func TestHashedAssetCacheControl(t *testing.T) {
	if got := cacheControl("/assets/index-abc.js", true); got != cacheHashed {
		t.Fatalf("got %q want %q", got, cacheHashed)
	}
	if got := cacheControl("/assets/index-abc.js", false); got != "" {
		t.Fatalf("missing asset cache %q", got)
	}
	if got := cacheControl("/", false); got != cacheIndex {
		t.Fatalf("index cache %q", got)
	}
}

func TestCommittedStubExists(t *testing.T) {
	for _, rel := range []string{"dist/index.html", "stub/index.html"} {
		b, err := os.ReadFile(rel)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if !strings.Contains(string(b), "LabDNS") {
			t.Errorf("%s missing LabDNS placeholder", rel)
		}
		if strings.Contains(string(b), "<style") || strings.Contains(string(b), `style="`) {
			t.Errorf("%s injects styles (forbidden by style-src 'self')", rel)
		}
		if strings.Contains(string(b), "<script") {
			t.Errorf("%s includes a script tag; stub must stay placeholder HTML", rel)
		}
	}
}

func assertSecurityHeaders(t *testing.T, name string, res *http.Response) {
	t.Helper()
	if got := res.Header.Get("Content-Security-Policy"); got != csp {
		t.Errorf("%s CSP=%q", name, got)
	}
	if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("%s nosniff=%q", name, got)
	}
	if got := res.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("%s referrer=%q", name, got)
	}
	if got := res.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("%s frame=%q", name, got)
	}
}
