package retry

import (
	"time"

	"github.com/Combine-Capital/cqi/pkg/errors"
)

// Policy defines when a function should be retried.
type Policy int

const (
	// PolicyTemporary retries only errors.Temporary errors.
	PolicyTemporary Policy = iota
	// PolicyAll retries all errors.
	PolicyAll
	// PolicyNone never retries (executes once).
	PolicyNone
)

// PolicyFunc is a custom function that determines if an error should be retried.
type PolicyFunc func(error) bool

// Config holds the retry configuration.
type Config struct {
	// MaxAttempts is the maximum number of attempts (initial attempt + retries).
	// 0 means unlimited attempts. Default is 10.
	MaxAttempts uint

	// InitialDelay is the initial backoff delay. Default is 100ms.
	InitialDelay time.Duration

	// MaxDelay is the maximum backoff delay. Default is 5 seconds.
	MaxDelay time.Duration

	// Multiplier is the backoff multiplier. Default is 2.0.
	Multiplier float64

	// Jitter is the randomization factor (0.0 to 1.0). Default is 0.25 (±25%).
	Jitter float64

	// MaxElapsedTime is the maximum total time for all retry attempts.
	// 0 means no time limit. Default is 0.
	MaxElapsedTime time.Duration

	// Policy determines which errors should be retried.
	// Default is PolicyTemporary.
	Policy Policy

	// PolicyFunc is a custom policy function. If set, it takes precedence over Policy.
	PolicyFunc PolicyFunc
}

// withDefaults returns a config with default values applied.
func (c Config) withDefaults() Config {
	if c.MaxAttempts == 0 {
		c.MaxAttempts = 10
	}
	if c.InitialDelay == 0 {
		c.InitialDelay = 100 * time.Millisecond
	}
	if c.MaxDelay == 0 {
		c.MaxDelay = 5 * time.Second
	}
	if c.Multiplier == 0 {
		c.Multiplier = 2.0
	}
	if c.Jitter == 0 {
		c.Jitter = 0.25 // ±25%
	}
	return c
}

// shouldRetry determines if an error should be retried based on the configured policy.
func (c Config) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Custom policy function takes precedence
	if c.PolicyFunc != nil {
		return c.PolicyFunc(err)
	}

	// Apply predefined policy
	switch c.Policy {
	case PolicyAll:
		return true
	case PolicyNone:
		return false
	case PolicyTemporary:
		return errors.IsTemporary(err)
	default:
		return errors.IsTemporary(err)
	}
}
