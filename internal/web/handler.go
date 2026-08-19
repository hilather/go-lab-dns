package web

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const (
	csp = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

	cacheHashed = "public, max-age=31536000, immutable"
	cacheIndex  = "no-store"
)

var content fs.FS

func init() {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("internal/web: embed dist: " + err.Error())
	}
	content = sub
}

// Handler returns the SPA file server.
func Handler() http.Handler {
	return http.HandlerFunc(serve)
}

func serve(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reqPath := path.Clean("/" + r.URL.Path)
	if reqPath == "/" || reqPath == "/index.html" {
		serveIndex(w, r)
		return
	}

	rel := strings.TrimPrefix(reqPath, "/")
	if isAssetPath(reqPath) {
		st, err := fs.Stat(content, rel)
		if err == nil && !st.IsDir() {
			if cc := cacheControl(reqPath, true); cc != "" {
				w.Header().Set("Cache-Control", cc)
			}
			serveFile(w, r, rel)
			return
		}
		http.NotFound(w, r)
		return
	}

	st, err := fs.Stat(content, rel)
	if err == nil && !st.IsDir() {
		serveFile(w, r, rel)
		return
	}
	serveIndex(w, r)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", cacheIndex)
	serveFile(w, r, "index.html")
}

func serveFile(w http.ResponseWriter, r *http.Request, name string) {
	f, err := content.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "unsupported file", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, path.Base(name), st.ModTime(), rs)
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
}

func isAssetPath(reqPath string) bool {
	return reqPath == "/assets" || strings.HasPrefix(reqPath, "/assets/")
}

func cacheControl(urlPath string, exists bool) string {
	if urlPath == "/" || urlPath == "/index.html" {
		return cacheIndex
	}
	if exists && isAssetPath(urlPath) {
		return cacheHashed
	}
	return ""
}
