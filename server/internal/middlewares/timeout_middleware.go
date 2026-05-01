package middlewares

import (
	"context"
	"net/http"
	"time"

	"github.com/your-username/go-mux-backend-template/server/internal/utils"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// TimeoutMiddleware wraps each request in a context with the given deadline.
// If the handler does not complete within the deadline, a 408 Request Timeout is returned.
// timeStr must be a valid Go duration string, e.g. "15s", "1m". Defaults to 10s on parse error.
func TimeoutMiddleware(timeStr string) func(http.Handler) http.Handler {
	duration, err := time.ParseDuration(timeStr)
	if err != nil {
		logger := pkg.NewLogger()
		if logger != nil {
			logger.Warn("Invalid timeout string, falling back to 10s", "value", timeStr)
			logger.Close()
		}
		duration = 10 * time.Second
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()

			r = r.WithContext(ctx)
			wr := utils.NewHttpWriter(w, r)

			done := make(chan struct{})
			go func() {
				next.ServeHTTP(w, r)
				close(done)
			}()

			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					wr.Error(ctx.Err(), http.StatusRequestTimeout)
				}
			case <-done:
				// Handler finished in time — nothing to do
			}
		})
	}
}
