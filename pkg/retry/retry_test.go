package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	cqierrors "github.com/Combine-Capital/cqi/pkg/errors"
)

// TestDoSuccess verifies that Do executes successfully without retry when no error occurs.
func TestDoSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
	}

	attempts := 0
	err := Do(ctx, cfg, func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

// TestDoRetryTemporaryError verifies that Do retries on temporary errors.
func TestDoRetryTemporaryError(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		Policy:       PolicyTemporary,
	}

	attempts := 0
	err := Do(ctx, cfg, func() error {
		attempts++
		if attempts < 3 {
			return cqierrors.NewTemporary("temporary error", nil)
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error after retries, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestDoNoPermanentError verifies that Do does not retry on permanent errors.
func TestDoNoPermanentRetry(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Millisecond,
		Policy:       PolicyTemporary,
	}

	attempts := 0
	err := Do(ctx, cfg, func() error {
		attempts++
		return cqierrors.NewPermanent("permanent error", nil)
	})

	if err == nil {
		t.Error("expected error, got nil")
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry for permanent errors), got %d", attempts)
	}
}

// TestDoMaxAttempts verifies that Do respects max attempts configuration.
func TestDoMaxAttempts(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		Policy:       PolicyAll,
	}

	attempts := 0
	err := Do(ctx, cfg, func() error {
		attempts++
		return errors.New("always fails")
	})

	if err == nil {
		t.Error("expected error after max attempts, got nil")
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestDoContextCancellation verifies that Do respects context cancellation.
func TestDoContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := Config{
		MaxAttempts:  10,
		InitialDelay: 50 * time.Millisecond,
		Policy:       PolicyAll,
	}

	attempts := 0
	errChan := make(chan error, 1)

	go func() {
		errChan <- Do(ctx, cfg, func() error {
			attempts++
			if attempts == 2 {
				cancel()
			}
			return errors.New("always fails")
		})
	}()

	err := <-errChan
	if err == nil {
		t.Error("expected error after context cancellation, got nil")
	}

	// Should stop after context cancellation
	if attempts > 3 {
		t.Errorf("expected <=3 attempts due to context cancellation, got %d", attempts)
	}
}

// TestDoPolicyAll verifies PolicyAll retries all errors.
func TestDoPolicyAll(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		Policy:       PolicyAll,
	}

	attempts := 0
	err := Do(ctx, cfg, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("any error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error after retries, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestDoPolicyNone verifies PolicyNone never retries.
func TestDoPolicyNone(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Millisecond,
		Policy:       PolicyNone,
	}

	attempts := 0
	err := Do(ctx, cfg, func() error {
		attempts++
		return errors.New("some error")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry with PolicyNone), got %d", attempts)
	}
}

// TestDoCustomPolicyFunc verifies custom policy function works.
func TestDoCustomPolicyFunc(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Millisecond,
		PolicyFunc: func(err error) bool {
			// Only retry if error message contains "retry"
			return err.Error() == "retry me"
		},
	}

	// Test case 1: Error that should be retried
	attempts := 0
	err := Do(ctx, cfg, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("retry me")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error after retries, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	// Test case 2: Error that should not be retried
	attempts = 0
	err = Do(ctx, cfg, func() error {
		attempts++
		return errors.New("don't retry me")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt (custom policy rejects retry), got %d", attempts)
	}
}

// TestDoWithDataSuccess verifies DoWithData returns data successfully.
func TestDoWithDataSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
	}

	attempts := 0
	result, err := DoWithData(ctx, cfg, func() (string, error) {
		attempts++
		return "success", nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

// TestDoWithDataRetry verifies DoWithData retries and returns data.
func TestDoWithDataRetry(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		Policy:       PolicyAll,
	}

	attempts := 0
	result, err := DoWithData(ctx, cfg, func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errors.New("temporary failure")
		}
		return 42, nil
	})

	if err != nil {
		t.Errorf("expected no error after retries, got %v", err)
	}

	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestDoWithDataMaxAttempts verifies DoWithData respects max attempts.
func TestDoWithDataMaxAttempts(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		Policy:       PolicyAll,
	}

	attempts := 0
	result, err := DoWithData(ctx, cfg, func() (string, error) {
		attempts++
		return "partial", errors.New("always fails")
	})

	if err == nil {
		t.Error("expected error after max attempts, got nil")
	}

	// Returns the last attempted value on failure
	if result != "partial" {
		t.Errorf("expected 'partial' (last attempted value), got %q", result)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestConfigDefaults verifies default values are applied.
func TestConfigDefaults(t *testing.T) {
	cfg := Config{}
	cfg = cfg.withDefaults()

	if cfg.MaxAttempts != 10 {
		t.Errorf("expected MaxAttempts default 10, got %d", cfg.MaxAttempts)
	}

	if cfg.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay default 100ms, got %v", cfg.InitialDelay)
	}

	if cfg.MaxDelay != 5*time.Second {
		t.Errorf("expected MaxDelay default 5s, got %v", cfg.MaxDelay)
	}

	if cfg.Multiplier != 2.0 {
		t.Errorf("expected Multiplier default 2.0, got %f", cfg.Multiplier)
	}

	if cfg.Jitter != 0.25 {
		t.Errorf("expected Jitter default 0.25, got %f", cfg.Jitter)
	}
}

// TestExponentialBackoff verifies backoff delays increase exponentially.
func TestExponentialBackoff(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:  4,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0.0, // No jitter for predictable testing
		Policy:       PolicyAll,
	}

	attempts := 0
	startTimes := make([]time.Time, 0)

	Do(ctx, cfg, func() error {
		attempts++
		startTimes = append(startTimes, time.Now())
		return errors.New("keep failing")
	})

	// Verify we had expected number of attempts
	if attempts != 4 {
		t.Errorf("expected 4 attempts, got %d", attempts)
	}

	// Verify delays are increasing (with some tolerance for timing variance)
	if len(startTimes) >= 3 {
		delay1 := startTimes[1].Sub(startTimes[0])
		delay2 := startTimes[2].Sub(startTimes[1])

		// Second delay should be roughly 2x the first (allowing 50% variance due to scheduling)
		if delay2 < delay1 {
			t.Errorf("expected increasing delays, but delay2 (%v) < delay1 (%v)", delay2, delay1)
		}
	}
}

// TestMaxElapsedTime verifies retry stops when max elapsed time is reached.
func TestMaxElapsedTime(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts:    100, // High number to ensure time limit is hit first
		InitialDelay:   10 * time.Millisecond,
		MaxElapsedTime: 50 * time.Millisecond,
		Policy:         PolicyAll,
	}

	attempts := 0
	start := time.Now()

	Do(ctx, cfg, func() error {
		attempts++
		return errors.New("keep failing")
	})

	elapsed := time.Since(start)

	// Should stop around 50ms (allowing some variance)
	if elapsed > 150*time.Millisecond {
		t.Errorf("expected to stop around 50ms, but took %v", elapsed)
	}

	// Should have made multiple attempts but stopped due to time
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts before timeout, got %d", attempts)
	}
}
