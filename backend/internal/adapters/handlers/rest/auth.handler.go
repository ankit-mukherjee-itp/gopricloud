package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"backend/internal/adapters/api"
	"backend/internal/adapters/handlers/rest/dto"
	"backend/internal/core/domain"
	"backend/internal/core/services"
)

type AuthHandler struct {
	auth *services.AuthUsecase
}

func NewAuthHandler(auth *services.AuthUsecase) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Signup handles POST /signup.
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req dto.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Name == "" || req.Email == "" || req.Password == "" {
		api.WriteError(w, http.StatusBadRequest, "name, email and password are required")
		return
	}
	if len(req.Password) < 8 {
		api.WriteError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	result, err := h.auth.Signup(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			api.WriteError(w, http.StatusConflict, "a user with this email already exists")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	api.WriteJSON(w, http.StatusCreated, dto.NewAuthResponse(result))
}

// Signin handles POST /signin.
func (h *AuthHandler) Signin(w http.ResponseWriter, r *http.Request) {
	var req dto.SigninRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		api.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	result, err := h.auth.Signin(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			api.WriteError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "could not sign in")
		return
	}

	api.WriteJSON(w, http.StatusOK, dto.NewAuthResponse(result))
}

// Refresh handles POST /refresh. It exchanges a valid refresh token (issued
// at signup/signin) for a new access token, rotating the refresh token in
// the same call.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		api.WriteError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	result, err := h.auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRefreshToken) {
			api.WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "could not refresh session")
		return
	}

	api.WriteJSON(w, http.StatusOK, dto.NewAuthResponse(result))
}
