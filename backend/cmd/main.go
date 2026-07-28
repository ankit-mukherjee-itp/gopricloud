package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"backend/cmd/server"
	"backend/configs"
	"backend/internal/adapters/handlers/rest"
	"backend/internal/adapters/providers/openstack"
	"backend/internal/adapters/repositories/postgres"
	"backend/internal/core/services"
	"backend/internal/core/token"
)

func init() {
	configs.LoadEnv()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run is the composition root: it constructs the concrete adapters, injects
// them into the core services through their ports, and hands the resulting
// handler to the server.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := configs.Load()
	if err != nil {
		return err
	}

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	// Outbound adapters.
	userRepo := postgres.NewUserRepository(db)
	tokenRepo := postgres.NewRefreshTokenRepository(db)
	computeRepo := postgres.NewComputeRepository(db)
	computeProvider := openstack.NewProvider(cfg.OSCloudName)

	// Core.
	jwtManager := token.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL)
	authService := services.NewAuthUsecase(userRepo, tokenRepo, jwtManager, cfg.RefreshTokenTTL)
	computeService := services.NewComputeUsecase(computeRepo, computeProvider)

	// Inbound adapters.
	authHandler := rest.NewAuthHandler(authService)
	testHandler := rest.NewTestHandler()
	computeHandler := rest.NewComputeHandler(computeService)
	router := rest.NewRouter(authHandler, testHandler, computeHandler, jwtManager)

	return server.Serve(ctx, router, cfg.Port)
}
