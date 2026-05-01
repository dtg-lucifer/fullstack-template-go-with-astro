// Package auth is the self-contained module for user registration, login, and token management.
// Each file has a single responsibility:
//   - schema.go  — input types and Validate() rules
//   - service.go — business logic returning utils.ApiResponse
//   - routes.go  — router wiring and controllers
package auth

import (
	"errors"
	"strings"
)

// RegisterInput holds validated fields for the register endpoint.
type RegisterInput struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

// Validate returns an error if any required field is missing or malformed.
func (i *RegisterInput) Validate() error {
	i.Email = strings.TrimSpace(strings.ToLower(i.Email))
	i.FirstName = strings.TrimSpace(i.FirstName)
	i.LastName = strings.TrimSpace(i.LastName)

	if i.FirstName == "" {
		return errors.New("first_name is required")
	}
	if i.Email == "" || !strings.Contains(i.Email, "@") {
		return errors.New("a valid email is required")
	}
	if len(i.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

// LoginInput holds validated fields for the login endpoint.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Validate returns an error if any required field is missing.
func (i *LoginInput) Validate() error {
	i.Email = strings.TrimSpace(strings.ToLower(i.Email))
	if i.Email == "" {
		return errors.New("email is required")
	}
	if i.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

// RefreshInput holds the refresh token sent by the client.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

// Validate returns an error if the refresh token is absent.
func (i *RefreshInput) Validate() error {
	if strings.TrimSpace(i.RefreshToken) == "" {
		return errors.New("refresh_token is required")
	}
	return nil
}
