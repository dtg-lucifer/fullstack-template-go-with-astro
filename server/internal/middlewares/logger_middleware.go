package middlewares

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/your-username/go-mux-backend-template/server/internal/utils"
)

// LoggerMiddleware logs every HTTP request with method, path, status code, duration, and
// client IP. It writes to both stdout and logs/events.log.
func LoggerMiddleware(next http.Handler) http.Handler {
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the writer so we can capture the status code after the handler runs
		rw := &utils.ResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		// Open the events log for this request (append mode)
		f, err := os.OpenFile("logs/events.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			slog.Error("Failed to open events log", "error", err)
		} else {
			defer f.Close()
			fileLogger := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
			fileLogger.Info("HTTP request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.StatusCode),
				slog.Duration("duration", duration),
				slog.String("ip", utils.GetIP(r)),
				slog.String("request_id", w.Header().Get("X-Request-ID")),
			)
		}

		stdLogger.Info("HTTP request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.StatusCode),
			slog.Duration("duration", duration),
			slog.String("ip", utils.GetIP(r)),
			slog.String("request_id", w.Header().Get("X-Request-ID")),
		)
	})
}
