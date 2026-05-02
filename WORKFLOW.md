# Application Workflow & Architecture

This document explains every module, component, and subsystem in this template — what it is, why it exists, and how it connects to everything else.

---

## Table of Contents

1. [High-Level Architecture](#1-high-level-architecture)
2. [Startup Sequence](#2-startup-sequence)
3. [Request Lifecycle](#3-request-lifecycle)
4. [PostgreSQL & SQLC Query Layer](#4-postgresql--sqlc-query-layer)
5. [Authentication & JWT](#5-authentication--jwt)
6. [Middleware Stack](#6-middleware-stack)
7. [Module Structure](#7-module-structure)
8. [Service Layer](#8-service-layer)
9. [Domain Event Bus](#9-domain-event-bus)
10. [WebSocket Realtime Hub](#10-websocket-realtime-hub)
11. [Redis Cache Layer](#11-redis-cache-layer)
12. [RabbitMQ Job Queue](#12-rabbitmq-job-queue)
13. [Configuration System](#13-configuration-system)
14. [Logging System](#14-logging-system)
15. [API Response Convention](#15-api-response-convention)
16. [Graceful Shutdown](#16-graceful-shutdown)
17. [Adding Features](#17-adding-features)

---

## 1. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         CLIENT                              │
│              (HTTP REST  /  WebSocket)                      │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTP
                           ▼
┌──────────────────────────────────────────────────────────────┐
│                  gorilla/mux Router                          │
│                                                              │
│  Global middleware chain:                                    │
│  RequestID → CORS → RateLimit → Logger                       │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ GET /health  │  │ POST /auth/* │  │  (your modules)  │   │
│  └──────────────┘  └──────┬───────┘  └──────────────────┘   │
│                           │                                  │
│  ┌──────────────┐  ┌──────────────┐                          │
│  │  GET /ws     │  │  GET /*      │  (SPA fallback)          │
│  └──────────────┘  └──────────────┘                          │
└─────────────────────────── │ ────────────────────────────────┘
                             │ service calls
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                     Service Layer                           │
│                                                             │
│  AuthService ──► repository.Queries ──► PostgreSQL (pgx)    │
│                │                                            │
│                └──► events.Bus ──► WebSocket Hub            │
│                                └──► RabbitMQ Queue          │
│                                └──► Audit Log (async DB)    │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Startup Sequence

**Entry point:** `main.go`

```
main.go
  │
  ├─ pkg.NewLogger()              initialise dual-output logger (file + stdout)
  ├─ godotenv.Load()              load .env into process env (non-fatal if missing)
  ├─ config.NewConfig(path)       parse config.yaml, apply defaults + env overrides
  ├─ WebFS()                      fs_dev.go (disk) or fs_prod.go (embedded), by build tag
  └─ server.New(cfg, logger, webHandler).Setup(ctx)
       │
       ├─ setupDatabase()         open pgxpool, ping database
       ├─ setupRedis()            connect Redis (skipped if redis.enabled: false)
       ├─ setupEventBus()         create in-process typed event bus
       ├─ setupRealtime()         create WebSocket hub (skipped if websocket.enabled: false)
       ├─ setupQueue()            connect RabbitMQ, declare queues, start consumers
       │                          (skipped if queue.rabbitmq.enabled: false)
       ├─ setupEventHandlers()    wire bus listeners: audit log writer, email job enqueuer
       └─ setupRouter()           create mux router, attach middleware, mount modules,
                                  mount WebSocket endpoint, mount frontend SPA handler
  └─ srv.Start()
       └─ httpSrv.ListenAndServe  start accepting connections (blocks until SIGINT/SIGTERM)
```

---

## 3. Request Lifecycle

Every HTTP request passes through this pipeline before reaching a handler:

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
  RateLimiter.Middleware per-IP sliding window (configurable in config.yaml)
      │
      ▼
  LoggerMiddleware       log method, path, status, duration, IP
      │
      ▼
  AuthMiddleware.Guard   (protected routes only) verify Bearer JWT or "jwt" cookie
      │
      ▼
  Validate[T] middleware (routes with a body) decode JSON → call T.Validate() →
                         store result in context
      │
      ▼
  Route Handler          retrieve validated body via BodyFromContext[T],
                         call service, get ApiResponse, call SendResponse()
      │
      ▼
  SendResponse()         inject X-Request-ID into body, WriteHeader(), json.Encode()
```

---

## 4. PostgreSQL & SQLC Query Layer

**Files:** `server/internal/db/queries/`, `server/internal/db/migrations/`, `server/internal/db/repository/`

**Why SQLC:** SQL stays in `.sql` files — readable, reviewable, and version-controlled. SQLC generates fully type-safe Go functions from those queries. No ORM magic, no runtime reflection.

**How it works:**

```
server/internal/db/queries/users.sql
  │
  └─ sqlc generate (make sqlc-gen)
       │
       └─► server/internal/db/repository/
               ├─ db.go          DBTX interface + New()
               ├─ models.go      Go structs mirroring DB tables
               ├─ helpers.go     Hand-written helpers (StringToUUID, etc.)
               └─ users.sql.go   Generated query functions
```

**Adding a query:**

1. Write SQL in `server/internal/db/queries/<domain>.sql` with a `-- name: FunctionName :one/:many/:exec` annotation
2. Run `make sqlc-gen`
3. Call the generated function from your service via `s.repo.FunctionName(ctx, params)`

**Migrations** use `golang-migrate` with sequential numbered files:

```
server/internal/db/migrations/
  000001_init_auth.up.sql
  000001_init_auth.down.sql
  000002_add_posts.up.sql     ← created by: make migrate-new
  000002_add_posts.down.sql
```

**Connection pool** is configured in `config.yaml → database`:

- `pool_size` — max open connections (default: 10)
- `connection_timeout_ms` — dial timeout in milliseconds (default: 10000)
- `idle_timeout_ms` — idle connection timeout in milliseconds (default: 30000)

---

## 5. Authentication & JWT

**Files:** `server/pkg/jwt.go`, `server/pkg/password.go`, `server/internal/middlewares/auth_middleware.go`, `server/internal/modules/auth/`

**Password hashing:** Argon2id (OWASP-recommended parameters: 64 MiB memory, 3 iterations, 4 threads). The encoded hash is self-describing — cost parameters are stored inside the hash string so they can be changed for new passwords without breaking existing ones.

**Token flow:**

```
POST /auth/register
  └─► Validate[RegisterInput] middleware
  └─► AuthService.Register()
        ├─ repo.GetUserByEmail()     check uniqueness
        ├─ pkg.HashPassword()        Argon2id hash
        ├─ repo.CreateUser()
        ├─ bus.EmitUserRegistered()  → enqueues welcome email via RabbitMQ
        └─ bus.EmitAuditLog()        → async DB write

POST /auth/login
  └─► Validate[LoginInput] middleware
  └─► AuthService.Login()
        ├─ repo.GetUserByEmail()
        ├─ pkg.ComparePassword()     Argon2id constant-time compare
        ├─ pkg.NewTokenSigner(JWT_SECRET).Sign()          24h access token
        ├─ pkg.NewTokenSigner(JWT_REFRESH_SECRET).Sign()  30d refresh token
        └─ bus.EmitAuditLog()        → async DB write

GET /auth/me  (protected)
  └─► AuthMiddleware.Guard
        ├─ extract token from "jwt" cookie or Authorization: Bearer header
        ├─ pkg.NewTokenSigner(JWT_SECRET).Verify()
        └─ inject uid into request context
  └─► AuthService.Me(uid)
        └─ repo.GetUserByID()

POST /auth/refresh
  └─► Validate[RefreshInput] middleware
  └─► AuthService.RefreshToken()
        ├─ pkg.NewTokenSigner(JWT_REFRESH_SECRET).Verify()
        └─ issue new access token (24h)
```

**`OptionalGuard`**: A variant of `Guard` that does not reject the request if no token is present — useful for endpoints that behave differently for authenticated vs anonymous users.

**Reading the UID in a handler:**

```go
uid := middlewares.UIDFromContext(r.Context())
```

---

## 6. Middleware Stack

**Files:** `server/internal/middlewares/`

| File | What it does |
|---|---|
| `requestid_middleware.go` | Generates a UUID per request, sets `X-Request-ID` response header |
| `logger_middleware.go` | Logs method, path, status, duration, IP to stdout + `logs/events.log` |
| `cors_middleware.go` | Sets `Access-Control-*` headers from `ALLOWED_ORIGINS` env var or `config.yaml` |
| `auth_middleware.go` | `Guard` (required auth), `OptionalGuard`, `UIDFromContext` |
| `ratelimit_middleware.go` | In-memory per-IP sliding-window rate limiter |
| `validate_middleware.go` | Generic `Validate[T]` — decodes JSON body, calls `T.Validate()`, stores in context |
| `context.go` | Context key helpers (`contextWithBody`, `validatedBodyKey`) |

**Registration order in `server.go`** (matters):

```
RequestID → CORS → RateLimit → Logger
```

RequestID runs first so every subsequent middleware and log entry has the ID available. Note: `TimeoutMiddleware` is defined but not wired into the global chain — apply it per-route if needed.

**The `Validate[T]` middleware** is the standard way to parse and validate request bodies. It is applied per-route in `routes.go`, not globally:

```go
sub.Handle("/register",
    middlewares.Validate[RegisterInput](http.HandlerFunc(register(svc))),
).Methods(http.MethodPost)
```

The handler then retrieves the validated value from context:

```go
input := middlewares.BodyFromContext[RegisterInput](r.Context())
```

---

## 7. Module Structure

**Files:** `server/internal/modules/`

Modules are self-contained feature packages. Each module owns its schema, service, and routes. The central registry in `routes.go` is the only place you need to touch to add a new module.

```
server/internal/modules/
  routes.go          ← central registry: call RegisterRoutes for each module here
  health/
    health.routes.go ← GET /health handler (no service needed)
  auth/
    auth.schema.go   ← input types + Validate() rules
    auth.service.go  ← business logic returning utils.ApiResponse
    auth.routes.go   ← RegisterRoutes() + handler functions
```

**Pattern inside a module's routes file:**

```go
func RegisterRoutes(r *mux.Router, pool *pgxpool.Pool, bus *events.Bus) {
    svc := newService(pool, bus)
    auth := middlewares.NewAuthMiddleware()

    sub := r.PathPrefix("/things").Subrouter()

    // Public route with body validation:
    sub.Handle("",
        middlewares.Validate[CreateThingInput](http.HandlerFunc(createThing(svc))),
    ).Methods(http.MethodPost)

    // Protected route:
    sub.Handle("/me",
        auth.Guard(http.HandlerFunc(getMyThing(svc))),
    ).Methods(http.MethodGet)
}
```

**Handler functions** are plain functions returning `http.HandlerFunc`:

```go
func createThing(svc *Service) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        input := middlewares.BodyFromContext[CreateThingInput](r.Context())
        utils.SendResponse(w, svc.Create(r.Context(), input))
    }
}
```

**Registering a new module** — one line in `server/internal/modules/routes.go`:

```go
func Register(apiRouter *mux.Router, pool *pgxpool.Pool, redis cache.Cache, bus *events.Bus, startTime time.Time) {
    health.RegisterRoutes(apiRouter, pool, redis, startTime)
    auth.RegisterRoutes(apiRouter, pool, bus)
    things.RegisterRoutes(apiRouter, pool, bus) // ← add here
}
```

---

## 8. Service Layer

**Files:** `server/internal/modules/<name>/<name>.service.go`

Services contain all business logic. They:

- Accept typed input structs (validated by the middleware before reaching the handler)
- Call `repository.Queries` methods for DB access
- Emit domain events via `events.Bus`
- Return `utils.ApiResponse` — **never** touch `http.ResponseWriter`

```go
func (s *Service) Create(ctx context.Context, input CreateInput) utils.ApiResponse {
    thing, err := s.repo.CreateThing(ctx, repository.CreateThingParams{
        Name: input.Name,
    })
    if err != nil {
        return utils.ApiError("failed to create thing", err.Error(), 500)
    }
    return utils.ApiSuccess("thing created", map[string]any{"thing": thing}, 201)
}
```

This separation means services are trivially unit-testable without an HTTP layer.

---

## 9. Domain Event Bus

**Files:** `server/internal/core/events/`

```
server/internal/core/events/
  bus.go           ← goroutine-safe in-process pub/sub bus
  audit.event.go   ← AuditLogPayload + EmitAuditLog / OnAuditLog
  user.event.go    ← UserRegisteredPayload + EmitUserRegistered / OnUserRegistered
  job.event.go     ← JobEnqueuedPayload + EmitJobEnqueued / OnJobEnqueued
```

The bus is a typed, in-process pub/sub system. Services emit events; listeners react to them. Neither side knows about the other, keeping business logic decoupled from side-effects.

**Listeners are registered in `server.go → setupEventHandlers()`:**

- `OnAuditLog` — spawns a goroutine to write the audit record to PostgreSQL asynchronously. Never blocks the request path.
- `OnUserRegistered` — publishes a `WelcomeEmailJob` to RabbitMQ, then emits `JobEnqueued`.

**Emitting an event from a service:**

```go
s.bus.EmitUserRegistered(events.UserRegisteredPayload{
    UserID: user.ID.String(),
    Email:  user.Email,
})
```

**Adding a new event:**

1. Create `server/internal/core/events/<name>.event.go` with a payload struct and `Emit<Name>` / `On<Name>` methods on `*Bus`
2. Emit from the relevant service
3. Register a listener in `setupEventHandlers()` in `server.go`

---

## 10. WebSocket Realtime Hub

**Files:** `server/internal/core/realtime/hub.go`

The Hub manages connected WebSocket clients and broadcasts domain events to all of them. It is wired to the event bus at startup and implements `http.Handler` so it mounts directly on the router.

```
GET /ws
  └─► hub.ServeHTTP()
        ├─ upgrader.Upgrade()       HTTP → WebSocket
        ├─ register client
        ├─ send "system:hello" frame
        └─ spawn readPump + writePump goroutines per client

bus.OnUserRegistered → hub.Broadcast("auth:user-registered", payload)
bus.OnJobEnqueued    → hub.Broadcast("queue:job-enqueued", payload)
```

**Wire format** — every message is a JSON frame:

```json
{ "event": "auth:user-registered", "payload": { "user_id": "...", "email": "..." } }
```

**Disable:** set `realtime.websocket.enabled: false` in `config.yaml` — the hub is never created and the `/ws` route is not mounted.

---

## 11. Redis Cache Layer

**Files:** `server/internal/core/cache/redis.go`

The `Cache` interface decouples modules from the concrete Redis client:

```go
type Cache interface {
    Get(ctx, key) (string, error)
    Set(ctx, key, value, ttl) error
    Del(ctx, keys...) error
    Exists(ctx, keys...) (bool, error)
    Ping(ctx) error
    Close() error
}
```

`ErrCacheMiss` (`redis.Nil`) is returned by `Get` when a key does not exist.

**Disable:** set `redis.enabled: false` in `config.yaml` — `s.redis` is `nil` and the health endpoint reports `"disabled"`.

**Using Redis in a module:**

```go
func RegisterRoutes(r *mux.Router, pool *pgxpool.Pool, redis cache.Cache, ...) {
    svc := newService(pool, redis, ...)
    ...
}
```

---

## 12. RabbitMQ Job Queue

**Files:** `server/internal/core/queue/queue.go`

The `Manager` owns the AMQP connection and channel. It declares durable queues at startup and provides `Publish` and `ConsumeWelcomeEmails`.

```
POST /auth/register
  └─► bus.EmitUserRegistered
        └─► OnUserRegistered listener (server.go)
              └─► qmgr.Publish(ctx, EmailQueue, WelcomeEmailJob{...})
                    └─► RabbitMQ broker
                          └─► ConsumeWelcomeEmails worker goroutines
                                └─► processWelcomeEmail(job) — your email logic here
```

**Feature flags** (all in `config.yaml`):

| Flag | Effect |
|---|---|
| `queue.rabbitmq.enabled: false` | No AMQP connection; queue-backed event handlers are skipped |
| `workers.process.enabled: false` | Consumer goroutines never start |
| `workers.notification_jobs.enabled: false` | Email job consumer is skipped |

**Adding a new job type:**

1. Define a job struct in `queue.go`
2. Declare a new queue name constant and add it to `declareQueues()`
3. Add a `Consume<JobType>` method on `Manager`
4. Emit the job from the relevant event listener in `setupEventHandlers()`

---

## 13. Configuration System

**Files:** `server/config/config.go`, `config.yaml`, `.env`

**Two-layer config:**

```
config.yaml     static, committed, non-secret settings
.env            secrets and environment-specific overrides
```

**Environment overrides** (take precedence over `config.yaml`):

| Env var | Overrides |
|---|---|
| `DB_URL` | Entire database DSN |
| `PORT` | `server.port` |
| `HOST` | `server.host` |
| `ENV` | `server.environment` |
| `JWT_SECRET` | *(env only — no yaml equivalent)* |
| `JWT_REFRESH_SECRET` | *(env only — no yaml equivalent)* |
| `ALLOWED_ORIGINS` | `security.cors.origins` |
| `REDIS_ADDR` | Redis `host:port` |
| `REDIS_PASSWORD` | Redis password |
| `AMQP_URL` | RabbitMQ connection URL |

**Optional subsystems** can be disabled without touching Go code:

```yaml
redis:
  enabled: false

realtime:
  websocket:
    enabled: false

queue:
  rabbitmq:
    enabled: false

workers:
  process:
    enabled: false
  notification_jobs:
    enabled: false
```

---

## 14. Logging System

**Files:** `server/pkg/logger.go`, `server/internal/middlewares/logger_middleware.go`

```
pkg.Logger
  │
  ├─► FileLogger   (slog.JSONHandler → logs/app.jsonl)   level: DEBUG (captures everything)
  └─► StdoutLogger (slog.TextHandler → os.Stdout)        level: INFO  (human-readable)

LoggerMiddleware
  │
  ├─► os.Stdout                                           HTTP access log (text)
  └─► logs/events.log                                     HTTP access log (text, append)
```

**Usage in services and handlers:**

```go
logger := pkg.NewLogger()
defer logger.Close()
logger.Info("user registered", "user_id", user.ID)
logger.Error("db query failed", "error", err)
```

The logger is passed into `server.New()` and flows down to subsystems that need it (queue manager, WebSocket hub, etc.).

---

## 15. API Response Convention

**Files:** `server/internal/utils/http.go`

All responses follow a single JSON envelope:

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
resp := svc.DoThing(r.Context(), input)
utils.SendResponse(w, resp)
```

`SendResponse` reads the `X-Request-ID` response header and injects it into `request_id` before encoding.

**`HttpWriter`** is an alternative fluent API for ad-hoc responses (used in the health handler):

```go
utils.NewHttpWriter(w, r).Status(http.StatusOK).JSON(utils.M{
    "success": true,
    "status":  "ok",
})
```

---

## 16. Graceful Shutdown

**File:** `server/server.go`

```
SIGINT / SIGTERM received
  │
  └─► httpSrv.Shutdown(ctx with 30s timeout)
        ├─ stop accepting new connections
        └─ wait for in-flight requests to complete
  └─► qmgr.Close()     close RabbitMQ channel + connection
  └─► redis.Close()    release Redis connection pool
  └─► pool.Close()     close pgxpool
```

Subsystems are torn down in reverse setup order. Each step is guarded by a nil check so disabled subsystems are safely skipped.

---

## 17. Adding Features

### New route module checklist

1. **SQL queries** → `server/internal/db/queries/<name>.sql`
2. **Regenerate** → `make sqlc-gen`
3. **Module directory** → `server/internal/modules/<name>/`
   - `<name>.schema.go` — input types + `Validate()` methods
   - `<name>.service.go` — business logic returning `utils.ApiResponse`
   - `<name>.routes.go` — `RegisterRoutes()` + handler functions
4. **Register** → add one line to `server/internal/modules/routes.go`:

```go
func Register(apiRouter *mux.Router, pool *pgxpool.Pool, redis cache.Cache, bus *events.Bus, startTime time.Time) {
    health.RegisterRoutes(apiRouter, pool, redis, startTime)
    auth.RegisterRoutes(apiRouter, pool, bus)
    things.RegisterRoutes(apiRouter, pool, bus) // ← add here
}
```

### New migration checklist

1. `make migrate-new` → enter a name
2. Write SQL in the generated `.up.sql` and `.down.sql` files
3. `make migrate-up`
4. If the migration adds/changes tables with queries, update `.sql` files and `make sqlc-gen`

### New domain event checklist

1. Create `server/internal/core/events/<name>.event.go` with a payload struct and typed `Emit<Name>` / `On<Name>` methods on `*Bus`
2. Emit from the relevant service: `s.bus.Emit<Name>(payload)`
3. Register a listener in `setupEventHandlers()` in `server/server.go`

### New job type checklist

1. Define a job struct in `server/internal/core/queue/queue.go`
2. Add a queue name constant and declare it in `declareQueues()`
3. Add a `Consume<JobType>` method on `Manager`
4. Wire it in `setupQueue()` in `server/server.go`
5. Publish from an event listener in `setupEventHandlers()`
