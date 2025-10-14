// Package runner provides orchestration for managing multiple services
// with dependency handling, restart logic, and aggregate health checks.
//
// Example usage:
//
//	runner := runner.New("my-app")
//
//	// Add services with dependencies
//	runner.Add(dbService, runner.WithRestartPolicy(runner.RestartAlways))
//	runner.Add(apiService,
//	    runner.WithDependsOn("database"),
//	    runner.WithStartDelay(2*time.Second),
//	    runner.WithRestartPolicy(runner.RestartOnFailure),
//	)
//
//	// Start all services in dependency order
//	if err := runner.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Stop all services in reverse order
//	defer runner.Stop(context.Background())
package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Combine-Capital/cqi/pkg/service"
)

// Runner manages multiple services with dependency handling and restart logic.
type Runner struct {
	name string

	// services maps service names to managed service instances
	services map[string]*managedService

	// startOrder defines the order in which services should be started
	startOrder []string

	// mu protects services map and startOrder
	mu sync.RWMutex

	// running indicates if the runner is currently active
	running bool

	// ctx is the context for the runner's lifecycle
	ctx context.Context

	// cancel cancels the runner's context
	cancel context.CancelFunc

	// wg tracks running service goroutines
	wg sync.WaitGroup
}

// managedService wraps a service with its configuration and runtime state.
type managedService struct {
	service service.Service
	config  serviceConfig

	// running indicates if the service is currently running
	running bool

	// restartCount tracks the number of restart attempts
	restartCount int

	// mu protects running and restartCount
	mu sync.RWMutex

	// cancel cancels the service's context
	cancel context.CancelFunc
}

// New creates a new Runner with the given name.
func New(name string) *Runner {
	return &Runner{
		name:     name,
		services: make(map[string]*managedService),
	}
}

// Add adds a service to the runner with optional configuration.
// The service will be started when Runner.Start() is called.
// Services are started in dependency order based on WithDependsOn options.
func (r *Runner) Add(svc service.Service, opts ...Option) error {
	if svc == nil {
		return fmt.Errorf("service cannot be nil")
	}

	name := svc.Name()
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("cannot add service while runner is running")
	}

	if _, exists := r.services[name]; exists {
		return fmt.Errorf("service %q already exists", name)
	}

	// Build configuration
	cfg := defaultServiceConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	r.services[name] = &managedService{
		service: svc,
		config:  cfg,
	}

	return nil
}

// Start starts all services in dependency order.
// Services are started concurrently where dependencies allow.
// Returns an error if any service fails to start.
func (r *Runner) Start(ctx context.Context) error {
	r.mu.Lock()

	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("runner is already running")
	}

	if len(r.services) == 0 {
		r.mu.Unlock()
		return fmt.Errorf("no services to start")
	}

	// Calculate startup order
	order, err := topologicalSort(r.services)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to resolve service dependencies: %w", err)
	}

	r.startOrder = order
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.running = true

	r.mu.Unlock()

	// Start services in order
	for _, name := range order {
		r.mu.RLock()
		svc := r.services[name]
		r.mu.RUnlock()

		if err := r.startService(r.ctx, name, svc); err != nil {
			// Stop all started services on failure
			r.Stop(context.Background())
			return fmt.Errorf("failed to start service %q: %w", name, err)
		}
	}

	return nil
}

// startService starts a single service with its configuration.
func (r *Runner) startService(ctx context.Context, name string, svc *managedService) error {
	// Wait for start delay
	if svc.config.StartDelay > 0 {
		select {
		case <-time.After(svc.config.StartDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Start the service
	svcCtx, cancel := context.WithCancel(ctx)
	svc.cancel = cancel

	if err := svc.service.Start(svcCtx); err != nil {
		cancel()
		return err
	}

	svc.mu.Lock()
	svc.running = true
	svc.mu.Unlock()

	// Start restart monitor if policy is not Never
	if svc.config.RestartConfig.Policy != RestartNever {
		r.wg.Add(1)
		go r.monitorService(ctx, name, svc)
	}

	return nil
}

// monitorService monitors a service and restarts it according to policy.
func (r *Runner) monitorService(ctx context.Context, name string, svc *managedService) {
	defer r.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check service health
		err := svc.service.Health()

		// Determine if restart is needed
		shouldRestart := false
		if err != nil {
			svc.mu.Lock()
			shouldRestart = svc.config.RestartConfig.shouldRestart(err, svc.restartCount)
			svc.mu.Unlock()
		}

		if !shouldRestart {
			time.Sleep(1 * time.Second) // Check every second
			continue
		}

		// Restart the service
		svc.mu.Lock()
		attempt := svc.restartCount
		svc.restartCount++
		svc.mu.Unlock()

		// Wait for backoff
		if err := svc.config.RestartConfig.waitForBackoff(ctx, attempt); err != nil {
			return // Context cancelled
		}

		// Stop the service
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		svc.service.Stop(stopCtx)
		stopCancel()

		if svc.cancel != nil {
			svc.cancel()
		}

		svc.mu.Lock()
		svc.running = false
		svc.mu.Unlock()

		// Start the service again
		svcCtx, cancel := context.WithCancel(ctx)
		svc.cancel = cancel

		if err := svc.service.Start(svcCtx); err != nil {
			cancel()
			// Continue monitoring - will retry on next iteration
			continue
		}

		svc.mu.Lock()
		svc.running = true
		svc.mu.Unlock()
	}
}

// Stop stops all services in reverse startup order.
// Services are stopped gracefully with the given context timeout.
func (r *Runner) Stop(ctx context.Context) error {
	r.mu.Lock()

	if !r.running {
		r.mu.Unlock()
		return nil
	}

	r.running = false

	// Cancel runner context to stop monitors
	if r.cancel != nil {
		r.cancel()
	}

	// Get reverse order for shutdown
	stopOrder := reverseOrder(r.startOrder)

	r.mu.Unlock()

	// Stop services in reverse order
	var firstErr error
	for _, name := range stopOrder {
		r.mu.RLock()
		svc := r.services[name]
		r.mu.RUnlock()

		svc.mu.RLock()
		running := svc.running
		svc.mu.RUnlock()

		if !running {
			continue
		}

		// Stop the service
		if err := svc.service.Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}

		// Cancel service context
		if svc.cancel != nil {
			svc.cancel()
		}

		svc.mu.Lock()
		svc.running = false
		svc.mu.Unlock()
	}

	// Wait for all monitor goroutines to finish
	r.wg.Wait()

	return firstErr
}

// Name returns the name of the runner.
func (r *Runner) Name() string {
	return r.name
}
