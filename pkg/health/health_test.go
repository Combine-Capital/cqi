package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockChecker is a test implementation of Checker
type mockChecker struct {
	checkFunc func(ctx context.Context) error
}

func (m *mockChecker) Check(ctx context.Context) error {
	if m.checkFunc != nil {
		return m.checkFunc(ctx)
	}
	return nil
}

// TestNew verifies Health instance creation with defaults
func TestNew(t *testing.T) {
	h := New()
	if h == nil {
		t.Fatal("New() returned nil")
	}

	if h.checkTimeout != 5*time.Second {
		t.Errorf("expected default timeout 5s, got %v", h.checkTimeout)
	}

	if h.cacheTTL != 1*time.Second {
		t.Errorf("expected default cache TTL 1s, got %v", h.cacheTTL)
	}

	if h.checkers == nil {
		t.Error("checkers map not initialized")
	}
}

// TestNewWithConfig verifies custom configuration
func TestNewWithConfig(t *testing.T) {
	h := NewWithConfig(10*time.Second, 5*time.Second)
	if h == nil {
		t.Fatal("NewWithConfig() returned nil")
	}

	if h.checkTimeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", h.checkTimeout)
	}

	if h.cacheTTL != 5*time.Second {
		t.Errorf("expected cache TTL 5s, got %v", h.cacheTTL)
	}
}

// TestRegisterChecker verifies checker registration
func TestRegisterChecker(t *testing.T) {
	h := New()

	checker := &mockChecker{}
	h.RegisterChecker("test", checker)

	h.mu.RLock()
	registered, exists := h.checkers["test"]
	h.mu.RUnlock()

	if !exists {
		t.Error("checker not registered")
	}

	if registered != checker {
		t.Error("registered checker does not match")
	}
}

// TestRegisterCheckerReplaces verifies replacing an existing checker
func TestRegisterCheckerReplaces(t *testing.T) {
	h := New()

	checker1 := &mockChecker{}
	checker2 := &mockChecker{}

	h.RegisterChecker("test", checker1)
	h.RegisterChecker("test", checker2)

	h.mu.RLock()
	registered := h.checkers["test"]
	h.mu.RUnlock()

	if registered != checker2 {
		t.Error("checker not replaced")
	}
}

// TestUnregisterChecker verifies checker removal
func TestUnregisterChecker(t *testing.T) {
	h := New()

	checker := &mockChecker{}
	h.RegisterChecker("test", checker)

	removed := h.UnregisterChecker("test")
	if !removed {
		t.Error("UnregisterChecker returned false for existing checker")
	}

	h.mu.RLock()
	_, exists := h.checkers["test"]
	h.mu.RUnlock()

	if exists {
		t.Error("checker still registered after unregister")
	}
}

// TestUnregisterCheckerNotFound verifies behavior when checker doesn't exist
func TestUnregisterCheckerNotFound(t *testing.T) {
	h := New()

	removed := h.UnregisterChecker("nonexistent")
	if removed {
		t.Error("UnregisterChecker returned true for nonexistent checker")
	}
}

// TestCheckNoCheckers verifies behavior with no registered checkers
func TestCheckNoCheckers(t *testing.T) {
	h := New()

	result := h.Check(context.Background())

	if result.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", result.Status)
	}

	if len(result.Checks) != 0 {
		t.Errorf("expected 0 checks, got %d", len(result.Checks))
	}
}

// TestCheckAllHealthy verifies all checkers passing
func TestCheckAllHealthy(t *testing.T) {
	h := New()

	h.RegisterChecker("check1", &mockChecker{
		checkFunc: func(ctx context.Context) error { return nil },
	})
	h.RegisterChecker("check2", &mockChecker{
		checkFunc: func(ctx context.Context) error { return nil },
	})

	result := h.Check(context.Background())

	if result.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", result.Status)
	}

	if len(result.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(result.Checks))
	}

	for name, check := range result.Checks {
		if check.Status != "ok" {
			t.Errorf("check %q: expected status 'ok', got %q", name, check.Status)
		}
		if check.Message != "" {
			t.Errorf("check %q: expected empty message, got %q", name, check.Message)
		}
	}
}

// TestCheckSomeUnhealthy verifies behavior when some checkers fail
func TestCheckSomeUnhealthy(t *testing.T) {
	h := New()

	h.RegisterChecker("healthy", &mockChecker{
		checkFunc: func(ctx context.Context) error { return nil },
	})
	h.RegisterChecker("unhealthy", &mockChecker{
		checkFunc: func(ctx context.Context) error {
			return fmt.Errorf("connection failed")
		},
	})

	result := h.Check(context.Background())

	if result.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %q", result.Status)
	}

	if len(result.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(result.Checks))
	}

	healthyCheck := result.Checks["healthy"]
	if healthyCheck.Status != "ok" {
		t.Errorf("healthy check: expected status 'ok', got %q", healthyCheck.Status)
	}

	unhealthyCheck := result.Checks["unhealthy"]
	if unhealthyCheck.Status != "error" {
		t.Errorf("unhealthy check: expected status 'error', got %q", unhealthyCheck.Status)
	}
	if unhealthyCheck.Message != "connection failed" {
		t.Errorf("unhealthy check: expected message 'connection failed', got %q", unhealthyCheck.Message)
	}
}

// TestCheckCaching verifies result caching
func TestCheckCaching(t *testing.T) {
	h := New()

	callCount := 0
	h.RegisterChecker("test", &mockChecker{
		checkFunc: func(ctx context.Context) error {
			callCount++
			return nil
		},
	})

	// First call
	result1 := h.Check(context.Background())
	if result1.Status != "healthy" {
		t.Errorf("first call: expected status 'healthy', got %q", result1.Status)
	}
	if callCount != 1 {
		t.Errorf("first call: expected 1 checker call, got %d", callCount)
	}

	// Second call immediately (should use cache)
	result2 := h.Check(context.Background())
	if result2.Status != "healthy" {
		t.Errorf("second call: expected status 'healthy', got %q", result2.Status)
	}
	if callCount != 1 {
		t.Errorf("second call: expected 1 checker call (cached), got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(h.cacheTTL + 100*time.Millisecond)

	// Third call (should call checker again)
	result3 := h.Check(context.Background())
	if result3.Status != "healthy" {
		t.Errorf("third call: expected status 'healthy', got %q", result3.Status)
	}
	if callCount != 2 {
		t.Errorf("third call: expected 2 checker calls, got %d", callCount)
	}
}

// TestClearCache verifies cache clearing
func TestClearCache(t *testing.T) {
	h := New()

	callCount := 0
	h.RegisterChecker("test", &mockChecker{
		checkFunc: func(ctx context.Context) error {
			callCount++
			return nil
		},
	})

	// First call
	h.Check(context.Background())
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}

	// Clear cache
	h.ClearCache()

	// Second call should re-execute
	h.Check(context.Background())
	if callCount != 2 {
		t.Errorf("expected 2 calls after cache clear, got %d", callCount)
	}
}

// TestCheckTimeout verifies timeout handling
func TestCheckTimeout(t *testing.T) {
	h := NewWithConfig(100*time.Millisecond, 1*time.Second)

	h.RegisterChecker("slow", &mockChecker{
		checkFunc: func(ctx context.Context) error {
			// Wait for context cancellation
			<-ctx.Done()
			return ctx.Err()
		},
	})

	start := time.Now()
	result := h.Check(context.Background())
	duration := time.Since(start)

	// Should complete near timeout
	if duration < 100*time.Millisecond || duration > 200*time.Millisecond {
		t.Errorf("expected duration ~100ms, got %v", duration)
	}

	if result.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %q", result.Status)
	}

	slowCheck := result.Checks["slow"]
	if slowCheck.Status != "error" {
		t.Errorf("slow check: expected status 'error', got %q", slowCheck.Status)
	}
}

// TestCheckContextCancellation verifies context cancellation is respected
func TestCheckContextCancellation(t *testing.T) {
	h := New()

	h.RegisterChecker("test", &mockChecker{
		checkFunc: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	result := h.Check(ctx)
	duration := time.Since(start)

	// Should complete near context timeout
	if duration > 100*time.Millisecond {
		t.Errorf("expected duration <100ms, got %v", duration)
	}

	if result.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %q", result.Status)
	}
}

// TestCheckComponent verifies checking a single component
func TestCheckComponent(t *testing.T) {
	h := New()

	h.RegisterChecker("test", &mockChecker{
		checkFunc: func(ctx context.Context) error { return nil },
	})

	err := h.CheckComponent(context.Background(), "test")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestCheckComponentNotFound verifies error when component doesn't exist
func TestCheckComponentNotFound(t *testing.T) {
	h := New()

	err := h.CheckComponent(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent component")
	}
}

// TestCheckComponentUnhealthy verifies error propagation
func TestCheckComponentUnhealthy(t *testing.T) {
	h := New()

	expectedErr := fmt.Errorf("check failed")
	h.RegisterChecker("test", &mockChecker{
		checkFunc: func(ctx context.Context) error { return expectedErr },
	})

	err := h.CheckComponent(context.Background(), "test")
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// TestIsHealthy verifies convenience method
func TestIsHealthy(t *testing.T) {
	h := New()

	h.RegisterChecker("healthy", &mockChecker{
		checkFunc: func(ctx context.Context) error { return nil },
	})

	if !h.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to return true")
	}

	// Clear cache before adding unhealthy checker
	h.ClearCache()

	h.RegisterChecker("unhealthy", &mockChecker{
		checkFunc: func(ctx context.Context) error {
			return fmt.Errorf("failed")
		},
	})

	if h.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to return false")
	}
}

// TestCheckerFunc verifies function adapter
func TestCheckerFunc(t *testing.T) {
	called := false
	checker := CheckerFunc(func(ctx context.Context) error {
		called = true
		return nil
	})

	err := checker.Check(context.Background())
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !called {
		t.Error("checker function not called")
	}
}

// TestLivenessHandler verifies liveness endpoint
func TestLivenessHandler(t *testing.T) {
	h := New()
	handler := h.LivenessHandler()

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "alive" {
		t.Errorf("expected status 'alive', got %q", response["status"])
	}
}

// TestReadinessHandlerHealthy verifies readiness endpoint with healthy checks
func TestReadinessHandlerHealthy(t *testing.T) {
	h := New()
	h.RegisterChecker("test", &mockChecker{
		checkFunc: func(ctx context.Context) error { return nil },
	})

	handler := h.ReadinessHandler()

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var result HealthResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", result.Status)
	}
}

// TestReadinessHandlerUnhealthy verifies readiness endpoint with failed checks
func TestReadinessHandlerUnhealthy(t *testing.T) {
	h := New()
	h.RegisterChecker("test", &mockChecker{
		checkFunc: func(ctx context.Context) error {
			return fmt.Errorf("check failed")
		},
	})

	handler := h.ReadinessHandler()

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}

	var result HealthResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %q", result.Status)
	}
}

// TestHealthHandler verifies combined health endpoint
func TestHealthHandler(t *testing.T) {
	h := New()
	h.RegisterChecker("test", &mockChecker{
		checkFunc: func(ctx context.Context) error { return nil },
	})

	handler := h.HealthHandler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["liveness"] != "alive" {
		t.Errorf("expected liveness 'alive', got %v", response["liveness"])
	}

	if response["readiness"] == nil {
		t.Error("expected readiness to be present")
	}
}

// TestConcurrentChecks verifies concurrent health check execution
func TestConcurrentChecks(t *testing.T) {
	h := New()

	// Register multiple checkers
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("check%d", i)
		h.RegisterChecker(name, &mockChecker{
			checkFunc: func(ctx context.Context) error {
				// Simulate some work
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		})
	}

	start := time.Now()
	result := h.Check(context.Background())
	duration := time.Since(start)

	// Should complete much faster than sequential execution (100ms)
	// Concurrent execution should be ~10ms + overhead
	if duration > 50*time.Millisecond {
		t.Errorf("expected duration <50ms with concurrent checks, got %v", duration)
	}

	if result.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", result.Status)
	}

	if len(result.Checks) != 10 {
		t.Errorf("expected 10 checks, got %d", len(result.Checks))
	}
}
