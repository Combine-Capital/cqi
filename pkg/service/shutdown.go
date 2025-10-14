package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ShutdownConfig configures graceful shutdown behavior.
type ShutdownConfig struct {
	// Timeout is the maximum time to wait for graceful shutdown.
	// After this timeout, services are forcefully stopped.
	Timeout time.Duration

	// Signals is the list of OS signals that trigger shutdown.
	// If empty, defaults to SIGINT and SIGTERM.
	Signals []os.Signal
}

// DefaultShutdownConfig returns sensible default shutdown configuration.
func DefaultShutdownConfig() ShutdownConfig {
	return ShutdownConfig{
		Timeout: 30 * time.Second,
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	}
}

// WaitForShutdown blocks until a shutdown signal is received, then gracefully
// stops the provided services. It handles SIGINT and SIGTERM by default.
//
// Services are stopped in the order provided. If a service fails to stop within
// the timeout, the error is logged but shutdown continues for remaining services.
//
// Example:
//
//	httpSvc := service.NewHTTPService("api", ":8080", handler)
//	if err := httpSvc.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Block until shutdown signal
//	service.WaitForShutdown(ctx, httpSvc)
func WaitForShutdown(ctx context.Context, services ...Service) {
	WaitForShutdownWithConfig(ctx, DefaultShutdownConfig(), services...)
}

// WaitForShutdownWithConfig is like WaitForShutdown but accepts custom shutdown configuration.
//
// Example:
//
//	cfg := service.ShutdownConfig{
//	    Timeout: 60 * time.Second,
//	    Signals: []os.Signal{syscall.SIGTERM},
//	}
//	service.WaitForShutdownWithConfig(ctx, cfg, httpSvc, grpcSvc)
func WaitForShutdownWithConfig(ctx context.Context, cfg ShutdownConfig, services ...Service) {
	// Set up signal handling
	signals := cfg.Signals
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, signals...)

	// Wait for signal
	sig := <-quit
	fmt.Printf("Received signal: %v, initiating graceful shutdown...\n", sig)

	// Stop cleanup signal notifications
	signal.Stop(quit)

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Stop all services
	for _, svc := range services {
		if err := svc.Stop(shutdownCtx); err != nil {
			fmt.Printf("Error stopping service %s: %v\n", svc.Name(), err)
		} else {
			fmt.Printf("Service %s stopped successfully\n", svc.Name())
		}
	}

	fmt.Println("Graceful shutdown completed")
}

// CleanupFunc represents a cleanup function to be executed during shutdown.
type CleanupFunc func(context.Context) error

// CleanupHandler manages cleanup functions that should be executed during shutdown.
// Cleanup functions are executed in LIFO order (last registered, first executed).
type CleanupHandler struct {
	cleanups []CleanupFunc
}

// NewCleanupHandler creates a new cleanup handler.
func NewCleanupHandler() *CleanupHandler {
	return &CleanupHandler{
		cleanups: make([]CleanupFunc, 0),
	}
}

// Register adds a cleanup function to be executed during shutdown.
// Cleanup functions are executed in reverse order (LIFO).
//
// Example:
//
//	cleanup := service.NewCleanupHandler()
//	cleanup.Register(func(ctx context.Context) error {
//	    return db.Close()
//	})
//	cleanup.Register(func(ctx context.Context) error {
//	    return cache.Close()
//	})
//	defer cleanup.Execute(ctx)
func (h *CleanupHandler) Register(fn CleanupFunc) {
	h.cleanups = append(h.cleanups, fn)
}

// Execute runs all registered cleanup functions in reverse order (LIFO).
// It continues executing cleanup functions even if some fail, collecting all errors.
// Returns the first error encountered, but logs all errors.
func (h *CleanupHandler) Execute(ctx context.Context) error {
	var firstErr error

	// Execute in reverse order (LIFO)
	for i := len(h.cleanups) - 1; i >= 0; i-- {
		if err := h.cleanups[i](ctx); err != nil {
			fmt.Printf("Cleanup error: %v\n", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// WithShutdownHandler wraps a service with automatic signal-based shutdown.
// It starts the service and blocks until a shutdown signal is received.
//
// Example:
//
//	svc := service.NewHTTPService("api", ":8080", handler)
//	if err := service.WithShutdownHandler(ctx, svc); err != nil {
//	    log.Fatal(err)
//	}
func WithShutdownHandler(ctx context.Context, svc Service) error {
	// Start the service
	if err := svc.Start(ctx); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Wait for shutdown signal
	WaitForShutdown(ctx, svc)

	return nil
}
