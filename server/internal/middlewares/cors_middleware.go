package middlewares

import (
	"net/http"
	"strings"

	"github.com/your-username/go-mux-backend-template/server/internal/utils"
)

// CORSMiddleware sets Access-Control-* headers based on the CORS_ORIGINS environment
// variable (comma-separated list of allowed origins). Preflight OPTIONS requests are
// answered immediately with 204 No Content.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedOrigins := utils.GetEnv("CORS_ORIGINS", "http://localhost:3000,http://localhost:5173")

		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = r.Header.Get("Host")
		}

		if strings.Contains(allowedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
