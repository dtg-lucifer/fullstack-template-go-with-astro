package auth

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	"github.com/your-username/go-mux-backend-template/server/internal/db/repository"
	"github.com/your-username/go-mux-backend-template/server/internal/utils"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// Service holds the dependencies needed by all auth business logic.
type Service struct {
	repo *repository.Queries
	pool *pgxpool.Pool
	bus  *events.Bus
}

func newService(pool *pgxpool.Pool, bus *events.Bus) *Service {
	return &Service{
		repo: repository.New(pool),
		pool: pool,
		bus:  bus,
	}
}

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
