package auth

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	"github.com/your-username/go-mux-backend-template/server/internal/middlewares"
)

// RegisterRoutes mounts all auth routes onto the provided subrouter.
//
//	POST /auth/register  — create a new account
//	POST /auth/login     — authenticate, receive access + refresh tokens
//	GET  /auth/me        — current authenticated user (protected)
//	POST /auth/refresh   — exchange refresh token for a new access token
func RegisterRoutes(r *mux.Router, pool *pgxpool.Pool, bus *events.Bus) {
	svc := newService(pool, bus)
	auth := middlewares.NewAuthMiddleware()

	sub := r.PathPrefix("/auth").Subrouter()

	sub.Handle("/register",
		middlewares.Validate[RegisterInput](http.HandlerFunc(register(svc))),
	).Methods(http.MethodPost)

	sub.Handle("/login",
		middlewares.Validate[LoginInput](http.HandlerFunc(login(svc))),
	).Methods(http.MethodPost)

	sub.Handle("/refresh",
		middlewares.Validate[RefreshInput](http.HandlerFunc(refresh(svc))),
	).Methods(http.MethodPost)

	sub.Handle("/me",
		auth.Guard(http.HandlerFunc(me(svc))),
	).Methods(http.MethodGet)
}
