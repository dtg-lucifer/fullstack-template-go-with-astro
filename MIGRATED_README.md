# Go + Mux Backend Template

A production-ready Go backend template built with **gorilla/mux**, **SQLC**, and **pgx**. Structured after the [bun-express-backend-template](https://github.com/dtg-lucifer/bun-express-backend-template) — same conventions, same discipline, different runtime.

## What's included

- **PostgreSQL** via `pgx/v5` with a connection pool
- **Type-safe SQL** via SQLC — queries in `internal/db/queries/`, generated code in `internal/db/repository/`
- **Migrations** via `golang-migrate` (plain `.up.sql` / `.down.sql` files)
- **OOP module pattern** — `Controller` struct owns the subrouter and handlers; `Service` struct owns business logic
- **`ApiResponse` envelope** — services return `ApiResponse`, handlers call `utils.SendResponse()`
- **JWT authentication** — access + refresh tokens, cookie + Bearer header support
- **Domain event bus** — in-process pub/sub for decoupled side effects (audit logs, job dispatch)
- **RabbitMQ job queue** — durable background jobs with retry; workers live in `internal/core/workers/`
- **Redis** cache client
- **WebSocket hub** — real-time broadcast driven by domain events
- **API documentation** — TypeSpec → `openapi.yaml` → [Scalar](https://scalar.com/) interactive UI at `/docs`
- **Per-request UUID** (`X-Request-ID` header, injected into every response body)
- **Structured logging** via `log/slog` — JSON to `logs/app.jsonl`, text to stdout
- **In-memory per-IP rate limiter**
- **CORS**, **timeout**, and **request validation** middleware
- **Graceful shutdown** on `SIGINT` / `SIGTERM`
- **Hot reload** via Air (`make dev`)
- Module rename script (`scripts/rename-module.sh`)

For a full explanation of every component, see [WORKFLOW.md](./WORKFLOW.md).

---

## Quick Start

### 1. Rename the module

```bash
./scripts/rename-module.sh github.com/your-org/your-project
go mod tidy
```

### 2. Install dev tools

```bash
make install        # go tools: sqlc, air, migrate, golangci-lint
make install-docs   # TypeSpec npm dependencies (docs/node_modules)
```

### 3. Start infrastructure

```bash
make infra          # PostgreSQL + Redis + RabbitMQ via Docker Compose
```

Or start services individually:

```bash
make db             # PostgreSQL only
make redis          # Redis only
make rabbitmq       # RabbitMQ only
```

### 4. Copy and fill in `.env`

```bash
cp .env.example .env
# Set JWT_SECRET and JWT_REFRESH_SECRET at minimum
```

### 5. Run migrations

```bash
make migrate-up
```

### 6. Generate SQLC repository code

```bash
make sqlc-gen
```

### 7. Start the dev server

```bash
make dev    # compiles TypeSpec docs first, then starts with hot reload
```

The API is available at `http://localhost:8080/api/v1`.
Interactive docs are at `http://localhost:8080/docs`.

---

## Make Targets

| Command | Description |
|---|---|
| `make dev` | Compile docs → start with hot reload (Air) |
| `make build` | Compile docs → build binary to `bin/app` |
| `make run` | Compile docs → build → run |
| `make docs` | Compile TypeSpec → regenerate `openapi.yaml` |
| `make docs-watch` | Watch TypeSpec files and recompile on change |
| `make install` | Install all Go dev tools |
| `make install-docs` | Install TypeSpec npm dependencies |
| `make test` | Run all tests with race detector |
| `make lint` | Run golangci-lint |
| `make fmt` | Format all Go files |
| `make sqlc-gen` | Regenerate `internal/db/repository/` from SQL |
| `make infra` | Start PostgreSQL + Redis + RabbitMQ |
| `make db` | Start PostgreSQL only |
| `make redis` | Start Redis only |
| `make rabbitmq` | Start RabbitMQ only |
| `make db-stop` | Stop all infrastructure |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back one migration |
| `make migrate-status` | Show current migration version |
| `make migrate-new` | Create a new migration file pair |
| `make clean` | Remove `bin/` and `logs/` |

---

## Project Structure

```
.
├── main.go                          Entry point
├── config.yaml                      Static, non-secret configuration
├── openapi.yaml                     Generated — do not edit (make docs)
├── .env.example                     Environment variable template
├── sqlc.yaml                        SQLC code generation config
├── Makefile
├── .air.toml                        Hot reload config
│
├── config/
│   └── config.go                    Config structs + YAML loader + env overrides
│
├── pkg/                             Shared utilities (no internal deps)
│   ├── logger.go                    Dual-output slog wrapper (file + stdout)
│   ├── jwt.go                       Token signing + verification
│   ├── password.go                  bcrypt hash + compare
│   └── env.go                       Env var helpers
│
├── internal/
│   ├── server.go                    Wires all subsystems, lifecycle management
│   │
│   ├── core/
│   │   ├── cache/                   Redis client wrapper
│   │   ├── events/                  In-process domain event bus
│   │   ├── queue/                   RabbitMQ producer + consumer manager
│   │   ├── realtime/                WebSocket hub
│   │   └── workers/                 Job handler functions (one file per job type)
│   │
│   ├── db/
│   │   ├── db.go                    pgxpool connection factory
│   │   ├── migrations/              SQL migration files
│   │   ├── queries/                 Hand-written SQL (sqlc input)
│   │   └── repository/              sqlc-generated Go code — do not edit
│   │       └── helpers.go           Hand-written helpers (StringToUUID, etc.)
│   │
│   ├── middlewares/                 HTTP middleware
│   │   ├── auth_middleware.go       JWT Guard + OptionalGuard + UIDFromContext
│   │   ├── validate_middleware.go   Decode + validate JSON body → context
│   │   ├── context.go               BodyFromContext[T], UIDFromContext
│   │   ├── cors_middleware.go       CORS headers
│   │   ├── logger_middleware.go     HTTP access log
│   │   ├── ratelimit_middleware.go  Per-IP sliding-window rate limiter
│   │   ├── requestid_middleware.go  UUID per request → X-Request-ID
│   │   └── timeout_middleware.go    Per-request context deadline
│   │
│   ├── modules/
│   │   ├── routes.go                Central route registry
│   │   ├── auth/
│   │   │   ├── auth.schema.go       Input types + Validate() methods
│   │   │   ├── auth.service.go      Business logic → ApiResponse
│   │   │   └── auth.controller.go   Controller struct, routes, handler methods
│   │   └── health/
│   │       └── health.routes.go     GET /health
│   │
│   └── utils/
│       └── http.go                  ApiResponse, SendResponse, HttpWriter
│
├── docs/                            TypeSpec API documentation project
│   ├── main.tsp                     Service metadata + imports
│   ├── tspconfig.yaml               Emitter config (outputs ../openapi.yaml)
│   ├── package.json                 TypeSpec npm dependencies
│   ├── models/
│   │   ├── common.tsp               Shared envelope + error models
│   │   ├── auth.tsp                 Auth request/response models
│   │   └── health.tsp               Health check models
│   └── routes/
│       ├── auth.tsp                 Auth route definitions
│       └── health.tsp               Health route definitions
│
├── scripts/
│   └── rename-module.sh             Rename the Go module path across the project
│
└── docker/
    └── docker-compose.yaml          PostgreSQL, Redis, RabbitMQ for local dev
```

---

## Module Pattern

Each feature module lives in `internal/modules/<name>/` with three files.

### Schema (`<name>.schema.go`)

Input types with validation — never changes between modules:

```go
type CreateThingInput struct {
    Name string `json:"name"`
}

func (i *CreateThingInput) Validate() error {
    if strings.TrimSpace(i.Name) == "" {
        return errors.New("name is required")
    }
    return nil
}
```

### Service (`<name>.service.go`)

Business logic. Holds `*repository.Queries` directly — no wrapper layer. Returns `utils.ApiResponse`, never touches `http.ResponseWriter`:

```go
type Service struct {
    repo *repository.Queries
    bus  *events.Bus
}

func NewService(pool *pgxpool.Pool, bus *events.Bus) *Service {
    return &Service{repo: repository.New(pool), bus: bus}
}

func (s *Service) Create(ctx context.Context, input CreateThingInput) utils.ApiResponse {
    thing, err := s.repo.CreateThing(ctx, repository.CreateThingParams{Name: input.Name})
    if err != nil {
        return utils.ApiError("failed to create thing", err.Error(), 500)
    }
    return utils.ApiSuccess("thing created", map[string]any{"thing": thing}, 201)
}
```

### Controller (`<name>.controller.go`)

Owns the subrouter and all handler methods. `Router` is the only exported field:

```go
type Controller struct {
    Router *mux.Router

    svc  *Service
    auth *middlewares.AuthMiddleware
}

func NewController(pool *pgxpool.Pool, bus *events.Bus) *Controller {
    c := &Controller{
        Router: mux.NewRouter(),
        svc:    NewService(pool, bus),
        auth:   middlewares.NewAuthMiddleware(),
    }
    c.registerRoutes()
    return c
}

func (c *Controller) registerRoutes() {
    c.Router.Handle("/",
        middlewares.Validate[CreateThingInput](http.HandlerFunc(c.create)),
    ).Methods(http.MethodPost)
}

func (c *Controller) create(w http.ResponseWriter, r *http.Request) {
    input := middlewares.BodyFromContext[CreateThingInput](r.Context())
    utils.SendResponse(w, c.svc.Create(r.Context(), input))
}
```

### Register in `routes.go`

```go
thingCtrl := thing.NewController(pool, bus)
apiRouter.PathPrefix("/things").Handler(thingCtrl.Router)
```

---

## API Documentation

Documentation is written in [TypeSpec](https://typespec.io/) and compiled to `openapi.yaml`, which is served as an interactive [Scalar](https://scalar.com/) UI.

```bash
make docs       # compile TypeSpec → openapi.yaml
```

The UI is available at `http://localhost:8080/docs` when `documentation.swagger.enabled: true` in `config.yaml`.

`openapi.yaml` is generated — it is in `.gitignore` and should never be edited by hand.

### Adding docs for a new module

1. `docs/models/<name>.tsp` — request/response models
2. `docs/routes/<name>.tsp` — route interface, imports from `../models/<name>.tsp`
3. Add both imports to `docs/main.tsp`
4. `make docs`

---

## API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/health` | — | Server health, DB ping, memory, uptime |
| `POST` | `/api/v1/auth/register` | — | Register a new user |
| `POST` | `/api/v1/auth/login` | — | Login, returns access + refresh tokens |
| `GET` | `/api/v1/auth/me` | Bearer | Current authenticated user |
| `POST` | `/api/v1/auth/refresh` | — | Exchange refresh token for new access token |

---

## Environment Variables

| Variable | Description |
|---|---|
| `DB_URL` | Full PostgreSQL DSN — overrides individual DB fields in config.yaml |
| `JWT_SECRET` | Access token signing secret |
| `JWT_REFRESH_SECRET` | Refresh token signing secret |
| `AMQP_URL` | RabbitMQ connection URL |
| `REDIS_ADDR` | Redis `host:port` |
| `REDIS_PASSWORD` | Redis password |
| `PORT` | HTTP listen port — overrides config.yaml |
| `HOST` | HTTP bind address — overrides config.yaml |
| `ENV` | Environment name (`development` / `production`) |
| `ALLOWED_ORIGINS` | Comma-separated CORS allowed origins |

---

## Migrations

```bash
make migrate-new      # prompts for a name, creates the .up.sql and .down.sql pair
make migrate-up       # apply all pending migrations
make migrate-down     # roll back one migration
make migrate-status   # show current version
```

After adding or changing queries, regenerate the repository:

```bash
make sqlc-gen
```

---

## Infrastructure

All services are defined in `docker/docker-compose.yaml`.

| Service | Port | Notes |
|---|---|---|
| PostgreSQL | `5432` | Database |
| Redis | `6379` | Cache + session store |
| RabbitMQ | `5672` | Job queue (AMQP) |
| RabbitMQ UI | `15672` | Management console — `guest` / `guest` |

Any service can be disabled in `config.yaml` without removing it from Docker Compose:

```yaml
redis:
  enabled: false

queue:
  rabbitmq:
    enabled: false

realtime:
  websocket:
    enabled: false
```
