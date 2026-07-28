package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"backend/internal/adapters/api"
	"backend/internal/core/token"
)

type contextKey string

const UserIDContextKey contextKey = "userID"
const UserEmailContextKey contextKey = "userEmail"

// Auth returns middleware that requires a valid, unexpired "Authorization:
// Bearer <access token>" header, rejecting the request with 401 otherwise.
// On success it stashes the caller's user id and email in the request
// context for downstream handlers.
func Auth(jwtManager *token.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const prefix = "Bearer "

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, prefix) {
				api.WriteError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			raw := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
			claims, err := jwtManager.ParseAccessToken(raw)
			if err != nil {
				status := http.StatusUnauthorized
				message := "invalid access token"
				if errors.Is(err, token.ErrExpiredAccessToken) {
					message = "access token expired"
				}
				api.WriteError(w, status, message)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDContextKey, claims.Subject)
			ctx = context.WithValue(ctx, UserEmailContextKey, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
