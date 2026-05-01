//go:build dev

// Development filesystem — compiled only when the "dev" build tag IS set.
// Files are read directly from disk on every request, so changes to the
// frontend or SQL files are picked up without restarting the Go process.
//
// Build:  go build -tags dev .
// Or:     make dev   (Air passes -tags dev automatically)
package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

// ── Web frontend ──────────────────────────────────────────────────────────────

// WebFS returns an http.Handler that serves the Astro SPA directly from
// web/dist on disk. Run "make build-web" (or the Astro dev server) to keep
// the dist directory up to date while iterating on the frontend.
// Unknown paths fall back to index.html so client-side routing works correctly.
func WebFS() http.Handler {
	distDir := "web/dist"
	fileServer := http.FileServer(http.Dir(distDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(distDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Fall back to index.html for SPA client-side routing.
		http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
	})
}

// ── Database migrations ───────────────────────────────────────────────────────

// MigrationsFS returns the on-disk migration directory as an fs.FS.
// Changes to .sql files are reflected immediately without rebuilding.
func MigrationsFS() (fs.FS, error) {
	migrationsPath := filepath.Join("server", "internal", "db", "migrations")

	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("migrations directory not found on disk: %s", migrationsPath)
	}

	return os.DirFS(migrationsPath), nil
}
