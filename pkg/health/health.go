package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Health manages health checks for infrastructure components.
// It coordinates multiple checker registrations and executes them with caching and timeout support.
type Health struct {
	mu       sync.RWMutex
	checkers map[string]Checker

	// Result caching to prevent stampede
	cacheMu      sync.RWMutex
	cachedResult *HealthResult
	cacheExpiry  time.Time
	cacheTTL     time.Duration

	// Default timeout for health checks
	checkTimeout time.Duration
}

// HealthResult represents the aggregated health check result.
type HealthResult struct {
	Status string                 `json:"status"` // "healthy" or "unhealthy"
	Checks map[string]CheckResult `json:"checks"`
}

// CheckResult represents the result of a single component health check.
type CheckResult struct {
	Status  string `json:"status"`            // "ok" or "error"
	Message string `json:"message,omitempty"` // error message if status is "error"
}

// New creates a new Health instance with default configuration.
// Default check timeout is 5 seconds and cache TTL is 1 second.
func New() *Health {
	return &Health{
		checkers:     make(map[string]Checker),
		checkTimeout: 5 * time.Second,
		cacheTTL:     1 * time.Second,
	}
}

// NewWithConfig creates a new Health instance with custom configuration.
func NewWithConfig(checkTimeout, cacheTTL time.Duration) *Health {
	return &Health{
		checkers:     make(map[string]Checker),
		checkTimeout: checkTimeout,
		cacheTTL:     cacheTTL,
	}
}

// RegisterChecker registers a health checker for a named component.
// If a checker with the same name is already registered, it will be replaced.
func (h *Health) RegisterChecker(name string, checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.checkers[name] = checker
}

// UnregisterChecker removes a health checker by name.
// Returns true if a checker was removed, false if no checker with that name existed.
func (h *Health) UnregisterChecker(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.checkers[name]; exists {
		delete(h.checkers, name)
		return true
	}
	return false
}

// Check executes all registered health checkers and returns the aggregated result.
// Results are cached for cacheTTL duration to prevent stampede under load.
// Each checker is executed with checkTimeout unless the context has a shorter deadline.
func (h *Health) Check(ctx context.Context) *HealthResult {
	// Check cache first
	h.cacheMu.RLock()
	if h.cachedResult != nil && time.Now().Before(h.cacheExpiry) {
		result := h.cachedResult
		h.cacheMu.RUnlock()
		return result
	}
	h.cacheMu.RUnlock()

	// Execute health checks
	result := h.executeChecks(ctx)

	// Update cache
	h.cacheMu.Lock()
	h.cachedResult = result
	h.cacheExpiry = time.Now().Add(h.cacheTTL)
	h.cacheMu.Unlock()

	return result
}

// executeChecks runs all registered checkers concurrently and aggregates results.
func (h *Health) executeChecks(ctx context.Context) *HealthResult {
	h.mu.RLock()
	checkers := make(map[string]Checker, len(h.checkers))
	for name, checker := range h.checkers {
		checkers[name] = checker
	}
	h.mu.RUnlock()

	// If no checkers registered, return healthy
	if len(checkers) == 0 {
		return &HealthResult{
			Status: "healthy",
			Checks: make(map[string]CheckResult),
		}
	}

	// Execute all checks concurrently with timeout
	type checkResponse struct {
		name   string
		result CheckResult
	}

	resultChan := make(chan checkResponse, len(checkers))
	var wg sync.WaitGroup

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker Checker) {
			defer wg.Done()

			// Create context with timeout if not already set
			checkCtx := ctx
			if _, hasDeadline := ctx.Deadline(); !hasDeadline {
				var cancel context.CancelFunc
				checkCtx, cancel = context.WithTimeout(ctx, h.checkTimeout)
				defer cancel()
			}

			// Execute check
			err := checker.Check(checkCtx)

			// Build result
			var result CheckResult
			if err != nil {
				result = CheckResult{
					Status:  "error",
					Message: err.Error(),
				}
			} else {
				result = CheckResult{
					Status: "ok",
				}
			}

			resultChan <- checkResponse{name: name, result: result}
		}(name, checker)
	}

	// Wait for all checks to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	checks := make(map[string]CheckResult, len(checkers))
	allHealthy := true

	for response := range resultChan {
		checks[response.name] = response.result
		if response.result.Status != "ok" {
			allHealthy = false
		}
	}

	// Build final result
	status := "healthy"
	if !allHealthy {
		status = "unhealthy"
	}

	return &HealthResult{
		Status: status,
		Checks: checks,
	}
}

// CheckComponent executes a single component's health check by name.
// Returns an error if the component is not registered or if the check fails.
func (h *Health) CheckComponent(ctx context.Context, name string) error {
	h.mu.RLock()
	checker, exists := h.checkers[name]
	h.mu.RUnlock()

	if !exists {
		return fmt.Errorf("health checker %q not registered", name)
	}

	// Create context with timeout if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.checkTimeout)
		defer cancel()
	}

	return checker.Check(ctx)
}

// IsHealthy returns true if all registered checkers are currently healthy.
// This is a convenience method that executes Check and inspects the result.
func (h *Health) IsHealthy(ctx context.Context) bool {
	result := h.Check(ctx)
	return result.Status == "healthy"
}

// ClearCache clears the cached health check result, forcing the next Check call to re-execute.
func (h *Health) ClearCache() {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()

	h.cachedResult = nil
	h.cacheExpiry = time.Time{}
}
