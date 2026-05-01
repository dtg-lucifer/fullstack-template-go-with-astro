//go:build !dev

// Production filesystem — compiled when the "dev" build tag is NOT set.
// Both web/dist and the SQL migrations are baked directly into the binary,
// so the deployed artifact is a single self-contained file with zero runtime
// dependencies on the local filesystem.
//
// Build:  go build .          (default, no tag needed)
// Or:     make build
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

// ── Web frontend ──────────────────────────────────────────────────────────────

// webDistFS holds the compiled Astro output embedded at compile time.
// "all:" ensures _astro/ (Astro's hashed asset directory) is included —
// go:embed skips directories whose names start with _ or . by default.
//
//go:embed all:web/dist
var webDistFS embed.FS

// WebFS returns an http.Handler that serves the embedded Astro SPA.
// Unknown paths fall back to index.html so client-side routing works correctly.
func WebFS() http.Handler {
	sub, err := fs.Sub(webDistFS, "web/dist")
	if err != nil {
		panic("[WEB] failed to create sub-filesystem from embedded web/dist: " + err.Error())
	}

	fsys := http.FS(sub)
	fileServer := http.FileServer(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip the leading "/" so fs.Open receives a clean relative path.
		cleanPath := strings.TrimPrefix(r.URL.Path, "/")

		// If the file exists and is not a directory, serve it directly.
		if f, err := fsys.Open(cleanPath); err == nil {
			if info, err := f.Stat(); err == nil && !info.IsDir() {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			f.Close()
		}

		// Fall back to index.html for SPA client-side routing.
		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, "index.html not found in embedded filesystem", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(index) //nolint:errcheck
	})
}

// ── Database migrations ───────────────────────────────────────────────────────

// migrationsFS holds all *.sql migration files embedded at compile time.
//
//go:embed server/internal/db/migrations/*.sql
var migrationsFS embed.FS

// MigrationsFS returns the embedded SQL migration filesystem.
// Pass the result directly to golang-migrate's iofs source driver.
func MigrationsFS() (fs.FS, error) {
	if _, err := fs.Stat(migrationsFS, "server/internal/db/migrations"); os.IsNotExist(err) {
		return nil, fmt.Errorf("migrations directory not found in embedded filesystem")
	}
	return migrationsFS, nil
}
