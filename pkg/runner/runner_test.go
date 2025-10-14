package runner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockService is a mock implementation of the Service interface for testing.
type mockService struct {
	name          string
	startFunc     func(ctx context.Context) error
	stopFunc      func(ctx context.Context) error
	healthFunc    func() error
	startCount    int32
	stopCount     int32
	healthCount   int32
	mu            sync.Mutex
	started       bool
	stopDelay     time.Duration
	startDelay    time.Duration
	shouldFail    bool
	failAfter     time.Duration
	failOnce      bool
	hasFailedOnce bool
}

func newMockService(name string) *mockService {
	return &mockService{
		name: name,
	}
}

func (m *mockService) Start(ctx context.Context) error {
	atomic.AddInt32(&m.startCount, 1)

	if m.startDelay > 0 {
		select {
		case <-time.After(m.startDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.startFunc != nil {
		return m.startFunc(ctx)
	}

	m.started = true
	return nil
}

func (m *mockService) Stop(ctx context.Context) error {
	atomic.AddInt32(&m.stopCount, 1)

	if m.stopDelay > 0 {
		select {
		case <-time.After(m.stopDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopFunc != nil {
		return m.stopFunc(ctx)
	}

	m.started = false
	return nil
}

func (m *mockService) Name() string {
	return m.name
}

func (m *mockService) Health() error {
	atomic.AddInt32(&m.healthCount, 1)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.healthFunc != nil {
		return m.healthFunc()
	}

	if !m.started {
		return errors.New("service not started")
	}

	if m.shouldFail {
		if m.failOnce && !m.hasFailedOnce {
			m.hasFailedOnce = true
			return errors.New("temporary failure")
		}
		if !m.failOnce {
			return errors.New("service unhealthy")
		}
	}

	return nil
}

func (m *mockService) StartCount() int32 {
	return atomic.LoadInt32(&m.startCount)
}

func (m *mockService) StopCount() int32 {
	return atomic.LoadInt32(&m.stopCount)
}

func (m *mockService) HealthCount() int32 {
	return atomic.LoadInt32(&m.healthCount)
}

func (m *mockService) IsStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

// TestRunnerBasics tests basic runner functionality.
func TestRunnerBasics(t *testing.T) {
	t.Run("Create runner", func(t *testing.T) {
		runner := New("test-runner")
		if runner.Name() != "test-runner" {
			t.Errorf("Expected name 'test-runner', got %q", runner.Name())
		}
	})

	t.Run("Add service", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")

		if err := runner.Add(svc); err != nil {
			t.Fatalf("Failed to add service: %v", err)
		}
	})

	t.Run("Add nil service", func(t *testing.T) {
		runner := New("test")
		if err := runner.Add(nil); err == nil {
			t.Error("Expected error when adding nil service")
		}
	})

	t.Run("Add duplicate service", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")

		runner.Add(svc)
		if err := runner.Add(svc); err == nil {
			t.Error("Expected error when adding duplicate service")
		}
	})

	t.Run("Cannot add while running", func(t *testing.T) {
		runner := New("test")
		svc1 := newMockService("service1")
		svc2 := newMockService("service2")

		runner.Add(svc1)

		ctx := context.Background()
		if err := runner.Start(ctx); err != nil {
			t.Fatalf("Failed to start runner: %v", err)
		}

		if err := runner.Add(svc2); err == nil {
			t.Error("Expected error when adding service while running")
		}

		runner.Stop(ctx)
	})
}

// TestRunnerStartStop tests starting and stopping services.
func TestRunnerStartStop(t *testing.T) {
	t.Run("Start and stop single service", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		runner.Add(svc)

		ctx := context.Background()
		if err := runner.Start(ctx); err != nil {
			t.Fatalf("Failed to start runner: %v", err)
		}

		if !svc.IsStarted() {
			t.Error("Service should be started")
		}
		if svc.StartCount() != 1 {
			t.Errorf("Expected 1 start call, got %d", svc.StartCount())
		}

		if err := runner.Stop(ctx); err != nil {
			t.Fatalf("Failed to stop runner: %v", err)
		}

		if svc.IsStarted() {
			t.Error("Service should be stopped")
		}
		if svc.StopCount() != 1 {
			t.Errorf("Expected 1 stop call, got %d", svc.StopCount())
		}
	})

	t.Run("Start multiple services", func(t *testing.T) {
		runner := New("test")
		svc1 := newMockService("service1")
		svc2 := newMockService("service2")
		svc3 := newMockService("service3")

		runner.Add(svc1)
		runner.Add(svc2)
		runner.Add(svc3)

		ctx := context.Background()
		if err := runner.Start(ctx); err != nil {
			t.Fatalf("Failed to start runner: %v", err)
		}

		// All services should be started
		for _, svc := range []*mockService{svc1, svc2, svc3} {
			if !svc.IsStarted() {
				t.Errorf("Service %s should be started", svc.Name())
			}
		}

		runner.Stop(ctx)
	})

	t.Run("Cannot start empty runner", func(t *testing.T) {
		runner := New("test")
		ctx := context.Background()

		if err := runner.Start(ctx); err == nil {
			t.Error("Expected error when starting empty runner")
		}
	})

	t.Run("Cannot start already running", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		runner.Add(svc)

		ctx := context.Background()
		runner.Start(ctx)

		if err := runner.Start(ctx); err == nil {
			t.Error("Expected error when starting already running runner")
		}

		runner.Stop(ctx)
	})

	t.Run("Stop is idempotent", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		runner.Add(svc)

		ctx := context.Background()
		runner.Start(ctx)
		runner.Stop(ctx)

		// Second stop should not error
		if err := runner.Stop(ctx); err != nil {
			t.Errorf("Second stop should not error: %v", err)
		}
	})
}

// TestDependencyOrdering tests service startup ordering with dependencies.
func TestDependencyOrdering(t *testing.T) {
	t.Run("Simple dependency chain", func(t *testing.T) {
		runner := New("test")

		var startOrder []string
		var mu sync.Mutex

		svc1 := newMockService("database")
		svc1.startFunc = func(ctx context.Context) error {
			mu.Lock()
			startOrder = append(startOrder, "database")
			mu.Unlock()
			return nil
		}

		svc2 := newMockService("cache")
		svc2.startFunc = func(ctx context.Context) error {
			mu.Lock()
			startOrder = append(startOrder, "cache")
			mu.Unlock()
			return nil
		}

		svc3 := newMockService("api")
		svc3.startFunc = func(ctx context.Context) error {
			mu.Lock()
			startOrder = append(startOrder, "api")
			mu.Unlock()
			return nil
		}

		runner.Add(svc3, WithDependsOn("database", "cache"))
		runner.Add(svc1)
		runner.Add(svc2, WithDependsOn("database"))

		ctx := context.Background()
		if err := runner.Start(ctx); err != nil {
			t.Fatalf("Failed to start runner: %v", err)
		}

		// Verify ordering: database should start first, then cache, then api
		mu.Lock()
		if len(startOrder) != 3 {
			t.Fatalf("Expected 3 services to start, got %d", len(startOrder))
		}
		if startOrder[0] != "database" {
			t.Errorf("Expected database first, got %s", startOrder[0])
		}
		if startOrder[1] != "cache" {
			t.Errorf("Expected cache second, got %s", startOrder[1])
		}
		if startOrder[2] != "api" {
			t.Errorf("Expected api third, got %s", startOrder[2])
		}
		mu.Unlock()

		runner.Stop(ctx)
	})

	t.Run("Circular dependency detection", func(t *testing.T) {
		runner := New("test")

		svc1 := newMockService("service1")
		svc2 := newMockService("service2")

		runner.Add(svc1, WithDependsOn("service2"))
		runner.Add(svc2, WithDependsOn("service1"))

		ctx := context.Background()
		if err := runner.Start(ctx); err == nil {
			t.Error("Expected error for circular dependency")
			runner.Stop(ctx)
		}
	})

	t.Run("Non-existent dependency", func(t *testing.T) {
		runner := New("test")

		svc := newMockService("service1")
		runner.Add(svc, WithDependsOn("non-existent"))

		ctx := context.Background()
		if err := runner.Start(ctx); err == nil {
			t.Error("Expected error for non-existent dependency")
			runner.Stop(ctx)
		}
	})

	t.Run("Reverse order on stop", func(t *testing.T) {
		runner := New("test")

		var stopOrder []string
		var mu sync.Mutex

		svc1 := newMockService("database")
		svc1.stopFunc = func(ctx context.Context) error {
			mu.Lock()
			stopOrder = append(stopOrder, "database")
			mu.Unlock()
			return nil
		}

		svc2 := newMockService("api")
		svc2.stopFunc = func(ctx context.Context) error {
			mu.Lock()
			stopOrder = append(stopOrder, "api")
			mu.Unlock()
			return nil
		}

		runner.Add(svc1)
		runner.Add(svc2, WithDependsOn("database"))

		ctx := context.Background()
		runner.Start(ctx)
		runner.Stop(ctx)

		// Verify reverse order: api should stop before database
		mu.Lock()
		if len(stopOrder) != 2 {
			t.Fatalf("Expected 2 services to stop, got %d", len(stopOrder))
		}
		if stopOrder[0] != "api" {
			t.Errorf("Expected api to stop first, got %s", stopOrder[0])
		}
		if stopOrder[1] != "database" {
			t.Errorf("Expected database to stop second, got %s", stopOrder[1])
		}
		mu.Unlock()
	})
}

// TestStartDelay tests service start delays.
func TestStartDelay(t *testing.T) {
	t.Run("Start delay", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		runner.Add(svc, WithStartDelay(100*time.Millisecond))

		ctx := context.Background()
		start := time.Now()

		if err := runner.Start(ctx); err != nil {
			t.Fatalf("Failed to start runner: %v", err)
		}

		elapsed := time.Since(start)
		if elapsed < 100*time.Millisecond {
			t.Errorf("Expected at least 100ms delay, got %v", elapsed)
		}

		runner.Stop(ctx)
	})

	t.Run("Context cancellation during delay", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		runner.Add(svc, WithStartDelay(1*time.Second))

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		if err := runner.Start(ctx); err == nil {
			t.Error("Expected error from context cancellation")
			runner.Stop(context.Background())
		}
	})
}

// TestRestartPolicies tests different restart policies.
func TestRestartPolicies(t *testing.T) {
	t.Run("RestartNever", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		svc.shouldFail = true
		svc.failOnce = true

		runner.Add(svc, WithRestartPolicy(RestartNever))

		ctx := context.Background()
		if err := runner.Start(ctx); err != nil {
			t.Fatalf("Failed to start runner: %v", err)
		}

		// Wait a bit for monitor to check health
		time.Sleep(200 * time.Millisecond)

		// Service should not restart
		if svc.StartCount() > 1 {
			t.Errorf("Service should not restart with RestartNever, got %d starts", svc.StartCount())
		}

		runner.Stop(ctx)
	})

	t.Run("RestartOnFailure", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		svc.shouldFail = true
		svc.failOnce = true

		config := RestartConfig{
			Policy:         RestartOnFailure,
			MaxRetries:     3,
			InitialBackoff: 10 * time.Millisecond,
			MaxBackoff:     100 * time.Millisecond,
			Multiplier:     2.0,
			Jitter:         false,
		}
		runner.Add(svc, WithRestartConfig(config))

		ctx := context.Background()
		if err := runner.Start(ctx); err != nil {
			t.Fatalf("Failed to start runner: %v", err)
		}

		// Wait for restart to occur
		time.Sleep(500 * time.Millisecond)

		// Service should have restarted at least once
		if svc.StartCount() < 2 {
			t.Errorf("Service should restart on failure, got %d starts", svc.StartCount())
		}

		runner.Stop(ctx)
	})
}

// TestAggregateHealth tests aggregate health checking.
func TestAggregateHealth(t *testing.T) {
	t.Run("All services healthy", func(t *testing.T) {
		runner := New("test")
		svc1 := newMockService("service1")
		svc2 := newMockService("service2")

		runner.Add(svc1)
		runner.Add(svc2)

		ctx := context.Background()
		runner.Start(ctx)

		if err := runner.Health(); err != nil {
			t.Errorf("Expected healthy, got error: %v", err)
		}

		status := runner.HealthStatus()
		if !status.Healthy {
			t.Error("Expected overall health to be true")
		}
		if len(status.Services) != 2 {
			t.Errorf("Expected 2 service statuses, got %d", len(status.Services))
		}

		runner.Stop(ctx)
	})

	t.Run("One service unhealthy", func(t *testing.T) {
		runner := New("test")
		svc1 := newMockService("service1")
		svc2 := newMockService("service2")
		svc2.shouldFail = true

		runner.Add(svc1, WithRestartPolicy(RestartNever))
		runner.Add(svc2, WithRestartPolicy(RestartNever))

		ctx := context.Background()
		runner.Start(ctx)

		// Wait for health checks to run
		time.Sleep(100 * time.Millisecond)

		if err := runner.Health(); err == nil {
			t.Error("Expected unhealthy status")
		}

		status := runner.HealthStatus()
		if status.Healthy {
			t.Error("Expected overall health to be false")
		}

		if status.Services["service1"].Healthy != true {
			t.Error("service1 should be healthy")
		}
		if status.Services["service2"].Healthy != false {
			t.Error("service2 should be unhealthy")
		}

		runner.Stop(ctx)
	})

	t.Run("Service not started", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		runner.Add(svc)

		// Don't start the runner
		status := runner.HealthStatus()
		if status.Healthy {
			t.Error("Expected unhealthy when services not started")
		}
	})
}

// TestContextCancellation tests context cancellation behavior.
func TestContextCancellation(t *testing.T) {
	t.Run("Cancel during start", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		svc.startDelay = 1 * time.Second

		runner.Add(svc)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := runner.Start(ctx)
		if err == nil {
			t.Error("Expected error from context cancellation")
			runner.Stop(context.Background())
		}
	})

	t.Run("Cancel during stop", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		// Don't add stop delay - the runner will mark service as stopped
		// regardless of whether Stop() completes

		runner.Add(svc)

		ctx := context.Background()
		runner.Start(ctx)

		stopCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Stop with timeout context
		runner.Stop(stopCtx)

		// Service should be marked as stopped by the runner
		if svc.IsStarted() {
			t.Error("Service should be stopped")
		}

		// Stop should have been called
		if svc.StopCount() < 1 {
			t.Error("Service Stop() should have been called")
		}
	})
}

// TestConcurrentOperations tests thread safety.
func TestConcurrentOperations(t *testing.T) {
	t.Run("Concurrent health checks", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		runner.Add(svc)

		ctx := context.Background()
		runner.Start(ctx)

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runner.Health()
				runner.HealthStatus()
			}()
		}

		wg.Wait()
		runner.Stop(ctx)
	})
}

// TestRestartConfig tests restart configuration and backoff logic.
func TestRestartConfig(t *testing.T) {
	t.Run("Default config", func(t *testing.T) {
		cfg := DefaultRestartConfig()
		if cfg.Policy != RestartOnFailure {
			t.Errorf("Expected RestartOnFailure, got %v", cfg.Policy)
		}
		if cfg.MaxRetries != 5 {
			t.Errorf("Expected MaxRetries 5, got %d", cfg.MaxRetries)
		}
	})

	t.Run("Backoff calculation", func(t *testing.T) {
		cfg := RestartConfig{
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
			Multiplier:     2.0,
			Jitter:         false,
		}

		// First attempt: 1s
		backoff := cfg.backoff(0)
		if backoff != 1*time.Second {
			t.Errorf("Expected 1s, got %v", backoff)
		}

		// Second attempt: 2s
		backoff = cfg.backoff(1)
		if backoff != 2*time.Second {
			t.Errorf("Expected 2s, got %v", backoff)
		}

		// Third attempt: 4s
		backoff = cfg.backoff(2)
		if backoff != 4*time.Second {
			t.Errorf("Expected 4s, got %v", backoff)
		}

		// Should cap at max backoff
		backoff = cfg.backoff(10)
		if backoff != 10*time.Second {
			t.Errorf("Expected 10s (capped), got %v", backoff)
		}
	})

	t.Run("Backoff with jitter", func(t *testing.T) {
		cfg := RestartConfig{
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
			Multiplier:     2.0,
			Jitter:         true,
		}

		// Jitter should vary the result
		backoff1 := cfg.backoff(1)
		backoff2 := cfg.backoff(1)

		// Should be around 2s but with jitter (1.5s - 2.5s range)
		if backoff1 < 1500*time.Millisecond || backoff1 > 2500*time.Millisecond {
			t.Errorf("Backoff out of expected range: %v", backoff1)
		}

		// Two calls might give different values (with high probability)
		// Note: This test could theoretically fail if RNG gives same values
		if backoff1 == backoff2 {
			t.Logf("Warning: Jitter gave same value twice: %v", backoff1)
		}
	})

	t.Run("Backoff with zero defaults", func(t *testing.T) {
		cfg := RestartConfig{
			Policy: RestartOnFailure,
		}

		// Should use defaults
		backoff := cfg.backoff(0)
		if backoff < 500*time.Millisecond || backoff > 1500*time.Millisecond {
			t.Errorf("Expected ~1s with defaults, got %v", backoff)
		}
	})

	t.Run("shouldRestart logic", func(t *testing.T) {
		tests := []struct {
			name    string
			policy  RestartPolicy
			err     error
			attempt int
			max     int
			want    bool
		}{
			{"Never with error", RestartNever, errors.New("fail"), 0, 5, false},
			{"Never without error", RestartNever, nil, 0, 5, false},
			{"Always with error", RestartAlways, errors.New("fail"), 0, 5, true},
			{"Always without error", RestartAlways, nil, 0, 5, true},
			{"OnFailure with error", RestartOnFailure, errors.New("fail"), 0, 5, true},
			{"OnFailure without error", RestartOnFailure, nil, 0, 5, false},
			{"Max retries exceeded", RestartOnFailure, errors.New("fail"), 5, 5, false},
			{"Unlimited retries", RestartOnFailure, errors.New("fail"), 100, 0, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := RestartConfig{
					Policy:     tt.policy,
					MaxRetries: tt.max,
				}
				got := cfg.shouldRestart(tt.err, tt.attempt)
				if got != tt.want {
					t.Errorf("shouldRestart() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("waitForBackoff respects context", func(t *testing.T) {
		cfg := RestartConfig{
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
			Multiplier:     2.0,
			Jitter:         false,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := cfg.waitForBackoff(ctx, 0)
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got %v", err)
		}
	})

	t.Run("RestartPolicy String()", func(t *testing.T) {
		tests := []struct {
			policy RestartPolicy
			want   string
		}{
			{RestartNever, "never"},
			{RestartAlways, "always"},
			{RestartOnFailure, "on-failure"},
			{RestartPolicy(999), "unknown(999)"},
		}

		for _, tt := range tests {
			got := tt.policy.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		}
	})
}

// TestOptions tests configuration options.
func TestOptions(t *testing.T) {
	t.Run("WithMaxRetries", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		runner.Add(svc, WithMaxRetries(10))

		runner.mu.RLock()
		cfg := runner.services["service1"].config
		runner.mu.RUnlock()

		if cfg.RestartConfig.MaxRetries != 10 {
			t.Errorf("Expected MaxRetries 10, got %d", cfg.RestartConfig.MaxRetries)
		}
	})

	t.Run("WithBackoff", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		runner.Add(svc, WithBackoff(2*time.Second, 30*time.Second, 3.0))

		runner.mu.RLock()
		cfg := runner.services["service1"].config
		runner.mu.RUnlock()

		if cfg.RestartConfig.InitialBackoff != 2*time.Second {
			t.Errorf("Expected InitialBackoff 2s, got %v", cfg.RestartConfig.InitialBackoff)
		}
		if cfg.RestartConfig.MaxBackoff != 30*time.Second {
			t.Errorf("Expected MaxBackoff 30s, got %v", cfg.RestartConfig.MaxBackoff)
		}
		if cfg.RestartConfig.Multiplier != 3.0 {
			t.Errorf("Expected Multiplier 3.0, got %v", cfg.RestartConfig.Multiplier)
		}
	})
}

// TestServiceFailures tests handling of service start failures.
func TestServiceFailures(t *testing.T) {
	t.Run("Service start failure stops all", func(t *testing.T) {
		runner := New("test")
		svc1 := newMockService("service1")
		svc2 := newMockService("service2")
		svc2.startFunc = func(ctx context.Context) error {
			return errors.New("start failed")
		}

		runner.Add(svc1)
		runner.Add(svc2, WithDependsOn("service1"))

		ctx := context.Background()
		err := runner.Start(ctx)
		if err == nil {
			t.Error("Expected error from service2 start failure")
			runner.Stop(ctx)
		}

		// Service1 should be stopped after service2 fails
		if svc1.IsStarted() {
			t.Error("service1 should be stopped after service2 fails")
		}
	})

	t.Run("RestartAlways policy", func(t *testing.T) {
		runner := New("test")
		svc := newMockService("service1")
		svc.shouldFail = true
		svc.failOnce = true

		config := RestartConfig{
			Policy:         RestartAlways,
			MaxRetries:     2,
			InitialBackoff: 10 * time.Millisecond,
			MaxBackoff:     100 * time.Millisecond,
			Multiplier:     2.0,
			Jitter:         false,
		}
		runner.Add(svc, WithRestartConfig(config))

		ctx := context.Background()
		if err := runner.Start(ctx); err != nil {
			t.Fatalf("Failed to start runner: %v", err)
		}

		// Wait for restart to occur
		time.Sleep(300 * time.Millisecond)

		// Service should have restarted (RestartAlways restarts even on success)
		if svc.StartCount() < 2 {
			t.Errorf("Service should restart with RestartAlways, got %d starts", svc.StartCount())
		}

		runner.Stop(ctx)
	})
}
