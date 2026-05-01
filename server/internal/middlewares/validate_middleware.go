package middlewares

import (
	"net/http"

	"github.com/your-username/go-mux-backend-template/server/internal/utils"
)

// Validatable is the interface any request input struct must satisfy to be used
// with the Validate middleware. Implement Validate() on your schema types in schema.go.
type Validatable interface {
	Validate() error
}

// Validate returns a middleware that:
//  1. Decodes the JSON request body into a new *T
//  2. Calls (*T).Validate() — your field-level rules live there
//  3. Stores the dereferenced value in the request context so the handler can
//     retrieve it with BodyFromContext[T]
//  4. Calls the next handler on success, or returns:
//     - 400 Bad Request on a malformed / missing body
//     - 422 Unprocessable Entity on a validation failure
//
// T must be a struct type whose pointer implements Validatable.
//
// Usage in routes.go:
//
//	sub.Handle("/register",
//	    middlewares.Validate[RegisterInput](http.HandlerFunc(register(svc))),
//	).Methods(http.MethodPost)
//
// Usage in the handler:
//
//	input := middlewares.BodyFromContext[RegisterInput](r.Context())
func Validate[T any, PT interface {
	*T
	Validatable
}](next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := PT(new(T))

		if err := utils.ParseBody(r, input); err != nil {
			utils.SendResponse(w, utils.ApiError(err.Error(), nil, http.StatusBadRequest))
			return
		}

		if err := input.Validate(); err != nil {
			utils.SendResponse(w, utils.ApiError(err.Error(), nil, http.StatusUnprocessableEntity))
			return
		}

		ctx := contextWithBody(r.Context(), *input)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BodyFromContext retrieves the validated body stored by the Validate middleware.
// Returns the zero value of T if nothing was stored (e.g. on routes without Validate).
func BodyFromContext[T any](ctx interface{ Value(any) any }) T {
	v, _ := ctx.Value(validatedBodyKey{}).(T)
	return v
}
