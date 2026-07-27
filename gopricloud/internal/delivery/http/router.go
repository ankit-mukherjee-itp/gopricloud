package http

import (
	"net/http"

	"gopricloud/gopricloud/internal/delivery/http/handler"
	"gopricloud/gopricloud/internal/delivery/http/middleware"
	"gopricloud/gopricloud/internal/token"
)

// NewRouter wires the public auth endpoints and the JWT-protected /test and
// /instances endpoints onto a stdlib ServeMux.
func NewRouter(
	authHandler *handler.AuthHandler,
	testHandler *handler.TestHandler,
	computeHandler *handler.ComputeHandler,
	jwtManager *token.JWTManager,
) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /signup", authHandler.Signup)
	mux.HandleFunc("POST /signin", authHandler.Signin)
	mux.HandleFunc("POST /refresh", authHandler.Refresh)

	requireAuth := middleware.Auth(jwtManager)
	mux.Handle("GET /test", requireAuth(http.HandlerFunc(testHandler.Test)))

	mux.Handle("POST /instances", requireAuth(http.HandlerFunc(computeHandler.Create)))
	mux.Handle("GET /instances", requireAuth(http.HandlerFunc(computeHandler.List)))
	mux.Handle("GET /instances/{id}", requireAuth(http.HandlerFunc(computeHandler.Get)))
	mux.Handle("DELETE /instances/{id}", requireAuth(http.HandlerFunc(computeHandler.Delete)))

	return mux
}
