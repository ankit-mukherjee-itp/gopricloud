package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"gopricloud/gopricloud/internal/delivery/http/dto"
	"gopricloud/gopricloud/internal/delivery/http/httpx"
	"gopricloud/gopricloud/internal/domain"
	"gopricloud/gopricloud/internal/usecase"
)

type AuthHandler struct {
	auth *usecase.AuthUsecase
}

func NewAuthHandler(auth *usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Signup handles POST /signup.
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req dto.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Name == "" || req.Email == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "name, email and password are required")
		return
	}
	if len(req.Password) < 8 {
		httpx.WriteError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	result, err := h.auth.Signup(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			httpx.WriteError(w, http.StatusConflict, "a user with this email already exists")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, dto.NewAuthResponse(result))
}

// Signin handles POST /signin.
func (h *AuthHandler) Signin(w http.ResponseWriter, r *http.Request) {
	var req dto.SigninRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	result, err := h.auth.Signin(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not sign in")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, dto.NewAuthResponse(result))
}

// Refresh handles POST /refresh. It exchanges a valid refresh token (issued
// at signup/signin) for a new access token, rotating the refresh token in
// the same call.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	result, err := h.auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRefreshToken) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not refresh session")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, dto.NewAuthResponse(result))
}
