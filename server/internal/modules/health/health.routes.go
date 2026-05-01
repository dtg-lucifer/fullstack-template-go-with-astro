// Package health exposes a single GET /health endpoint.
package health

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-username/go-mux-backend-template/server/internal/core/cache"
	"github.com/your-username/go-mux-backend-template/server/internal/utils"
)

// RegisterRoutes mounts the health check route onto the provided router.
func RegisterRoutes(r *mux.Router, pool *pgxpool.Pool, redis cache.Cache, startTime time.Time) {
	r.HandleFunc("/health", healthHandler(pool, redis, startTime)).Methods(http.MethodGet)
}

func healthHandler(pool *pgxpool.Pool, redis cache.Cache, startTime time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()

		dbStatus := "ok"
		if err := pool.Ping(ctx); err != nil {
			dbStatus = "unreachable: " + err.Error()
		}

		redisStatus := "disabled"
		if redis != nil {
			redisStatus = "ok"
			if err := redis.Ping(ctx); err != nil {
				redisStatus = "unreachable: " + err.Error()
			}
		}

		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		stat := pool.Stat()
		utils.NewHttpWriter(w, r).Status(http.StatusOK).JSON(utils.M{
			"success": true,
			"status":  "ok",
			"uptime":  time.Since(startTime).String(),
			"services": utils.M{
				"database": utils.M{
					"status":     dbStatus,
					"open_conns": stat.TotalConns(),
					"idle_conns": stat.IdleConns(),
				},
				"redis": utils.M{
					"status": redisStatus,
				},
			},
			"memory": utils.M{
				"alloc_mb":       toMB(mem.Alloc),
				"total_alloc_mb": toMB(mem.TotalAlloc),
				"sys_mb":         toMB(mem.Sys),
				"num_gc":         mem.NumGC,
			},
		})
	}
}

func toMB(b uint64) float64 { return float64(b) / 1024 / 1024 }
