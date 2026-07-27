package main

import (
	"context"
	"github.com/joho/godotenv"
	"gopricloud/gopricloud/internal/config"
	deliveryhttp "gopricloud/gopricloud/internal/delivery/http"
	"gopricloud/gopricloud/internal/delivery/http/handler"
	"gopricloud/gopricloud/internal/infrastructure/postgres"
	"gopricloud/gopricloud/internal/token"
	"gopricloud/gopricloud/internal/usecase"
	"gopricloud/gopricloud/openstack/compute"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error", err)
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	userRepo := postgres.NewUserRepository(db)
	tokenRepo := postgres.NewRefreshTokenRepository(db)
	computeRepo := postgres.NewComputeRepository(db)
	computeProvider := compute.NewProvider(cfg.OSCloudName)

	jwtManager := token.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL)
	authUsecase := usecase.NewAuthUsecase(userRepo, tokenRepo, jwtManager, cfg.RefreshTokenTTL)
	computeUsecase := usecase.NewComputeUsecase(computeRepo, computeProvider)

	authHandler := handler.NewAuthHandler(authUsecase)
	testHandler := handler.NewTestHandler()
	computeHandler := handler.NewComputeHandler(computeUsecase)
	router := deliveryhttp.NewRouter(authHandler, testHandler, computeHandler, jwtManager)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("shutting down")
	return server.Shutdown(shutdownCtx)
}
