package auth

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	"github.com/your-username/go-mux-backend-template/server/internal/middlewares"
	"github.com/your-username/go-mux-backend-template/server/internal/utils"
)

// Controller owns the auth subrouter and all auth HTTP handlers.
//
// Architecture notes:
//   - Router is the only public field; it is mounted by the route registry.
//   - The service dependency is injected as a concrete *Service.
//   - Handler methods are regular methods (not closures) — `svc` is accessed
//     via the receiver, which is idiomatic Go and avoids closure capture bugs.
type Controller struct {
	// Router is the subrouter for all /auth/* endpoints.
	// Mount it in the route registry: apiRouter.PathPrefix("/auth").Subrouter()
	Router *mux.Router

	svc  *Service
	auth *middlewares.AuthMiddleware
}

// NewController creates an AuthController, wires its Service, and registers
// all routes on the provided subrouter.
func NewController(router *mux.Router, pool *pgxpool.Pool, bus *events.Bus) *Controller {
	c := &Controller{
		Router: router,
		svc:    NewService(pool, bus),
		auth:   middlewares.NewAuthMiddleware(),
	}
	c.registerRoutes()
	return c
}

// registerRoutes wires every handler to its path + method.
// Called once from the constructor — never called again.
func (c *Controller) registerRoutes() {
	c.Router.Handle("/register",
		middlewares.Validate[RegisterInput](http.HandlerFunc(c.register)),
	).Methods(http.MethodPost)

	c.Router.Handle("/login",
		middlewares.Validate[LoginInput](http.HandlerFunc(c.login)),
	).Methods(http.MethodPost)

	c.Router.Handle("/refresh",
		middlewares.Validate[RefreshInput](http.HandlerFunc(c.refresh)),
	).Methods(http.MethodPost)

	c.Router.Handle("/me",
		c.auth.Guard(http.HandlerFunc(c.me)),
	).Methods(http.MethodGet)
}

// ── Handlers ───────────────────────────────────────────────────────────────────
// Each handler is a method on *Controller so it accesses c.svc via the receiver.
// This is the Go equivalent of TypeScript's private arrow-function handlers.

func (c *Controller) register(w http.ResponseWriter, r *http.Request) {
	input := middlewares.BodyFromContext[RegisterInput](r.Context())
	utils.SendResponse(w, c.svc.Register(r.Context(), input, r))
}

func (c *Controller) login(w http.ResponseWriter, r *http.Request) {
	input := middlewares.BodyFromContext[LoginInput](r.Context())
	utils.SendResponse(w, c.svc.Login(r.Context(), input, r))
}

func (c *Controller) me(w http.ResponseWriter, r *http.Request) {
	uid := middlewares.UIDFromContext(r.Context())
	utils.SendResponse(w, c.svc.Me(r.Context(), uid))
}

func (c *Controller) refresh(w http.ResponseWriter, r *http.Request) {
	input := middlewares.BodyFromContext[RefreshInput](r.Context())
	utils.SendResponse(w, c.svc.RefreshToken(r.Context(), input))
}
