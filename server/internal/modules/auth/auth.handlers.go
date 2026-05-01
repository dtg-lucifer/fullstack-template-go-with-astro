package auth

import (
	"net/http"

	"github.com/your-username/go-mux-backend-template/server/internal/middlewares"
	"github.com/your-username/go-mux-backend-template/server/internal/utils"
)

// ---- Auth handlers -------------------------------------------------------------------

func register(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := middlewares.BodyFromContext[RegisterInput](r.Context())
		utils.SendResponse(w, svc.Register(r.Context(), input, r))
	}
}

func login(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := middlewares.BodyFromContext[LoginInput](r.Context())
		utils.SendResponse(w, svc.Login(r.Context(), input, r))
	}
}

func me(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := middlewares.UIDFromContext(r.Context())
		utils.SendResponse(w, svc.Me(r.Context(), uid))
	}
}

func refresh(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := middlewares.BodyFromContext[RefreshInput](r.Context())
		utils.SendResponse(w, svc.RefreshToken(r.Context(), input))
	}
}
