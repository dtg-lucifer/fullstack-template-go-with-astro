package auth

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	"github.com/your-username/go-mux-backend-template/server/internal/middlewares"
	"github.com/your-username/go-mux-backend-template/server/internal/utils"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// Controller owns the auth subrouter and all auth HTTP handlers.
//
// Architecture notes (mirroring the OOP pattern from the migration report):
//   - Router is the only public field; it is mounted by the route registry.
//   - The service dependency is injected as ServiceIface so the controller
//     never depends on the concrete *Service type.
//   - WithDebug is called in the constructor so every service method is
//     automatically instrumented with debug logging.
//   - Handler methods are regular methods (not closures) — `svc` is accessed
//     via the receiver, which is idiomatic Go and avoids closure capture bugs.
type Controller struct {
	// Router is the subrouter for all /auth/* endpoints.
	// Mount it in the route registry: apiRouter.PathPrefix("/auth").Handler(ctrl.Router)
	Router *mux.Router

	svc  ServiceIface
	auth *middlewares.AuthMiddleware
}

// NewController creates an AuthController, wires its ServiceIface (with debug
// logging enabled), and registers all routes.
//
// Equivalent to the TypeScript AuthController constructor that calls
// AuthService.withDebug(...) and this.registerRoutes().
func NewController(pool *pgxpool.Pool, bus *events.Bus, logger *pkg.Logger) *Controller {
	c := &Controller{
		Router: mux.NewRouter(),
		svc:    WithDebug(pool, bus, logger),
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
