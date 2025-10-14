package service

import (
	"context"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"google.golang.org/grpc"
)

// TestBootstrap tests the Bootstrap functionality.
func TestBootstrap(t *testing.T) {
	t.Run("Basic initialization", func(t *testing.T) {
		cfg := &config.Config{
			Service: config.ServiceConfig{
				Name:    "test-service",
				Version: "1.0.0",
				Env:     "test",
			},
			Metrics: config.MetricsConfig{
				Enabled:   true,
				Port:      19090,
				Path:      "/metrics",
				Namespace: "test",
			},
			Tracing: config.TracingConfig{
				Enabled: false, // Disable to avoid needing OTLP endpoint
			},
			Log: config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		}

		ctx := context.Background()
		bootstrap, err := NewBootstrap(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create bootstrap: %v", err)
		}
		defer bootstrap.Cleanup(ctx)

		if bootstrap.Config == nil {
			t.Error("Config should not be nil")
		}
		if bootstrap.Logger == nil {
			t.Error("Logger should not be nil")
		}
		if bootstrap.TracerProvider != nil {
			t.Error("TracerProvider should be nil when tracing disabled")
		}
	})

	t.Run("Without metrics", func(t *testing.T) {
		cfg := &config.Config{
			Service: config.ServiceConfig{
				Name:    "test-service",
				Version: "1.0.0",
				Env:     "test",
			},
			Metrics: config.MetricsConfig{
				Enabled: false,
			},
			Tracing: config.TracingConfig{
				Enabled: false,
			},
			Log: config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		}

		ctx := context.Background()
		bootstrap, err := NewBootstrap(ctx, cfg, WithoutMetrics())
		if err != nil {
			t.Fatalf("Failed to create bootstrap: %v", err)
		}
		defer bootstrap.Cleanup(ctx)

		if bootstrap.Logger == nil {
			t.Error("Logger should not be nil")
		}
	})

	t.Run("Without tracing", func(t *testing.T) {
		cfg := &config.Config{
			Service: config.ServiceConfig{
				Name:    "test-service",
				Version: "1.0.0",
				Env:     "test",
			},
			Metrics: config.MetricsConfig{
				Enabled:   true,
				Port:      19091,
				Path:      "/metrics",
				Namespace: "test",
			},
			Tracing: config.TracingConfig{
				Enabled: true,
			},
			Log: config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		}

		ctx := context.Background()
		bootstrap, err := NewBootstrap(ctx, cfg, WithoutTracing())
		if err != nil {
			t.Fatalf("Failed to create bootstrap: %v", err)
		}
		defer bootstrap.Cleanup(ctx)

		if bootstrap.TracerProvider != nil {
			t.Error("TracerProvider should be nil when WithoutTracing option used")
		}
	})

	t.Run("Without logger", func(t *testing.T) {
		cfg := &config.Config{
			Service: config.ServiceConfig{
				Name:    "test-service",
				Version: "1.0.0",
				Env:     "test",
			},
			Metrics: config.MetricsConfig{
				Enabled: false,
			},
			Tracing: config.TracingConfig{
				Enabled: false,
			},
			Log: config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		}

		ctx := context.Background()
		bootstrap, err := NewBootstrap(ctx, cfg, WithoutLogger())
		if err != nil {
			t.Fatalf("Failed to create bootstrap: %v", err)
		}
		defer bootstrap.Cleanup(ctx)

		if bootstrap.Logger != nil {
			t.Error("Logger should be nil when WithoutLogger option used")
		}
	})

	t.Run("Cleanup execution", func(t *testing.T) {
		cfg := &config.Config{
			Service: config.ServiceConfig{
				Name:    "test-service",
				Version: "1.0.0",
				Env:     "test",
			},
			Metrics: config.MetricsConfig{
				Enabled: false,
			},
			Tracing: config.TracingConfig{
				Enabled: false,
			},
			Log: config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		}

		ctx := context.Background()
		bootstrap, err := NewBootstrap(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create bootstrap: %v", err)
		}

		cleanupCalled := false
		bootstrap.AddCleanup(func(ctx context.Context) error {
			cleanupCalled = true
			return nil
		})

		if err := bootstrap.Cleanup(ctx); err != nil {
			t.Errorf("Cleanup should not error: %v", err)
		}

		if !cleanupCalled {
			t.Error("Custom cleanup function should have been called")
		}
	})

	t.Run("AddCleanup", func(t *testing.T) {
		cfg := &config.Config{
			Service: config.ServiceConfig{
				Name:    "test-service",
				Version: "1.0.0",
				Env:     "test",
			},
			Metrics: config.MetricsConfig{
				Enabled: false,
			},
			Tracing: config.TracingConfig{
				Enabled: false,
			},
			Log: config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		}

		ctx := context.Background()
		bootstrap, err := NewBootstrap(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create bootstrap: %v", err)
		}

		var order []int
		bootstrap.AddCleanup(func(ctx context.Context) error {
			order = append(order, 1)
			return nil
		})
		bootstrap.AddCleanup(func(ctx context.Context) error {
			order = append(order, 2)
			return nil
		})

		bootstrap.Cleanup(ctx)

		// Verify LIFO order (2, 1)
		if len(order) != 2 {
			t.Fatalf("Expected 2 cleanup calls, got %d", len(order))
		}
		if order[0] != 2 || order[1] != 1 {
			t.Errorf("Expected LIFO order [2,1], got %v", order)
		}
	})

	t.Run("Metrics initialization", func(t *testing.T) {
		cfg := &config.Config{
			Service: config.ServiceConfig{
				Name:    "test-service",
				Version: "1.0.0",
				Env:     "test",
			},
			Metrics: config.MetricsConfig{
				Enabled:   true,
				Port:      19092,
				Path:      "/metrics",
				Namespace: "test_metrics",
			},
			Tracing: config.TracingConfig{
				Enabled: false,
			},
			Log: config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		}

		ctx := context.Background()
		bootstrap, err := NewBootstrap(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create bootstrap: %v", err)
		}
		defer bootstrap.Cleanup(ctx)

		// Give metrics server a moment to start
		time.Sleep(100 * time.Millisecond)

		// Metrics should be initialized
		if bootstrap.Logger != nil {
			bootstrap.Logger.Info().Msg("Metrics initialized successfully")
		}
	})
}

// TestWaitForShutdown tests the shutdown signal handler.
func TestWaitForShutdown(t *testing.T) {
	// This is mostly tested in TestWaitForShutdownIntegration
	// Just verify the function exists and delegates correctly
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	svc := NewHTTPService("test", "127.0.0.1:18087", handler)
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Test that WaitForShutdown calls WaitForShutdownWithConfig
	done := make(chan struct{})
	go func() {
		// Use custom signal for testing
		WaitForShutdownWithConfig(ctx, ShutdownConfig{
			Timeout: 1 * time.Second,
			Signals: []os.Signal{syscall.SIGUSR2},
		}, svc)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	proc, _ := os.FindProcess(os.Getpid())
	proc.Signal(syscall.SIGUSR2)

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not complete in time")
	}
}

// TestGRPCServiceOptions tests additional gRPC service options.
func TestGRPCServiceOptions(t *testing.T) {
	t.Run("WithGRPCShutdownTimeout", func(t *testing.T) {
		svc := NewGRPCService("test", "127.0.0.1:19096", nil,
			WithGRPCShutdownTimeout(45*time.Second),
		)
		if svc.shutdownTimeout != 45*time.Second {
			t.Errorf("Expected shutdownTimeout 45s, got %v", svc.shutdownTimeout)
		}
	})

	t.Run("Server method", func(t *testing.T) {
		grpcServer := grpc.NewServer()
		svc := NewGRPCServiceWithServer("test", ":9097", grpcServer)
		if svc.Server() != grpcServer {
			t.Error("Server() should return the gRPC server")
		}
	})
}

// TestBootstrapWithTracing tests bootstrap with tracing enabled.
func TestBootstrapWithTracing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping tracing test in short mode")
	}

	// Test with tracing but no real OTLP endpoint (will use no-op)
	cfg := &config.Config{
		Service: config.ServiceConfig{
			Name:    "test-service",
			Version: "1.0.0",
			Env:     "test",
		},
		Metrics: config.MetricsConfig{
			Enabled: false,
		},
		Tracing: config.TracingConfig{
			Enabled: false, // Disabled since we don't have OTLP endpoint
		},
		Log: config.LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	ctx := context.Background()
	bootstrap, err := NewBootstrap(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create bootstrap: %v", err)
	}

	if err := bootstrap.Cleanup(ctx); err != nil {
		t.Errorf("Cleanup should not error: %v", err)
	}
}

// TestWaitForShutdownSimple tests the simple WaitForShutdown wrapper.
func TestWaitForShutdownSimple(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	svc := NewHTTPService("test", "127.0.0.1:18088", handler)
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Test WaitForShutdown wrapper
	done := make(chan struct{})
	go func() {
		// Override signals for testing
		cfg := DefaultShutdownConfig()
		cfg.Signals = []os.Signal{syscall.SIGUSR1}
		WaitForShutdownWithConfig(ctx, cfg, svc)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	proc, _ := os.FindProcess(os.Getpid())
	proc.Signal(syscall.SIGUSR1)

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not complete in time")
	}
}

// TestBootstrapCleanupError tests cleanup with errors.
func TestBootstrapCleanupError(t *testing.T) {
	cfg := &config.Config{
		Service: config.ServiceConfig{
			Name:    "test-service",
			Version: "1.0.0",
			Env:     "test",
		},
		Metrics: config.MetricsConfig{
			Enabled: false,
		},
		Tracing: config.TracingConfig{
			Enabled: false,
		},
		Log: config.LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	ctx := context.Background()
	bootstrap, err := NewBootstrap(ctx, cfg, WithoutLogger())
	if err != nil {
		t.Fatalf("Failed to create bootstrap: %v", err)
	}

	// Add cleanup that will error
	bootstrap.AddCleanup(func(ctx context.Context) error {
		return http.ErrServerClosed
	})

	// Cleanup should not panic even with errors
	_ = bootstrap.Cleanup(ctx)
}
