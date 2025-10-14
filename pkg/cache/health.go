package cache

import (
	"context"
	"time"

	"github.com/Combine-Capital/cqi/pkg/errors"
)

// HealthChecker defines the interface for cache health checking.
// This can be used with health check frameworks.
type HealthChecker interface {
	CheckHealth(ctx context.Context) error
}

// CheckHealthWithTimeout performs a health check with the specified timeout.
// This is a convenience wrapper that creates a context with timeout.
func CheckHealthWithTimeout(cache Cache, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := cache.CheckHealth(ctx); err != nil {
		return errors.NewTemporary("cache health check failed", err)
	}

	return nil
}
