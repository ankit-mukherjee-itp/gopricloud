package dto

import (
	"time"

	"backend/internal/core/services"
)

type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SigninRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type AuthResponse struct {
	User                  UserResponse `json:"user"`
	AccessToken           string       `json:"access_token"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
}

// NewAuthResponse maps a services.AuthResult onto the wire representation.
func NewAuthResponse(result *services.AuthResult) AuthResponse {
	return AuthResponse{
		User: UserResponse{
			ID:    result.User.ID.String(),
			Name:  result.User.Name,
			Email: result.User.Email,
		},
		AccessToken:           result.AccessToken,
		AccessTokenExpiresAt:  result.AccessTokenExpiresAt,
		RefreshToken:          result.RefreshToken,
		RefreshTokenExpiresAt: result.RefreshTokenExpiresAt,
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}
