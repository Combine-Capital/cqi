// Package service provides abstractions for microservice lifecycle management.
// It supports unified lifecycle for HTTP and gRPC servers with graceful shutdown,
// signal handling, and cleanup hooks.
//
// Example usage:
//
//	// Create HTTP service
//	httpSvc := service.NewHTTPService("my-service", ":8080", handler,
//	    service.WithReadTimeout(10*time.Second),
//	    service.WithShutdownTimeout(30*time.Second),
//	)
//
//	// Start service
//	if err := httpSvc.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Wait for shutdown signal
//	service.WaitForShutdown(ctx, httpSvc)
package service

import "context"

// Service represents a service that can be started, stopped, and health-checked.
// It provides a unified interface for HTTP and gRPC servers.
type Service interface {
	// Start starts the service and blocks until the service is ready.
	// Returns an error if the service fails to start.
	// The context can be used to cancel startup if needed.
	Start(ctx context.Context) error

	// Stop gracefully stops the service, waiting for in-flight requests to complete.
	// Returns an error if the service fails to stop within the timeout.
	// The context deadline determines how long to wait for graceful shutdown.
	Stop(ctx context.Context) error

	// Name returns the name of the service for logging and identification.
	Name() string

	// Health performs a health check on the service.
	// Returns nil if the service is healthy, or an error describing the problem.
	Health() error
}
