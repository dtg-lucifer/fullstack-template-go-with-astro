// Package modules is the central route registry.
// Add a single line here to mount a new module — nothing else needs to change.
package modules

import (
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-username/go-mux-backend-template/server/internal/core/cache"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	"github.com/your-username/go-mux-backend-template/server/internal/modules/auth"
	"github.com/your-username/go-mux-backend-template/server/internal/modules/health"
)

// Register mounts every module's routes onto the API subrouter.
// apiRouter is already scoped to the API prefix (e.g. /api/v1).
func Register(apiRouter *mux.Router, pool *pgxpool.Pool, redis cache.Cache, bus *events.Bus, startTime time.Time) {
	health.RegisterRoutes(apiRouter, pool, redis, startTime)
	auth.RegisterRoutes(apiRouter, pool, bus)
	// Add new modules here:
	// user.RegisterRoutes(apiRouter, pool, redis, bus)
}
