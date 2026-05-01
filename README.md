# Go + Astro Fullstack Template

A production-ready fullstack template that ships as a **single binary**. The Go backend embeds the compiled Astro frontend at build time — one file, zero runtime dependencies, deploy anywhere.

Built on **gorilla/mux**, **SQLC**, **pgx**, and **Astro**. Batteries included: auth, audit logs, WebSocket, Redis, RabbitMQ, rate limiting, structured logging, graceful shutdown, and hot reload.

For a deep dive into every subsystem, see [WORKFLOW.md](./WORKFLOW.md).

---

## What's included

**Backend**
- PostgreSQL via `pgx/v5` with a connection pool
- Type-safe SQL via **SQLC** — write SQL, get Go functions
- Database migrations via **golang-migrate**
- JWT authentication (access + refresh tokens, cookie + Bearer header)
- In-memory domain event bus — decoupled side-effects
- WebSocket hub wired to the event bus
- Redis cache layer with a clean `Cache` interface
- RabbitMQ job queue with retry logic
- Per-request UUID (`X-Request-ID`)
- Structured logging via `log/slog` — JSON to file, text to stdout
- In-memory per-IP rate limiter
- CORS, timeout, and request-ID middleware
- Async audit log writer (every auth action recorded to DB)
- Graceful shutdown on `SIGINT` / `SIGTERM`

**Frontend**
- Astro project in `web/` — swap for any framework (React, Svelte, Vue, vanilla)
- Built output embedded into the Go binary via `go:embed`
- SPA fallback routing — unknown paths serve `index.html`

**Developer experience**
- Hot reload via **Air** (`make dev`) — Go rebuilds on save, frontend reads from disk live
- Two-mode filesystem: `fs_dev.go` (disk) vs `fs_prod.go` (embedded), selected by build tag
- Module rename script — one command to make this template yours
- Docker Compose for PostgreSQL, Redis, and RabbitMQ

---

## Quick start

### 1. Rename the module

```bash
./scripts/rename-module.sh github.com/your-org/your-project
go mod tidy
```

### 2. Install dev tools

```bash
make install
```

This installs: `sqlc`, `air`, `golang-migrate`, and `golangci-lint`.

### 3. Start infrastructure

```bash
make infra        # PostgreSQL + Redis + RabbitMQ (all three)
# or individually:
make db           # PostgreSQL only
make redis        # Redis only
make rabbitmq     # RabbitMQ only
```

To disable services you don't need, set `enabled: false` in `config.yaml` for `redis`, `queue.rabbitmq`, and `realtime.websocket`.

### 4. Configure environment

```bash
cp .env.example .env
# Edit .env — at minimum set JWT_SECRET and JWT_REFRESH_SECRET
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
make dev
```

Air starts the Go server with `-tags dev`. The backend hot-reloads on any `.go` or `.yaml` change. The frontend is served live from `web/dist/` on disk — run `cd web && bun run dev` in a separate terminal for frontend hot reload.

---

## Building for production

```bash
make build
```

This runs two steps in order:

1. `make build-web` — runs `bun run build` inside `web/`, producing `web/dist/`
2. `make build-server` — runs `go build .`, which embeds `web/dist/` and all SQL migrations into the binary via `go:embed`

The result is a single self-contained binary at `bin/app`. Copy it anywhere with `config.yaml` and a `.env` file and it runs.

```bash
./bin/app -config config.yaml
```

---

## Make commands

| Command | Description |
|---|---|
| `make dev` | Start backend with hot reload (Air, `-tags dev`) |
| `make build` | Build frontend then embed into Go binary |
| `make build-web` | Build Astro frontend only → `web/dist/` |
| `make build-server` | Build Go binary only (requires `web/dist/` to exist) |
| `make run` | Build everything then run the binary |
| `make test` | Run all tests with race detector |
| `make lint` | Run golangci-lint |
| `make fmt` | Format all Go files |
| `make sqlc-gen` | Regenerate `server/internal/db/repository/` from SQL |
| `make infra` | Start all Docker services (PostgreSQL + Redis + RabbitMQ) |
| `make db` | Start PostgreSQL only |
| `make redis` | Start Redis only |
| `make rabbitmq` | Start RabbitMQ only |
| `make db-stop` | Stop all Docker services |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back one migration |
| `make migrate-status` | Show current migration version |
| `make migrate-new` | Create a new migration file pair |
| `make clean` | Remove `bin/` and `logs/` |

---

## Project structure

```
.
├── main.go                        Entry point — wires config, logger, and server
├── fs_prod.go                     go:embed for web/dist + migrations (!dev build tag)
├── fs_dev.go                      Live disk reads for web/dist + migrations (dev build tag)
├── config.yaml                    Static, non-secret configuration
├── .env.example                   Environment variable template
├── sqlc.yaml                      SQLC code generation config
├── Makefile                       All dev and build commands
├── .air.toml                      Hot reload config (passes -tags dev)
│
├── server/                        All Go backend code
│   ├── server.go                  HTTP server wiring, startup, graceful shutdown
│   │
│   ├── config/
│   │   └── config.go              Config loader (YAML + env overrides)
│   │
│   ├── pkg/                       Shared utilities — no business logic
│   │   ├── logger.go              Dual-output structured logger (file + stdout)
│   │   ├── jwt.go                 JWT sign / verify + StandardClaims helper
│   │   ├── password.go            Argon2id hash + constant-time compare
│   │   └── env.go                 GetEnv helper
│   │
│   └── internal/
│       ├── core/
│       │   ├── cache/
│       │   │   └── redis.go       Redis client + Cache interface
│       │   ├── events/
│       │   │   ├── bus.go         In-process typed domain event bus
│       │   │   ├── audit.event.go AuditLog event payload + emitter/listener
│       │   │   ├── user.event.go  UserRegistered event payload + emitter/listener
│       │   │   └── job.event.go   JobEnqueued event payload + emitter/listener
│       │   ├── queue/
│       │   │   └── queue.go       RabbitMQ manager — publish + consume with retry
│       │   └── realtime/
│       │       └── hub.go         WebSocket hub — broadcast domain events to clients
│       │
│       ├── db/
│       │   ├── db.go              pgxpool connection factory
│       │   ├── migrations/        SQL migration files (*.up.sql / *.down.sql)
│       │   ├── queries/           SQLC query definitions (*.sql)
│       │   └── repository/        SQLC-generated Go code (do not edit manually)
│       │       └── helpers.go     Hand-written helpers (StringToUUID, etc.)
│       │
│       ├── middlewares/
│       │   ├── requestid_middleware.go  UUID per request → X-Request-ID header
│       │   ├── logger_middleware.go     HTTP access log
│       │   ├── cors_middleware.go       CORS headers from config.yaml
│       │   ├── timeout_middleware.go    Per-request deadline
│       │   ├── auth_middleware.go       JWT Guard + OptionalGuard + UIDFromContext
│       │   ├── ratelimit_middleware.go  In-memory per-IP rate limiter
│       │   ├── validate_middleware.go   Generic body parser + validator
│       │   └── context.go              Context key helpers
│       │
│       ├── modules/
│       │   ├── routes.go          Central registry — mount modules here
│       │   ├── health/
│       │   │   └── health.routes.go  GET /health — DB ping, Redis, memory, uptime
│       │   └── auth/
│       │       ├── auth.schema.go    Input types + validation
│       │       ├── auth.service.go   Business logic returning ApiResponse
│       │       └── auth.routes.go    Router + handler functions
│       │
│       └── utils/
│           ├── http.go            ApiResponse, SendResponse, HttpWriter, ParseBody
│           └── utils.go           GetEnv, GetIP helpers
│
├── web/                           Astro frontend
│   ├── src/
│   │   ├── pages/                 Astro pages (file-based routing)
│   │   ├── components/            Astro/UI components
│   │   └── layouts/               Page layouts
│   ├── public/                    Static assets (copied as-is to dist/)
│   ├── astro.config.mjs           Astro configuration
│   └── package.json
│
├── scripts/
│   └── rename-module.sh           Rename the Go module path across the whole project
│
└── docker/
    └── docker-compose.yaml        PostgreSQL, Redis, RabbitMQ for local development
```

---

## API endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/health` | — | Server health, DB ping, Redis status, memory, uptime |
| `POST` | `/api/v1/auth/register` | — | Register a new user |
| `POST` | `/api/v1/auth/login` | — | Login, returns access + refresh tokens |
| `GET` | `/api/v1/auth/me` | Bearer | Current authenticated user |
| `POST` | `/api/v1/auth/refresh` | — | Exchange refresh token for new access token |
| `GET` | `/ws` | — | WebSocket endpoint |
| `GET` | `/*` | — | Astro frontend (SPA fallback to index.html) |

---

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `DB_URL` | *(built from config.yaml)* | Full PostgreSQL DSN — overrides individual DB fields |
| `JWT_SECRET` | `change-me-in-production` | Access token signing secret |
| `JWT_REFRESH_SECRET` | `change-me-refresh-secret` | Refresh token signing secret |
| `AMQP_URL` | `amqp://guest:guest@localhost:5672/` | RabbitMQ connection URL |
| `REDIS_ADDR` | `localhost:6379` | Redis address in `host:port` form |
| `REDIS_PASSWORD` | *(empty)* | Redis password |
| `CORS_ORIGINS` | *(from config.yaml)* | Comma-separated allowed origins |
| `PORT` | `8080` | HTTP listen port — overrides config.yaml |
| `HOST` | `localhost` | HTTP bind address — overrides config.yaml |
| `ENV` | `development` | Environment name — overrides config.yaml |

---

## Adding a new API module

1. Create the module directory under `server/internal/modules/<name>/`:

```
server/internal/modules/posts/
  posts.schema.go    — input types + Validate() methods
  posts.service.go   — business logic returning utils.ApiResponse
  posts.routes.go    — RegisterRoutes() + handler functions
```

2. Write SQL queries in `server/internal/db/queries/posts.sql`, then run `make sqlc-gen`.

3. Register in `server/internal/modules/routes.go` — one line:

```go
func Register(apiRouter *mux.Router, pool *pgxpool.Pool, redis cache.Cache, bus *events.Bus, startTime time.Time) {
    health.RegisterRoutes(apiRouter, pool, redis, startTime)
    auth.RegisterRoutes(apiRouter, pool, bus)
    posts.RegisterRoutes(apiRouter, pool, bus) // ← add here
}
```

See [WORKFLOW.md](./WORKFLOW.md) for a full walkthrough with code examples.

---

## Disabling optional subsystems

Every optional subsystem can be turned off in `config.yaml` without touching Go code:

```yaml
redis:
  enabled: false          # no REDIS_ADDR required

realtime:
  websocket:
    enabled: false        # no WebSocket endpoint

queue:
  rabbitmq:
    enabled: false        # no AMQP_URL required

workers:
  process:
    enabled: false        # worker goroutines never start
```

---

## Migrations

```bash
make migrate-new     # prompts for a name, creates the .up.sql and .down.sql pair
make migrate-up      # apply all pending migrations
make migrate-down    # roll back one migration
make migrate-status  # show current version
```

Migration files live in `server/internal/db/migrations/` and are embedded into the production binary automatically.
