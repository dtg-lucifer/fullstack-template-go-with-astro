package auth

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	coreutils "github.com/your-username/go-mux-backend-template/server/internal/core/utils"
	"github.com/your-username/go-mux-backend-template/server/internal/db/repository"
	"github.com/your-username/go-mux-backend-template/server/internal/utils"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// ── Service interface ──────────────────────────────────────────────────────────
//
// Declaring an interface for the service layer enables two things:
//  1. The Controller depends on the interface, not the concrete type — making
//     it easy to swap implementations (e.g. a mock in tests).
//  2. WithDebug() can return a proxy that also satisfies the interface, giving
//     us transparent method-level debug logging without touching handler code.

// ServiceIface is the contract that AuthController depends on.
// Every method mirrors the concrete Service method signature exactly.
type ServiceIface interface {
	Register(ctx context.Context, input RegisterInput, r *http.Request) utils.ApiResponse
	Login(ctx context.Context, input LoginInput, r *http.Request) utils.ApiResponse
	Me(ctx context.Context, uid string) utils.ApiResponse
	RefreshToken(ctx context.Context, input RefreshInput) utils.ApiResponse
}

// ── Concrete service ───────────────────────────────────────────────────────────

// Service holds the dependencies needed by all auth business logic.
// It satisfies ServiceIface.
type Service struct {
	repo *repository.Queries // sqlc-generated data access layer
	bus  *events.Bus
}

// NewService creates a Service backed by the sqlc repository.
// Use WithDebug instead of NewService when you want method-level debug logging.
func NewService(pool *pgxpool.Pool, bus *events.Bus) *Service {
	return &Service{
		repo: repository.New(pool),
		bus:  bus,
	}
}

// WithDebug wraps a new Service in a debug-logging proxy and returns it as
// ServiceIface. Every method call will emit structured DEBUG log lines:
//
//	[AuthService.Register] --> START
//	[AuthService.Register] <-- END   duration=3ms
//
// Drop-in replacement for NewService — the Controller always calls WithDebug.
func WithDebug(pool *pgxpool.Pool, bus *events.Bus, logger *pkg.Logger) ServiceIface {
	svc := NewService(pool, bus)
	d := coreutils.NewDispatcher(svc, "AuthService", logger)
	return &serviceDebugProxy{svc: svc, d: d}
}

// ── Debug proxy ────────────────────────────────────────────────────────────────

// serviceDebugProxy implements ServiceIface by forwarding every call through
// the Dispatcher, which handles timing and structured logging automatically.
type serviceDebugProxy struct {
	svc *Service
	d   *coreutils.Dispatcher
}

func (p *serviceDebugProxy) Register(ctx context.Context, input RegisterInput, r *http.Request) utils.ApiResponse {
	results := p.d.Call("Register", ctx, input, r)
	return results[0].Interface().(utils.ApiResponse)
}

func (p *serviceDebugProxy) Login(ctx context.Context, input LoginInput, r *http.Request) utils.ApiResponse {
	results := p.d.Call("Login", ctx, input, r)
	return results[0].Interface().(utils.ApiResponse)
}

func (p *serviceDebugProxy) Me(ctx context.Context, uid string) utils.ApiResponse {
	results := p.d.Call("Me", ctx, uid)
	return results[0].Interface().(utils.ApiResponse)
}

func (p *serviceDebugProxy) RefreshToken(ctx context.Context, input RefreshInput) utils.ApiResponse {
	results := p.d.Call("RefreshToken", ctx, input)
	return results[0].Interface().(utils.ApiResponse)
}

// ── Business logic ─────────────────────────────────────────────────────────────

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, input RegisterInput, r *http.Request) utils.ApiResponse {
	existing, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err == nil && existing.ID.Valid {
		return utils.ApiError("email already registered", nil, 409)
	}

	hashed, err := pkg.HashPassword(input.Password)
	if err != nil {
		return utils.ApiError("failed to process password", err.Error(), 500)
	}

	user, err := s.repo.CreateUser(ctx, repository.CreateUserParams{
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Email:     input.Email,
		Password:  hashed,
	})
	if err != nil {
		return utils.ApiError("failed to create user", err.Error(), 500)
	}

	if s.bus != nil {
		s.bus.EmitUserRegistered(events.UserRegisteredPayload{
			UserID: user.ID.String(),
			Email:  user.Email,
		})
		s.bus.EmitAuditLog(events.AuditLogPayload{
			ActorUserID: user.ID.String(),
			Action:      "register",
			Entity:      "user",
			IP:          utils.GetIP(r),
			UserAgent:   r.UserAgent(),
		})
	}

	return utils.ApiSuccess("user registered successfully", map[string]any{
		"user": safeUser(user),
	}, 201)
}

// Login authenticates a user and returns access + refresh tokens.
func (s *Service) Login(ctx context.Context, input LoginInput, r *http.Request) utils.ApiResponse {
	user, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return utils.ApiError("invalid credentials", nil, 401)
	}

	if err := pkg.ComparePassword(user.Password, input.Password); err != nil {
		return utils.ApiError("invalid credentials", nil, 401)
	}

	accessToken, err := pkg.NewTokenSigner(getenv("JWT_SECRET", "change-me")).
		Sign(pkg.StandardClaims(user.ID.String(), 24*time.Hour))
	if err != nil {
		return utils.ApiError("failed to generate access token", err.Error(), 500)
	}

	refreshToken, err := pkg.NewTokenSigner(getenv("JWT_REFRESH_SECRET", "change-me-refresh")).
		Sign(pkg.StandardClaims(user.ID.String(), 30*24*time.Hour))
	if err != nil {
		return utils.ApiError("failed to generate refresh token", err.Error(), 500)
	}

	if s.bus != nil {
		s.bus.EmitAuditLog(events.AuditLogPayload{
			ActorUserID: user.ID.String(),
			Action:      "login",
			Entity:      "user",
			IP:          utils.GetIP(r),
			UserAgent:   r.UserAgent(),
		})
	}

	return utils.ApiSuccess("login successful", map[string]any{
		"user":          safeUser(user),
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	}, 200)
}

// Me returns the current authenticated user.
func (s *Service) Me(ctx context.Context, uid string) utils.ApiResponse {
	pgUID, err := repository.StringToUUID(uid)
	if err != nil {
		return utils.ApiError("invalid user id", err.Error(), 400)
	}

	user, err := s.repo.GetUserByID(ctx, pgUID)
	if err != nil {
		return utils.ApiError("user not found", nil, 404)
	}

	return utils.ApiSuccess("ok", map[string]any{"user": safeUser(user)}, 200)
}

// RefreshToken validates a refresh token and issues a new access token.
func (s *Service) RefreshToken(_ context.Context, input RefreshInput) utils.ApiResponse {
	claims, err := pkg.NewTokenSigner(getenv("JWT_REFRESH_SECRET", "change-me-refresh")).
		Verify(input.RefreshToken)
	if err != nil {
		return utils.ApiError("invalid or expired refresh token", nil, 401)
	}

	uid, ok := claims["uid"].(string)
	if !ok || uid == "" {
		return utils.ApiError("malformed refresh token", nil, 401)
	}

	newToken, err := pkg.NewTokenSigner(getenv("JWT_SECRET", "change-me")).
		Sign(pkg.StandardClaims(uid, 24*time.Hour))
	if err != nil {
		return utils.ApiError("failed to generate access token", err.Error(), 500)
	}

	return utils.ApiSuccess("token refreshed", map[string]any{"access_token": newToken}, 200)
}

// ── Helpers ────────────────────────────────────────────────────────────────────

func safeUser(u repository.User) map[string]any {
	return map[string]any{
		"id":         u.ID,
		"first_name": u.FirstName,
		"last_name":  u.LastName,
		"email":      u.Email,
		"verified":   u.Verified,
		"created_at": u.CreatedAt,
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
