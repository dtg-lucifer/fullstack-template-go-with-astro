package middlewares

import (
	"context"
	"net/http"
	"strings"

	"github.com/your-username/go-mux-backend-template/server/internal/utils"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// contextKey is an unexported type for context keys to avoid collisions with other packages.
type contextKey string

// ContextKeyUID is the context key under which the authenticated user's UUID is stored.
const ContextKeyUID contextKey = "uid"

// AuthMiddleware validates JWT tokens from the Authorization header or a "jwt" cookie.
// On success it injects the user's UID into the request context under ContextKeyUID.
// On failure it returns 401 Unauthorized with a JSON body.
type AuthMiddleware struct{}

// NewAuthMiddleware creates an AuthMiddleware.
func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

// Guard is the middleware function. Wrap protected routes with this.
func (am *AuthMiddleware) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := am.extractToken(r)
		if token == "" {
			utils.NewHttpWriter(w, r).Status(http.StatusUnauthorized).JSON(utils.M{
				"success": false,
				"message": "missing or malformed token",
			})
			return
		}

		secret := utils.GetEnv("JWT_SECRET", "change-me-in-production")
		signer := pkg.NewTokenSigner(secret)

		claims, err := signer.Verify(token)
		if err != nil {
			utils.NewHttpWriter(w, r).Status(http.StatusUnauthorized).JSON(utils.M{
				"success": false,
				"message": "invalid or expired token",
			})
			return
		}

		uid, ok := claims["uid"].(string)
		if !ok || uid == "" {
			utils.NewHttpWriter(w, r).Status(http.StatusUnauthorized).JSON(utils.M{
				"success": false,
				"message": "token is missing uid claim",
			})
			return
		}

		ctx := context.WithValue(r.Context(), ContextKeyUID, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalGuard is like Guard but does not reject the request if no token is present.
// Useful for endpoints that behave differently for authenticated vs anonymous users.
func (am *AuthMiddleware) OptionalGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := am.extractToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		secret := utils.GetEnv("JWT_SECRET", "change-me-in-production")
		signer := pkg.NewTokenSigner(secret)

		claims, err := signer.Verify(token)
		if err != nil {
			// Token present but invalid — still let the request through without a UID
			next.ServeHTTP(w, r)
			return
		}

		if uid, ok := claims["uid"].(string); ok && uid != "" {
			ctx := context.WithValue(r.Context(), ContextKeyUID, uid)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// extractToken tries the "jwt" cookie first, then falls back to the Authorization header.
func (am *AuthMiddleware) extractToken(r *http.Request) string {
	if c, err := r.Cookie("jwt"); err == nil {
		return c.Value
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// UIDFromContext retrieves the authenticated user's UID from the request context.
// Returns an empty string if the UID is not present (unauthenticated request).
func UIDFromContext(ctx context.Context) string {
	uid, _ := ctx.Value(ContextKeyUID).(string)
	return uid
}
