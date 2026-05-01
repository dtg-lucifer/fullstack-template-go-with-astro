package middlewares

import "context"

// validatedBodyKey is the unexported context key for the validated request body.
// Using a private struct type prevents key collisions with other packages.
type validatedBodyKey struct{}

// contextWithBody stores a validated body value in the context.
func contextWithBody(ctx context.Context, v any) context.Context {
	return context.WithValue(ctx, validatedBodyKey{}, v)
}
