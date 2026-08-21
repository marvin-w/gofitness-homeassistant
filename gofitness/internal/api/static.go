package api

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/marvin-w/gofitness-homeassistant/gofitness/web"
)

// registerStatic serves the embedded single-page frontend.
//
// Home Assistant ingress mounts the add-on under a path like
// /api/hassio_ingress/<token>/, so nothing may use absolute URLs. The index
// page gets a <base> tag built from the X-Ingress-Path header and every asset
// and API call in the frontend is written relative to it.
func (s *Server) registerStatic(m *http.ServeMux) {
	assets, err := fs.Sub(web.Files, "static")
	if err != nil {
		s.log.Error("embedded assets missing", "err", err)
		return
	}
	fileServer := http.FileServer(http.FS(assets))

	m.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")

		// Anything that exists on disk is served as-is.
		if clean != "" && clean != "index.html" {
			if f, err := assets.Open(clean); err == nil {
				f.Close()
				// Hashed assets would allow long caching; these are not hashed,
				// so revalidation keeps updates from being missed after an
				// add-on upgrade.
				w.Header().Set("Cache-Control", "no-cache")
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		s.serveIndex(w, r)
	})
}

// serveIndex renders index.html with the ingress base path injected.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	raw, err := fs.ReadFile(web.Files, "static/index.html")
	if err != nil {
		http.Error(w, "frontend not built", http.StatusInternalServerError)
		return
	}

	base := strings.TrimRight(strings.TrimSpace(r.Header.Get("X-Ingress-Path")), "/") + "/"
	if base == "/" {
		base = "./"
	}

	page := bytes.Replace(raw, []byte("__BASE_HREF__"), []byte(htmlEscape(base)), 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	// The frontend is entirely self-contained, so a strict policy costs nothing
	// and keeps a compromised recipe link from turning into script execution.
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; "+
			"script-src 'self'; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors *")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Write(page)
}

// htmlEscape escapes the few characters that matter inside an attribute value.
func htmlEscape(s string) string {
	r := strings.NewReplacer(`&`, "&amp;", `<`, "&lt;", `>`, "&gt;", `"`, "&quot;", `'`, "&#39;")
	return r.Replace(s)
}
