package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server encapsulates an HTTP server with production timeouts and graceful shutdown.
type Server struct {
	httpServer *http.Server
	drainTime  time.Duration
}

// Config holds HTTP server parameters.
type Config struct {
	Addr              string
	Handler           http.Handler
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	DrainTimeout      time.Duration
}

// New creates a production-hardened Server.
func New(cfg Config) *Server {
	if cfg.ReadHeaderTimeout <= 0 {
		cfg.ReadHeaderTimeout = 5 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 60 * time.Second
	}
	if cfg.DrainTimeout <= 0 {
		cfg.DrainTimeout = 10 * time.Second
	}

	return &Server{
		drainTime: cfg.DrainTimeout,
		httpServer: &http.Server{
			Addr:              cfg.Addr,
			Handler:           cfg.Handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
	}
}

// ListenAndServeWithGracefulShutdown starts the HTTP server and blocks until
// an interrupt signal is received, then gracefully drains in-flight requests.
func (s *Server) ListenAndServeWithGracefulShutdown() error {
	shutdownErr := make(chan error, 1)

	// Background listener for OS termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("[FlowGate Server] Received signal %v. Initiating graceful shutdown (drain timeout: %v)...", sig, s.drainTime)

		ctx, cancel := context.WithTimeout(context.Background(), s.drainTime)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			shutdownErr <- fmt.Errorf("graceful shutdown failed: %w", err)
			return
		}

		log.Printf("[FlowGate Server] All active connections drained successfully. Goodbye.")
		shutdownErr <- nil
	}()

	log.Printf("[FlowGate Server] Listening on %s...", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return <-shutdownErr
}
