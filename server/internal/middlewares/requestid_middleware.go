// Package middlewares contains all HTTP middleware components for the application.
// Middleware handles cross-cutting concerns: request IDs, logging, CORS, auth, timeouts.
package middlewares

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// RequestIDMiddleware generates a UUID for every incoming request, attaches it to the
// X-Request-ID response header, and logs the incoming request. Downstream handlers and
// the HttpWriter.JSON() helper read this header to include the ID in every response body.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := pkg.NewLogger()
		defer logger.Close()

		rid, err := uuid.NewRandom()
		if err != nil {
			logger.Error("Failed to generate request ID", "error", err)
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-Request-ID", rid.String())
		logger.Info("Incoming request", "request_id", rid.String(), "method", r.Method, "path", r.URL.Path)

		next.ServeHTTP(w, r)
	})
}
