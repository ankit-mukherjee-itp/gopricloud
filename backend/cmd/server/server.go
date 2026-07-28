// Package server owns the HTTP server bootstrap: it listens, waits for the
// caller's context to be cancelled, and shuts down gracefully. It is
// deliberately separate from the composition root in cmd/main.go, which only
// builds and injects dependencies.
package server

import (
	"context"
	"log"
	"net/http"
	"time"
)

// Serve runs handler on the given port until ctx is cancelled, then drains
// in-flight requests with a 10s deadline.
func Serve(ctx context.Context, handler http.Handler, port string) error {
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on :%s", port)
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
