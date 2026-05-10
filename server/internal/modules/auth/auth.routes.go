package auth

import (
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// RegisterRoutes is a convenience shim kept for backward compatibility.
// Prefer instantiating NewController directly in the route registry so the
// logger (and therefore debug proxy) is available.
//
// Deprecated: use NewController(pool, bus, logger) and mount ctrl.Router.
func RegisterRoutes(r *mux.Router, pool *pgxpool.Pool, bus *events.Bus, logger *pkg.Logger) {
	ctrl := NewController(pool, bus, logger)
	r.PathPrefix("/auth").Handler(ctrl.Router)
}
