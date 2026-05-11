package middlewares

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/your-username/go-mux-backend-template/server/internal/utils"
)

// LoggerMiddleware logs every HTTP request with method, path, status code, duration, and
// client IP. It writes to both stdout and logs/events.log. Query/params/body are logged
// only when env is development.
func LoggerMiddleware(env string) func(http.Handler) http.Handler {
	// Ensure the log directory and file exist before the first request arrives
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		if err := os.Mkdir("logs", os.ModePerm); err != nil {
			slog.Error("Failed to create logs directory", "error", err)
		}
	}
	if _, err := os.Stat("logs/events.log"); os.IsNotExist(err) {
		if _, err := os.Create("logs/events.log"); err != nil {
			slog.Error("Failed to create events log file", "error", err)
		}
	}

	stdLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logDetails := strings.EqualFold(env, "development")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			var body string
			if logDetails && r.Body != nil {
				bodyBytes, _ := io.ReadAll(r.Body)
				body = string(bodyBytes)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// Wrap the writer so we can capture the status code after the handler runs
			rw := &utils.ResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			args := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.StatusCode,
				"duration", duration,
				"ip", utils.GetIP(r),
				"request_id", w.Header().Get("X-Request-ID"),
			}
			if logDetails {
				args = append(args,
					"query", r.URL.RawQuery,
					"params", mux.Vars(r),
					"body", body,
				)
			}

			// Open the events log for this request (append mode)
			f, err := os.OpenFile("logs/events.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
			if err != nil {
				slog.Error("Failed to open events log", "error", err)
			} else {
				defer f.Close()
				fileLogger := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
				fileLogger.Info("HTTP request", args...)
			}

			stdLogger.Info("HTTP request", args...)
		})
	}
}
