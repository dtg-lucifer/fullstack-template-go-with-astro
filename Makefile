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
help: ## Show this help
	@echo "Usage: make <target>"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ── Install dev tools ──────────────────────────────────────────────────────────
.PHONY: install
install: install-sqlc install-air install-migrate install-lint ## Install all dev tools
	@echo ">> All tools installed."

.PHONY: install-sqlc
install-sqlc: ## Install sqlc
	@echo ">> Installing sqlc…"
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

.PHONY: install-air
install-air: ## Install air (hot reload)
	@echo ">> Installing air (hot reload)…"
	@go install github.com/air-verse/air@latest

.PHONY: install-migrate
install-migrate: ## Install golang-migrate
	@echo ">> Installing golang-migrate…"
	@go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest

.PHONY: install-lint
install-lint: ## Install golangci-lint
	@echo ">> Installing golangci-lint…"
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
		| sh -s -- -b $(go env GOPATH)/bin v2.11.4

# ── Build ──────────────────────────────────────────────────────────────────────
# Full build: compile the frontend first, then embed it into the Go binary.
.PHONY: build
build: build-web build-server ## Build frontend + backend binary
	@echo ">> Single binary ready: $(BIN_FILE)"

.PHONY: build-server
build-server: clean-bin ## Build Go backend binary
	@echo ">> Building Go binary…"
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_FILE) $(CMD_PATH)
	@echo ">> Binary: $(BIN_FILE)"

.PHONY: build-web
build-web: ## Build Astro frontend
	@echo ">> Building Astro frontend…"
	@cd $(WEB_DIR) && bun run build
	@echo ">> Frontend built → web/dist/"

# ── Run ────────────────────────────────────────────────────────────────────────
.PHONY: run
run: build ## Build then run the app
	@echo ">> Running $(APP_NAME)…"
	$(BIN_FILE)

# ── Dev (hot reload via Air) ───────────────────────────────────────────────────
.PHONY: dev
dev: ## Run with hot reload via air
	@$(AIR)

# ── Test ───────────────────────────────────────────────────────────────────────
.PHONY: test
test: ## Run tests (race + coverage)
	@echo ">> Running tests…"
	$(GO) test ./... -v -race -cover

# ── Lint & Format ──────────────────────────────────────────────────────────────
.PHONY: lint
lint: ## Run golangci-lint
	@echo ">> Linting…"
	-$(GOLANGCI_LINT) run ./... || true

.PHONY: fmt
fmt: ## Format Go code
	@echo ">> Formatting…"
	$(GO) fmt ./...
	@echo ">> Running GFMT in hidden mode"
	@gofmt -s -w $(GO_FILES)

# ── SQLC code generation ───────────────────────────────────────────────────────
.PHONY: sqlc-gen
sqlc-gen: ## Generate code from SQL
	@echo ">> Generating repository code from SQL…"
	$(SQLC) generate

# ── Database ───────────────────────────────────────────────────────────────────
.PHONY: db
db: ## Start PostgreSQL
	@echo ">> Starting PostgreSQL…"
	@docker compose -f docker/docker-compose.yaml up -d postgres

.PHONY: rabbitmq
rabbitmq: ## Start RabbitMQ
	@echo ">> Starting RabbitMQ…"
	@docker compose -f docker/docker-compose.yaml up -d rabbitmq

.PHONY: redis
redis: ## Start Redis
	@echo ">> Starting Redis…"
	@docker compose -f docker/docker-compose.yaml up -d redis

.PHONY: infra
infra: ## Start PostgreSQL + RabbitMQ + Redis
	@echo ">> Starting all infrastructure (PostgreSQL + RabbitMQ + Redis)…"
	@docker compose -f docker/docker-compose.yaml up -d

.PHONY: db-stop
db-stop: ## Stop all infrastructure
	@echo ">> Stopping all infrastructure…"
	@docker compose -f docker/docker-compose.yaml down

# ── Migrations ─────────────────────────────────────────────────────────────────
.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	@echo ">> Applying all pending migrations…"
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back last migration
	@echo ">> Rolling back last migration…"
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down 1

.PHONY: migrate-status
migrate-status: ## Show migration status
	@echo ">> Migration status…"
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version

.PHONY: migrate-drop
migrate-drop: ## Drop all migrations (destructive)
	@echo ">> Dropping all migrations (DESTRUCTIVE)…"
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" drop -f

.PHONY: migrate-new
migrate-new: ## Create a new migration (prompts for name)
	@read -p "Migration name: " name; \
		$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq "$$name"; \
		echo ">> Created migration: $$name"

# ── Clean ──────────────────────────────────────────────────────────────────────
.PHONY: clean
clean: clean-bin ## Remove logs and build artifacts
	@echo ">> Removing log files…"
	@rm -rf $(LOGS_DIR)

.PHONY: clean-bin
clean-bin: ## Remove build artifacts only
	@echo ">> Cleaning build artifacts…"
	@rm -rf $(BIN_DIR)
