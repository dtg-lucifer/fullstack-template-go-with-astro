# Application Workflow & Architecture

This document explains every module, component, and subsystem in this template — what it is, why it exists, and how it connects to everything else.

---

## Table of Contents

1. [High-Level Architecture](#1-high-level-architecture)
2. [Project Structure](#2-project-structure)
3. [Startup Sequence](#3-startup-sequence)
4. [Request Lifecycle](#4-request-lifecycle)
5. [Module Pattern (Controller + Service)](#5-module-pattern-controller--service)
6. [PostgreSQL & SQLC Query Layer](#6-postgresql--sqlc-query-layer)
7. [Authentication & JWT](#7-authentication--jwt)
8. [Middleware Stack](#8-middleware-stack)
9. [Domain Events & Audit Logs](#9-domain-events--audit-logs)
10. [Queue & Workers](#10-queue--workers)
11. [WebSocket / Realtime](#11-websocket--realtime)
12. [API Documentation (TypeSpec + Scalar)](#12-api-documentation-typespec--scalar)
13. [Configuration System](#13-configuration-system)
14. [Logging System](#14-logging-system)
15. [API Response Convention](#15-api-response-convention)
16. [Graceful Shutdown](#16-graceful-shutdown)
17. [Adding Features](#17-adding-features)

---

## 1. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                          CLIENT                              │
│                      (HTTP REST)                             │
└───────────────────────────┬──────────────────────────────────┘
                            │ HTTP
                            ▼
┌──────────────────────────────────────────────────────────────┐
│                   gorilla/mux Router                         │
│                                                              │
│  Global middleware chain:                                    │
│  RequestID → CORS → RateLimit → Logger                       │
│                                                              │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐     │
│  │ GET /health │  │ /auth/*      │  │  (your modules)  │     │
│  └─────────────┘  └──────┬───────┘  └──────────────────┘     │
│                          │ ctrl.Router (subrouter)           │
└──────────────────────────┼───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│                      Controller                              │
│  Parses validated input from context, calls service,         │
│  calls utils.SendResponse()                                  │
└───────────────────────────┬──────────────────────────────────┘
                            │
                            ▼
┌──────────────────────────────────────────────────────────────┐
│                      Service                                 │
│  All business logic. Holds *repository.Queries directly.     │
│  Returns utils.ApiResponse — never touches ResponseWriter.   │
└───────────────────────────┬──────────────────────────────────┘
                            │
                            ▼
┌──────────────────────────────────────────────────────────────┐
│              repository.Queries (sqlc-generated)             │
│                      PostgreSQL (pgx)                        │
└──────────────────────────────────────────────────────────────┘
```

---

## 2. Project Structure

```
.
├── main.go                          Entry point
├── config.yaml                      Static, non-secret config
├── openapi.yaml                     Generated — do not edit (make docs)
├── Makefile
│
├── config/
│   └── config.go                    Config structs + loader
│
├── pkg/                             Shared utilities (no internal deps)
│   ├── logger.go                    Dual-output slog wrapper
│   ├── jwt.go                       Token signing + verification
│   ├── password.go                  bcrypt helpers
│   └── env.go                       Env var helpers
│
├── internal/
│   ├── server.go                    Wires all subsystems, lifecycle mgmt
│   │
│   ├── core/
│   │   ├── cache/                   Redis client wrapper
│   │   ├── events/                  In-process domain event bus
│   │   ├── queue/                   RabbitMQ producer + consumer
│   │   ├── realtime/                WebSocket hub
│   │   └── workers/                 Job handler functions (one file per job)
│   │
│   ├── db/
│   │   ├── db.go                    pgxpool connection helper
│   │   ├── migrations/              golang-migrate SQL files
│   │   ├── queries/                 Hand-written SQL (sqlc input)
│   │   └── repository/              sqlc-generated Go code — do not edit
│   │
│   ├── middlewares/                 HTTP middleware (auth, cors, logger, …)
│   │
│   ├── modules/
│   │   ├── routes.go                Central route registry
│   │   ├── auth/
│   │   │   ├── auth.schema.go       Input types + Validate() methods
│   │   │   ├── auth.service.go      Business logic → ApiResponse
│   │   │   └── auth.controller.go   Controller struct, routes, handlers
│   │   └── health/
│   │       └── health.routes.go     Health check endpoint
│   │
│   └── utils/
│       └── http.go                  ApiResponse, SendResponse, HttpWriter
│
└── docs/                            TypeSpec API documentation project
    ├── main.tsp                     Entry point — service metadata + imports
    ├── tspconfig.yaml               Emitter config (outputs ../openapi.yaml)
    ├── package.json                 TypeSpec npm dependencies
    ├── models/
    │   ├── common.tsp               Shared envelope + error models
    │   ├── auth.tsp                 Auth request/response models
    │   └── health.tsp               Health check models
    └── routes/
        ├── auth.tsp                 Auth route definitions
        └── health.tsp               Health route definitions
```

---

## 3. Startup Sequence

**Entry point:** `main.go`

```
main.go
  │
  ├─ godotenv.Load()              load .env (non-fatal if missing)
  ├─ pkg.NewLogger()              dual-output logger (file + stdout)
  ├─ config.NewConfig(path)       parse config.yaml, apply env overrides
  └─ server.New(cfg, logger)
       │
       ├─ setupDatabase()         open pgxpool, ping
       ├─ setupRedis()            connect Redis (skipped if disabled)
       ├─ setupEventBus()         in-process domain event bus
       ├─ setupRealtime()         WebSocket hub (skipped if disabled)
       ├─ setupQueue()            RabbitMQ + start workers (skipped if disabled)
       ├─ setupEventHandlers()    wire bus listeners (audit log, email job)
       ├─ setupRouter()           gorilla/mux + middleware + mount modules
       ├─ setupDocs()             Scalar UI at /docs (skipped if disabled)
       └─ httpSrv.ListenAndServe
```

---

## 4. Request Lifecycle

```
Incoming Request
      │
      ▼
  RequestIDMiddleware    generate UUID → X-Request-ID header
      │
      ▼
  CORSMiddleware         validate Origin, set Access-Control-* headers
      │
      ▼
  RateLimiter.Middleware per-IP sliding window (config.yaml)
      │
      ▼
  LoggerMiddleware       log method, path, status, duration, IP
      │
      ▼
  Validate[T] middleware (route-level) decode + validate JSON body → context
      │
      ▼
  AuthMiddleware.Guard   (protected routes only) verify Bearer JWT → uid in context
      │
      ▼
  Controller handler     pull input from context, call service
      │
      ▼
  Service method         business logic → ApiResponse
      │
      ▼
  utils.SendResponse()   inject request_id, WriteHeader, json.Encode
```

---

## 5. Module Pattern (Controller + Service)

Each feature module lives in `internal/modules/<name>/` and has three files:

| File | Responsibility |
|---|---|
| `<name>.schema.go` | Input structs + `Validate()` methods — untouched by codegen |
| `<name>.service.go` | Business logic, holds `*repository.Queries`, returns `utils.ApiResponse` |
| `<name>.controller.go` | `Controller` struct, `NewController`, route wiring, handler methods |

### Controller

```go
type Controller struct {
    Router *mux.Router          // only exported field — mount this in routes.go

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
```

- `Router` is the only exported field. Mount it with `apiRouter.PathPrefix("/x").Handler(ctrl.Router)`.
- Handlers are methods on `*Controller` — they access `c.svc` via the receiver, no closures needed.
- `registerRoutes()` is called last in the constructor, after all fields are set.

### Service

```go
type Service struct {
    repo *repository.Queries   // sqlc-generated, used directly — no wrapper
    bus  *events.Bus
}

func NewService(pool *pgxpool.Pool, bus *events.Bus) *Service {
    return &Service{repo: repository.New(pool), bus: bus}
}

func (s *Service) DoThing(ctx context.Context, input ThingInput) utils.ApiResponse {
    result, err := s.repo.CreateThing(ctx, repository.CreateThingParams{...})
    if err != nil {
        return utils.ApiError("failed to create thing", err.Error(), 500)
    }
    return utils.ApiSuccess("thing created", map[string]any{"thing": result}, 201)
}
```

- Services never touch `http.ResponseWriter` — they only return `utils.ApiResponse`.
- The `pool` field is not stored; it is only used in `NewService` to build `repository.New(pool)`.

### Route registry

`internal/modules/routes.go` is the only place modules are mounted:

```go
func Register(apiRouter *mux.Router, pool *pgxpool.Pool, redis cache.Cache, bus *events.Bus, startTime time.Time, logger *pkg.Logger) {
    health.RegisterRoutes(apiRouter, pool, redis, startTime)

    authCtrl := auth.NewController(pool, bus)
    apiRouter.PathPrefix("/auth").Handler(authCtrl.Router)
}
```

---

## 6. PostgreSQL & SQLC Query Layer

**Files:** `internal/db/queries/`, `internal/db/migrations/`, `internal/db/repository/`

**Why SQLC:** SQL stays in `.sql` files — readable, reviewable, version-controlled. SQLC generates fully type-safe Go functions. No ORM, no runtime reflection.

```
internal/db/queries/users.sql
  │
  └─ make sqlc-gen
       │
       └─► internal/db/repository/
               ├─ db.go          DBTX interface + New()
               ├─ models.go      Go structs mirroring DB tables
               └─ users.sql.go   Generated query functions
```

**Adding a query:**

1. Write SQL in `internal/db/queries/<domain>.sql` with a `-- name: FunctionName :one/:many/:exec` annotation
2. `make sqlc-gen`
3. Call `s.repo.FunctionName(ctx, params)` from your service

**Migrations** use `golang-migrate` with sequential numbered files:

```
internal/db/migrations/
  000001_init_auth.up.sql
  000001_init_auth.down.sql
  000002_add_posts.up.sql     ← created by: make migrate-new
  000002_add_posts.down.sql
```

---

## 7. Authentication & JWT

**Files:** `pkg/jwt.go`, `internal/middlewares/auth_middleware.go`, `internal/modules/auth/`

**Token flow:**

```
POST /auth/register
  └─► AuthService.Register()
        ├─ check email uniqueness (repo.GetUserByEmail)
        ├─ pkg.HashPassword()          bcrypt
        ├─ repo.CreateUser()
        ├─ bus.EmitUserRegistered()    → queues welcome email
        └─ bus.EmitAuditLog()

POST /auth/login
  └─► AuthService.Login()
        ├─ repo.GetUserByEmail()
        ├─ pkg.ComparePassword()       bcrypt timing-safe compare
        ├─ pkg.NewTokenSigner(JWT_SECRET).Sign()          24h access token
        ├─ pkg.NewTokenSigner(JWT_REFRESH_SECRET).Sign()  30d refresh token
        └─ bus.EmitAuditLog()

GET /auth/me  (protected)
  └─► AuthMiddleware.Guard
        ├─ extract token from Authorization: Bearer header or "jwt" cookie
        ├─ pkg.NewTokenSigner(JWT_SECRET).Verify()
        └─ inject uid into context
  └─► AuthService.Me(uid)
        └─ repo.GetUserByID()

POST /auth/refresh
  └─► AuthService.RefreshToken()
        ├─ pkg.NewTokenSigner(JWT_REFRESH_SECRET).Verify()
        └─ issue new access token
```

**Reading the UID in a handler:**

```go
uid := middlewares.UIDFromContext(r.Context())
```

**`OptionalGuard`** — does not reject the request if no token is present; useful for endpoints that behave differently for authenticated vs anonymous users.

---

## 8. Middleware Stack

**Files:** `internal/middlewares/`

| File | What it does |
|---|---|
| `requestid_middleware.go` | Generates a UUID per request, sets `X-Request-ID` header |
| `cors_middleware.go` | Sets `Access-Control-*` headers from config |
| `ratelimit_middleware.go` | In-memory per-IP sliding-window rate limiter |
| `logger_middleware.go` | Logs method, path, status, duration, IP |
| `auth_middleware.go` | `Guard` (required), `OptionalGuard`, `UIDFromContext` |
| `validate_middleware.go` | Decodes + validates JSON body, stores result in context |
| `context.go` | `BodyFromContext[T]`, `UIDFromContext` — typed context helpers |

**Registration order in `server.go`** (matters):

```
RequestID → CORS → RateLimit → Logger
```

RequestID runs first so every subsequent log entry has the ID available.

**Reading a validated body in a handler:**

```go
input := middlewares.BodyFromContext[RegisterInput](r.Context())
```

---

## 9. Domain Events & Audit Logs

**Files:** `internal/core/events/`

The event bus is an in-process pub/sub. Services emit events; listeners react asynchronously.

```
Service
  └─ bus.EmitUserRegistered(payload)
       │
       └─► OnUserRegistered listener (server.go)
             └─ qmgr.Publish(EmailQueue, WelcomeEmailJob)

  └─ bus.EmitAuditLog(payload)
       │
       └─► OnAuditLog listener (server.go)
             └─ goroutine: repo.CreateAuditLog()   (best-effort, non-blocking)
```

Audit log writes are fire-and-forget — they never block the request path. If the write fails it is logged but not retried.

---

## 10. Queue & Workers

**Files:** `internal/core/queue/`, `internal/core/workers/`

RabbitMQ is used for durable background jobs. The queue is optional — set `queue.rabbitmq.enabled: false` in `config.yaml` to run without it.

```
Service emits event
  └─► bus listener publishes job to RabbitMQ queue
        │
        └─► workers.WelcomeEmailHandler (consumer goroutine)
              └─ process job, ack on success / nack + requeue on failure
```

**Worker files** live in `internal/core/workers/` — one file per job type. Workers are pure functions: they receive a typed job struct and return `error`. They are passed as callbacks to `queue.Manager.ConsumeXxx`.

```go
// internal/core/workers/email_worker.go
func WelcomeEmailHandler(logger *pkg.Logger) func(queue.WelcomeEmailJob) error {
    return func(job queue.WelcomeEmailJob) error {
        // TODO: call your email service here
        logger.Info("[WORKER] Sending welcome email", "user_id", job.UserID)
        return nil
    }
}
```

**Adding a new job type:**

1. Define the job struct in `internal/core/queue/queue.go`
2. Add `ConsumeXxx` method to `queue.Manager`
3. Create `internal/core/workers/<job>_worker.go`
4. Wire the consumer in `server.go → setupQueue()`

---

## 11. WebSocket / Realtime

**Files:** `internal/core/realtime/`

The WebSocket hub is optional — set `realtime.websocket.enabled: false` to disable it.

```
Client connects to /ws
  └─► realtime.Hub.ServeHTTP()
        ├─ upgrade HTTP → WebSocket
        ├─ register client
        └─ listen for domain events from the bus → broadcast to clients
```

The hub subscribes to the domain event bus and forwards relevant events to connected WebSocket clients.

---

## 12. API Documentation (TypeSpec + Scalar)

**Files:** `docs/`, `openapi.yaml`, `internal/server.go → setupDocs()`

The API is documented using [TypeSpec](https://typespec.io/). TypeSpec compiles to `openapi.yaml` at the project root, which is then served as an interactive [Scalar](https://scalar.com/) UI.

### File layout

```
docs/
├── main.tsp              Service metadata, server URL, imports
├── tspconfig.yaml        Emitter config → outputs ../openapi.yaml
├── models/
│   ├── common.tsp        Shared: ApiSuccess<T>, ApiError, error models
│   ├── auth.tsp          Auth: SafeUser, request bodies, response shapes
│   └── health.tsp        Health: DatabaseStatus, MemoryStats, HealthData
└── routes/
    ├── auth.tsp          Auth route definitions (imports models/auth.tsp)
    └── health.tsp        Health route definitions (imports models/health.tsp)
```

**Rule:** route files contain only interface definitions. All models live in `docs/models/`.

### Workflow

```bash
make docs          # compile TypeSpec → openapi.yaml
make dev           # compile docs first, then start server with hot reload
make build         # compile docs first, then build binary
```

The Scalar UI is served at the path configured in `config.yaml`:

```yaml
documentation:
  swagger:
    enabled: true
    path: /docs
    openapi_file: openapi.yaml
```

`openapi.yaml` is generated — it is listed in `.gitignore` and should never be edited by hand.

### Adding docs for a new module

1. Create `docs/models/<name>.tsp` — define all request/response models
2. Create `docs/routes/<name>.tsp` — define the interface, import from `../models/<name>.tsp`
3. Add both imports to `docs/main.tsp`
4. `make docs` to regenerate `openapi.yaml`

---

## 13. Configuration System

**Files:** `config/config.go`, `config.yaml`, `.env`

**Two-layer config:**

```
config.yaml     static, committed, non-secret settings
.env            secrets and per-environment overrides (not committed)
```

**Key sections in `config.yaml`:**

| Section | Purpose |
|---|---|
| `server` | Host, port, API prefix, environment, TLS |
| `security` | CORS origins, rate limit window + max requests |
| `database` | Host, port, credentials, pool size |
| `redis` | Host, port, enabled flag |
| `middlewares` | Toggle request ID, logger, body parser |
| `realtime.websocket` | Enable/disable WebSocket, path, buffer sizes |
| `queue.rabbitmq` | Enable/disable RabbitMQ, retry attempts, backoff |
| `workers` | Enable/disable worker process and notification jobs |
| `documentation.swagger` | Enable/disable Scalar UI, path, openapi_file path |

**Environment variable overrides** (take precedence over `config.yaml`):

| Env var | Overrides |
|---|---|
| `DB_URL` | Entire database DSN |
| `PORT` | `server.port` |
| `HOST` | `server.host` |
| `ENV` | `server.environment` |
| `REDIS_ADDR` | Redis host:port |
| `REDIS_PASSWORD` | Redis password |
| `AMQP_URL` | RabbitMQ connection URL |
| `JWT_SECRET` | Access token signing key (env only) |
| `JWT_REFRESH_SECRET` | Refresh token signing key (env only) |
| `ALLOWED_ORIGINS` | CORS allowed origins (comma-separated) |

---

## 14. Logging System

**Files:** `pkg/logger.go`

```
pkg.Logger
  │
  ├─► FileLogger   (slog.JSONHandler → logs/app.jsonl)   level: DEBUG (captures everything)
  └─► StdoutLogger (slog.TextHandler → os.Stdout)        level: INFO  (human-readable)
```

**Usage:**

```go
logger.Info("user registered", "user_id", user.ID)
logger.Warn("redis unavailable, skipping cache")
logger.Error("db query failed", "error", err)
logger.Debug("token verified", "uid", uid)
```

The logger uses slog-style key-value pairs. `Debug` calls only appear in the file log unless the console level is set to `debug` in `config.yaml → logging.level`.

---

## 15. API Response Convention

**File:** `internal/utils/http.go`

All endpoints return a single JSON envelope:

```json
// Success
{
  "success": true,
  "message": "user registered successfully",
  "data": { "user": { ... } },
  "status_code": 201,
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}

// Error
{
  "success": false,
  "message": "email already registered",
  "errors": null,
  "status_code": 409,
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Services** build responses:

```go
return utils.ApiSuccess("ok", data, 200)
return utils.ApiError("not found", nil, 404)
```

**Handlers** send them:

```go
utils.SendResponse(w, c.svc.DoThing(r.Context(), input))
```

`SendResponse` injects the `X-Request-ID` header value into `request_id` before encoding.

---

## 16. Graceful Shutdown

**File:** `internal/server.go`

```
SIGINT / SIGTERM received
  │
  └─► httpSrv.Shutdown(30s timeout)
        ├─ stop accepting new connections
        ├─ wait for in-flight requests to complete
        ├─ qmgr.Close()     close RabbitMQ channel + connection
        ├─ redis.Close()    close Redis connection pool
        └─ pool.Close()     close pgxpool
```

---

## 17. Adding Features

### New module checklist

1. **SQL** → `internal/db/queries/<name>.sql`
2. **Regenerate** → `make sqlc-gen`
3. **Schema** → `internal/modules/<name>/<name>.schema.go`
   - Input structs with `Validate() error` methods
4. **Service** → `internal/modules/<name>/<name>.service.go`
   - `Service` struct holding `*repository.Queries` and `*events.Bus`
   - `NewService(pool, bus)` constructor
   - Methods return `utils.ApiResponse`, never touch `http.ResponseWriter`
5. **Controller** → `internal/modules/<name>/<name>.controller.go`
   - `Controller` struct with `Router *mux.Router` as the only exported field
   - `NewController(pool, bus)` calls `NewService`, then `registerRoutes()`
   - Handlers are methods on `*Controller`
6. **Register** → add to `internal/modules/routes.go`:
   ```go
   ctrl := name.NewController(pool, bus)
   apiRouter.PathPrefix("/name").Handler(ctrl.Router)
   ```
7. **Docs** → `docs/models/<name>.tsp` + `docs/routes/<name>.tsp`, import in `docs/main.tsp`
8. **Regenerate docs** → `make docs`

### New migration checklist

1. `make migrate-new` → enter a name
2. Write SQL in the generated `.up.sql` and `.down.sql` files
3. `make migrate-up`
4. If the migration adds/changes tables, update `.sql` query files and `make sqlc-gen`

### New worker checklist

1. Define the job struct in `internal/core/queue/queue.go`
2. Add `ConsumeXxx(ctx, concurrency, handler)` to `queue.Manager`
3. Create `internal/core/workers/<job>_worker.go` with `XxxHandler(logger) func(Job) error`
4. Wire in `server.go → setupQueue()`: `qmgr.ConsumeXxx(ctx, concurrency, workers.XxxHandler(s.logger))`
