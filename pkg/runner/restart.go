package runner

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RestartPolicy determines when and how a service should be restarted on failure.
type RestartPolicy int

const (
	// RestartNever means the service will never be automatically restarted.
	RestartNever RestartPolicy = iota

	// RestartAlways means the service will always be restarted regardless of exit status.
	RestartAlways

	// RestartOnFailure means the service will be restarted only if it exits with an error.
	RestartOnFailure
)

// String returns the string representation of the restart policy.
func (p RestartPolicy) String() string {
	switch p {
	case RestartNever:
		return "never"
	case RestartAlways:
		return "always"
	case RestartOnFailure:
		return "on-failure"
	default:
		return fmt.Sprintf("unknown(%d)", p)
	}
}

// RestartConfig configures restart behavior for a service.
type RestartConfig struct {
	// Policy determines when to restart the service.
	Policy RestartPolicy

	// MaxRetries is the maximum number of restart attempts (0 = unlimited).
	MaxRetries int

	// InitialBackoff is the initial delay before first restart.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum delay between restarts.
	MaxBackoff time.Duration

	// Multiplier is the factor by which the backoff increases on each retry.
	Multiplier float64

	// Jitter adds randomness to backoff (±25%) to prevent thundering herd.
	Jitter bool
}

// DefaultRestartConfig returns a restart config with sensible defaults.
func DefaultRestartConfig() RestartConfig {
	return RestartConfig{
		Policy:         RestartOnFailure,
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     60 * time.Second,
		Multiplier:     2.0,
		Jitter:         true,
	}
}

// backoff calculates the backoff duration for the given attempt number.
func (c RestartConfig) backoff(attempt int) time.Duration {
	if c.InitialBackoff == 0 {
		c.InitialBackoff = 1 * time.Second
	}
	if c.MaxBackoff == 0 {
		c.MaxBackoff = 60 * time.Second
	}
	if c.Multiplier == 0 {
		c.Multiplier = 2.0
	}

	// Calculate exponential backoff: initialBackoff * multiplier^attempt
	backoff := float64(c.InitialBackoff) * math.Pow(c.Multiplier, float64(attempt))

	// Cap at max backoff
	if backoff > float64(c.MaxBackoff) {
		backoff = float64(c.MaxBackoff)
	}

	duration := time.Duration(backoff)

	// Add jitter (±25%) to prevent thundering herd
	if c.Jitter {
		jitter := float64(duration) * 0.25
		jitterAmount := (rand.Float64()*2 - 1) * jitter // Random value between -jitter and +jitter
		duration = time.Duration(float64(duration) + jitterAmount)

		// Ensure we don't go below zero or above max
		if duration < 0 {
			duration = time.Duration(backoff * 0.75)
		}
		if duration > c.MaxBackoff {
			duration = c.MaxBackoff
		}
	}

	return duration
}

// shouldRestart determines if a service should be restarted based on policy and error.
func (c RestartConfig) shouldRestart(err error, attempt int) bool {
	// Check max retries (0 = unlimited)
	if c.MaxRetries > 0 && attempt >= c.MaxRetries {
		return false
	}

	switch c.Policy {
	case RestartNever:
		return false
	case RestartAlways:
		return true
	case RestartOnFailure:
		return err != nil
	default:
		return false
	}
}

// waitForBackoff waits for the appropriate backoff duration, respecting context cancellation.
// Returns nil if backoff completed, or context.Canceled/context.DeadlineExceeded if interrupted.
func (c RestartConfig) waitForBackoff(ctx context.Context, attempt int) error {
	backoff := c.backoff(attempt)

	select {
	case <-time.After(backoff):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
