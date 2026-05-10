// Package modules is the central route registry.
// Add a single line here to mount a new module — nothing else needs to change.
//
// Pattern (mirroring the OOP migration):
//  1. Instantiate each controller, passing the shared dependencies it needs.
//  2. Mount controller.Router onto the API subrouter.
//  3. Controllers that don't need DB/bus (e.g. HealthController) take no pool/bus args.
package modules

import (
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-username/go-mux-backend-template/server/internal/core/cache"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	"github.com/your-username/go-mux-backend-template/server/internal/modules/auth"
	"github.com/your-username/go-mux-backend-template/server/internal/modules/health"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// Register mounts every module's routes onto the API subrouter.
// apiRouter is already scoped to the API prefix (e.g. /api/v1).
func Register(apiRouter *mux.Router, pool *pgxpool.Pool, redis cache.Cache, bus *events.Bus, startTime time.Time, logger *pkg.Logger) {
	// ── Health ─────────────────────────────────────────────────────────────────
	// No DB/bus dependency — health check reads pool/redis directly for liveness.
	health.RegisterRoutes(apiRouter, pool, redis, startTime)

	// ── Auth ───────────────────────────────────────────────────────────────────
	authCtrl := auth.NewController(pool, bus, logger)
	apiRouter.PathPrefix("/auth").Handler(authCtrl.Router)

	// ── Add new modules here ───────────────────────────────────────────────────
	// userCtrl := user.NewController(pool, bus, logger)
	// apiRouter.PathPrefix("/users").Handler(userCtrl.Router)
}
