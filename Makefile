## ── Project metadata ──────────────────────────────────────────────────────────
APP_NAME    := app
CMD_PATH    := .
BIN_DIR     := ./bin
BIN_FILE    := $(BIN_DIR)/$(APP_NAME)
LOGS_DIR    := ./logs
WEB_DIR     := ./web

## ── Tools ─────────────────────────────────────────────────────────────────────
GO              := go
SQLC            := sqlc
GOLANGCI_LINT   := golangci-lint
MIGRATE         := migrate
AIR             := air

## ── DB config (override via env or .env) ──────────────────────────────────────
DB_URL          ?= postgres://postgres:postgres@localhost:5432/app_db?sslmode=disable
MIGRATIONS_DIR  ?= server/internal/db/migrations

## ── Flags ─────────────────────────────────────────────────────────────────────
GO_FILES := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

.DEFAULT_GOAL := build

.PHONY: help
help:
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Tooling:"
	@printf "  %-18s %s\n" "install" "Install all dev tools"
	@printf "  %-18s %s\n" "install-sqlc" "Install sqlc"
	@printf "  %-18s %s\n" "install-air" "Install air (hot reload)"
	@printf "  %-18s %s\n" "install-migrate" "Install golang-migrate"
	@printf "  %-18s %s\n" "install-lint" "Install golangci-lint"
	@echo ""
	@echo "Build & run:"
	@printf "  %-18s %s\n" "build" "Build frontend + backend binary"
	@printf "  %-18s %s\n" "build-web" "Build Astro frontend only"
	@printf "  %-18s %s\n" "build-server" "Build Go backend binary only"
	@printf "  %-18s %s\n" "run" "Build then run the app"
	@printf "  %-18s %s\n" "dev" "Run with hot reload via air"
	@echo ""
	@echo "Quality:"
	@printf "  %-18s %s\n" "test" "Run tests (race + coverage)"
	@printf "  %-18s %s\n" "lint" "Run golangci-lint"
	@printf "  %-18s %s\n" "fmt" "Format Go code"
	@echo ""
	@echo "Codegen:"
	@printf "  %-18s %s\n" "sqlc-gen" "Generate code from SQL"
	@echo ""
	@echo "Infrastructure:"
	@printf "  %-18s %s\n" "db" "Start PostgreSQL"
	@printf "  %-18s %s\n" "rabbitmq" "Start RabbitMQ"
	@printf "  %-18s %s\n" "redis" "Start Redis"
	@printf "  %-18s %s\n" "infra" "Start PostgreSQL + RabbitMQ + Redis"
	@printf "  %-18s %s\n" "db-stop" "Stop all infrastructure"
	@echo ""
	@echo "Migrations:"
	@printf "  %-18s %s\n" "migrate-up" "Apply all pending migrations"
	@printf "  %-18s %s\n" "migrate-down" "Rollback last migration"
	@printf "  %-18s %s\n" "migrate-status" "Show migration status"
	@printf "  %-18s %s\n" "migrate-drop" "Drop all migrations (destructive)"
	@printf "  %-18s %s\n" "migrate-new" "Create a new migration (prompts for name)"
	@echo ""
	@echo "Cleanup:"
	@printf "  %-18s %s\n" "clean" "Remove logs and build artifacts"
	@printf "  %-18s %s\n" "clean-bin" "Remove build artifacts only"
	@echo ""
	@echo "Notes:"
	@echo "  Override DB_URL and MIGRATIONS_DIR for migration commands."
	@echo "  Example: make migrate-up DB_URL=postgres://user:pass@localhost:5432/db?sslmode=disable"

# ── Install dev tools ──────────────────────────────────────────────────────────
.PHONY: install
install: install-sqlc install-air install-migrate install-lint
	@echo ">> All tools installed."

.PHONY: install-sqlc
install-sqlc:
	@echo ">> Installing sqlc…"
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

.PHONY: install-air
install-air:
	@echo ">> Installing air (hot reload)…"
	@go install github.com/air-verse/air@latest

.PHONY: install-migrate
install-migrate:
	@echo ">> Installing golang-migrate…"
	@go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest

.PHONY: install-lint
install-lint:
	@echo ">> Installing golangci-lint…"
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
		| sh -s -- -b $(go env GOPATH)/bin v2.11.4

# ── Build ──────────────────────────────────────────────────────────────────────
# Full build: compile the frontend first, then embed it into the Go binary.
.PHONY: build
build: build-web build-server
	@echo ">> Single binary ready: $(BIN_FILE)"

.PHONY: build-server
build-server: clean-bin
	@echo ">> Building Go binary…"
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_FILE) $(CMD_PATH)
	@echo ">> Binary: $(BIN_FILE)"

.PHONY: build-web
build-web:
	@echo ">> Building Astro frontend…"
	@cd $(WEB_DIR) && bun run build
	@echo ">> Frontend built → web/dist/"

# ── Run ────────────────────────────────────────────────────────────────────────
.PHONY: run
run: build
	@echo ">> Running $(APP_NAME)…"
	$(BIN_FILE)

# ── Dev (hot reload via Air) ───────────────────────────────────────────────────
.PHONY: dev
dev:
	@$(AIR)

# ── Test ───────────────────────────────────────────────────────────────────────
.PHONY: test
test:
	@echo ">> Running tests…"
	$(GO) test ./... -v -race -cover

# ── Lint & Format ──────────────────────────────────────────────────────────────
.PHONY: lint
lint:
	@echo ">> Linting…"
	-$(GOLANGCI_LINT) run ./... || true

.PHONY: fmt
fmt:
	@echo ">> Formatting…"
	$(GO) fmt ./...
	@echo ">> Running GFMT in hidden mode"
	@gofmt -s -w $(GO_FILES)

# ── SQLC code generation ───────────────────────────────────────────────────────
.PHONY: sqlc-gen
sqlc-gen:
	@echo ">> Generating repository code from SQL…"
	$(SQLC) generate

# ── Database ───────────────────────────────────────────────────────────────────
.PHONY: db
db:
	@echo ">> Starting PostgreSQL…"
	@docker compose -f docker/docker-compose.yaml up -d postgres

.PHONY: rabbitmq
rabbitmq:
	@echo ">> Starting RabbitMQ…"
	@docker compose -f docker/docker-compose.yaml up -d rabbitmq

.PHONY: redis
redis:
	@echo ">> Starting Redis…"
	@docker compose -f docker/docker-compose.yaml up -d redis

.PHONY: infra
infra:
	@echo ">> Starting all infrastructure (PostgreSQL + RabbitMQ + Redis)…"
	@docker compose -f docker/docker-compose.yaml up -d

.PHONY: db-stop
db-stop:
	@echo ">> Stopping all infrastructure…"
	@docker compose -f docker/docker-compose.yaml down

# ── Migrations ─────────────────────────────────────────────────────────────────
.PHONY: migrate-up
migrate-up:
	@echo ">> Applying all pending migrations…"
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

.PHONY: migrate-down
migrate-down:
	@echo ">> Rolling back last migration…"
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down 1

.PHONY: migrate-status
migrate-status:
	@echo ">> Migration status…"
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version

.PHONY: migrate-drop
migrate-drop:
	@echo ">> Dropping all migrations (DESTRUCTIVE)…"
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" drop -f

.PHONY: migrate-new
migrate-new:
	@read -p "Migration name: " name; \
		$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq "$$name"; \
		echo ">> Created migration: $$name"

# ── Clean ──────────────────────────────────────────────────────────────────────
.PHONY: clean
clean: clean-bin
	@echo ">> Removing log files…"
	@rm -rf $(LOGS_DIR)

.PHONY: clean-bin
clean-bin:
	@echo ">> Cleaning build artifacts…"
	@rm -rf $(BIN_DIR)
