// Package health provides a health check framework for monitoring service and infrastructure component health.
// It supports liveness and readiness probes for Kubernetes and load balancers.
//
// Example usage:
//
//	// Create health checker
//	h := health.New()
//
//	// Register infrastructure component checkers
//	h.RegisterChecker("database", dbPool)
//	h.RegisterChecker("cache", redisClient)
//	h.RegisterChecker("event_bus", eventBus)
//
//	// Set up HTTP endpoints
//	http.HandleFunc("/health/live", h.LivenessHandler())
//	http.HandleFunc("/health/ready", h.ReadinessHandler())
//
// Liveness checks verify the service is running (no dependency checks).
// Readiness checks verify all registered infrastructure components are healthy.
package health

import (
	"context"
)

// Checker defines the interface for health checking infrastructure components.
// All infrastructure components (database, cache, event bus) should implement this interface.
type Checker interface {
	// Check performs a health check on the component.
	// It should verify connectivity and basic functionality with a reasonable timeout.
	// Returns nil if the component is healthy, or an error describing the problem.
	// The context may include a timeout, which the implementation must respect.
	Check(ctx context.Context) error
}

// CheckerFunc is a function adapter that implements the Checker interface.
// This allows simple functions to be used as health checkers.
type CheckerFunc func(ctx context.Context) error

// Check implements the Checker interface by calling the function.
func (f CheckerFunc) Check(ctx context.Context) error {
	return f(ctx)
}
