package utils

import (
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

// GetEnv returns the value of the environment variable key, or defaultVal if not set.
func GetEnv(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultVal
}

// GetParam extracts a named URL path variable from a gorilla/mux request.
// Returns an empty string if the variable is not present.
func GetParam(r *http.Request, key string) string {
	vars := mux.Vars(r)
	return vars[key]
}

// GetIP extracts the real client IP from the request, respecting common proxy headers.
func GetIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For may contain a comma-separated list; the first is the client
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
