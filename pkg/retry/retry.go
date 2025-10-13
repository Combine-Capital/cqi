// Package retry provides retry logic with exponential backoff for transient failures.
//
// This package wraps github.com/cenkalti/backoff/v5 and integrates it with the CQI
// error package to provide intelligent retry policies based on error types. It provides
// exponential backoff with jitter to prevent thundering herd problems when multiple
// clients retry simultaneously.
//
// Example usage:
//
//	cfg := retry.Config{
//		MaxAttempts:    5,
//		InitialDelay:   100 * time.Millisecond,
//		MaxDelay:       5 * time.Second,
//		Multiplier:     2.0,
//		Policy:         retry.PolicyTemporary,
//	}
//
//	err := retry.Do(ctx, cfg, func() error {
//		return someOperation()
//	})
package retry

import (
	"context"

	"github.com/cenkalti/backoff/v5"
)

// Do executes the provided function with retry logic based on the configuration.
// It respects context cancellation and applies exponential backoff between retries.
//
// The function will retry on errors according to the configured policy:
//   - PolicyTemporary: Only retry errors.Temporary errors
//   - PolicyAll: Retry all errors
//   - PolicyNone: Never retry (execute once)
//   - Custom policy functions can be provided via Config.PolicyFunc
//
// Returns the error from the last attempt if all retries are exhausted.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	// Apply defaults
	cfg = cfg.withDefaults()

	// Create backoff strategy
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialDelay
	b.MaxInterval = cfg.MaxDelay
	b.Multiplier = cfg.Multiplier
	b.RandomizationFactor = cfg.Jitter

	// Build retry options
	opts := []backoff.RetryOption{
		backoff.WithBackOff(b),
	}

	if cfg.MaxAttempts > 0 {
		opts = append(opts, backoff.WithMaxTries(cfg.MaxAttempts))
	}

	if cfg.MaxElapsedTime > 0 {
		opts = append(opts, backoff.WithMaxElapsedTime(cfg.MaxElapsedTime))
	}

	// Execute with retry
	// backoff.Retry requires Operation[T] which returns (T, error)
	// For Do (no return value), we use a dummy struct{} as T
	operation := func() (struct{}, error) {
		err := fn()
		if err == nil {
			return struct{}{}, nil
		}

		// Check if we should retry based on policy
		if !cfg.shouldRetry(err) {
			// Mark as permanent error to stop retrying
			return struct{}{}, backoff.Permanent(err)
		}

		return struct{}{}, err
	}

	_, err := backoff.Retry(ctx, operation, opts...)
	return err
}

// DoWithData executes the provided function with retry logic and returns a value.
// It works the same as Do but supports functions that return both a value and an error.
//
// Example:
//
//	data, err := retry.DoWithData(ctx, cfg, func() (string, error) {
//		return fetchData()
//	})
func DoWithData[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	// Apply defaults
	cfg = cfg.withDefaults()

	// Create backoff strategy
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialDelay
	b.MaxInterval = cfg.MaxDelay
	b.Multiplier = cfg.Multiplier
	b.RandomizationFactor = cfg.Jitter

	// Build retry options
	opts := []backoff.RetryOption{
		backoff.WithBackOff(b),
	}

	if cfg.MaxAttempts > 0 {
		opts = append(opts, backoff.WithMaxTries(cfg.MaxAttempts))
	}

	if cfg.MaxElapsedTime > 0 {
		opts = append(opts, backoff.WithMaxElapsedTime(cfg.MaxElapsedTime))
	}

	// Execute with retry
	operation := func() (T, error) {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		// Check if we should retry based on policy
		if !cfg.shouldRetry(err) {
			// Mark as permanent error to stop retrying
			var zero T
			return zero, backoff.Permanent(err)
		}

		return result, err
	}

	return backoff.Retry(ctx, operation, opts...)
}
