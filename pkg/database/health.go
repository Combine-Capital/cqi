package database

import (
	"context"
	"fmt"
	"time"
)

// CheckHealth performs a health check on the database by executing a simple query.
// It returns nil if the database is healthy, or an error if the database is unreachable
// or the query times out.
//
// The default timeout is 5 seconds unless the context has a shorter timeout.
func CheckHealth(ctx context.Context, db *Pool) error {
	// Create a context with timeout if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	// Execute a simple query to verify database connectivity
	var result int
	err := db.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("health check returned unexpected result: %d", result)
	}

	return nil
}

// Ping verifies a connection to the database is still alive.
// This is a convenience wrapper around Pool.Ping that implements HealthChecker interface.
func (p *Pool) PingWithTimeout(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return p.Ping(ctx)
}

// HealthCheck performs a comprehensive health check including connection pool stats.
// It returns an error if the database is unhealthy, along with diagnostic information.
func (p *Pool) HealthCheck(ctx context.Context) error {
	// First, check basic connectivity
	if err := CheckHealth(ctx, p); err != nil {
		return err
	}

	// Check pool statistics (if available)
	stats := p.Stats()
	if stats != nil {
		if stats.AcquireCount() > 0 && stats.IdleConns() == 0 && stats.TotalConns() == stats.MaxConns() {
			// Pool is exhausted - all connections in use
			return fmt.Errorf("connection pool exhausted: %d/%d connections in use",
				stats.TotalConns(), stats.MaxConns())
		}
	}

	return nil
}

// Check implements the health.Checker interface for the database pool.
// This allows the pool to be used directly with the health check framework.
//
// Example usage:
//
//	import "github.com/Combine-Capital/cqi/pkg/health"
//
//	h := health.New()
//	h.RegisterChecker("database", dbPool)
func (p *Pool) Check(ctx context.Context) error {
	return p.HealthCheck(ctx)
}
