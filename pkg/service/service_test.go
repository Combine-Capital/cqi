package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// TestHTTPService tests the HTTP service lifecycle.
func TestHTTPService(t *testing.T) {
	t.Run("Start and Stop", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		svc := NewHTTPService("test-http", "127.0.0.1:18080", handler)
		ctx := context.Background()

		// Start service
		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}

		// Verify service is running
		if err := svc.Health(); err != nil {
			t.Errorf("Service should be healthy: %v", err)
		}

		// Make HTTP request
		resp, err := http.Get("http://127.0.0.1:18080/")
		if err != nil {
			t.Errorf("Failed to make request: %v", err)
		} else {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		}

		// Stop service
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := svc.Stop(stopCtx); err != nil {
			t.Errorf("Failed to stop service: %v", err)
		}

		// Verify service is stopped
		if err := svc.Health(); err == nil {
			t.Error("Service should not be healthy after stop")
		}
	})

	t.Run("Name", func(t *testing.T) {
		svc := NewHTTPService("test-service", ":8080", nil)
		if svc.Name() != "test-service" {
			t.Errorf("Expected name 'test-service', got '%s'", svc.Name())
		}
	})

	t.Run("Already Started", func(t *testing.T) {
		svc := NewHTTPService("test-http", "127.0.0.1:18081", http.NotFoundHandler())
		ctx := context.Background()

		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}
		defer svc.Stop(context.Background())

		// Try to start again
		if err := svc.Start(ctx); err == nil {
			t.Error("Expected error when starting already running service")
		}
	})

	t.Run("Stop Not Started", func(t *testing.T) {
		svc := NewHTTPService("test-http", ":8082", http.NotFoundHandler())
		ctx := context.Background()

		// Stop without starting should not error
		if err := svc.Stop(ctx); err != nil {
			t.Errorf("Stop on non-started service should not error: %v", err)
		}
	})

	t.Run("With Options", func(t *testing.T) {
		svc := NewHTTPService("test-http", "127.0.0.1:18083", http.NotFoundHandler(),
			WithReadTimeout(5*time.Second),
			WithWriteTimeout(10*time.Second),
			WithShutdownTimeout(15*time.Second),
			WithMaxHeaderBytes(2048),
		)

		if svc.readTimeout != 5*time.Second {
			t.Errorf("Expected readTimeout 5s, got %v", svc.readTimeout)
		}
		if svc.writeTimeout != 10*time.Second {
			t.Errorf("Expected writeTimeout 10s, got %v", svc.writeTimeout)
		}
		if svc.shutdownTimeout != 15*time.Second {
			t.Errorf("Expected shutdownTimeout 15s, got %v", svc.shutdownTimeout)
		}
		if svc.maxHeaderBytes != 2048 {
			t.Errorf("Expected maxHeaderBytes 2048, got %d", svc.maxHeaderBytes)
		}
	})

	t.Run("Graceful Shutdown Timeout", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow request
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		})

		svc := NewHTTPService("test-http", "127.0.0.1:18084", handler,
			WithShutdownTimeout(100*time.Millisecond),
		)
		ctx := context.Background()

		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}

		// Make request in background
		go func() {
			http.Get("http://127.0.0.1:18084/")
		}()

		time.Sleep(50 * time.Millisecond)

		// Stop with short timeout
		stopCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		// Shutdown should timeout but not fail
		_ = svc.Stop(stopCtx)
	})
}

// TestGRPCService tests the gRPC service lifecycle.
func TestGRPCService(t *testing.T) {
	t.Run("Start and Stop", func(t *testing.T) {
		svc := NewGRPCService("test-grpc", "127.0.0.1:19090",
			func(s *grpc.Server) {
				// Register health service
				grpc_health_v1.RegisterHealthServer(s, &healthServer{})
			},
		)
		ctx := context.Background()

		// Start service
		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}

		// Verify service is running
		if err := svc.Health(); err != nil {
			t.Errorf("Service should be healthy: %v", err)
		}

		// Stop service
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := svc.Stop(stopCtx); err != nil {
			t.Errorf("Failed to stop service: %v", err)
		}

		// Verify service is stopped
		if err := svc.Health(); err == nil {
			t.Error("Service should not be healthy after stop")
		}
	})

	t.Run("Name", func(t *testing.T) {
		svc := NewGRPCService("test-grpc-service", ":9090", nil)
		if svc.Name() != "test-grpc-service" {
			t.Errorf("Expected name 'test-grpc-service', got '%s'", svc.Name())
		}
	})

	t.Run("With Reflection", func(t *testing.T) {
		svc := NewGRPCService("test-grpc", "127.0.0.1:19091", nil,
			WithReflection(true),
		)
		ctx := context.Background()

		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}
		defer svc.Stop(context.Background())

		if !svc.enableReflection {
			t.Error("Reflection should be enabled")
		}
	})

	t.Run("With Custom Server", func(t *testing.T) {
		grpcServer := grpc.NewServer()
		svc := NewGRPCServiceWithServer("test-grpc", "127.0.0.1:19092", grpcServer)

		if svc.server != grpcServer {
			t.Error("Custom server should be used")
		}
	})

	t.Run("Already Started", func(t *testing.T) {
		svc := NewGRPCService("test-grpc", "127.0.0.1:19093", nil)
		ctx := context.Background()

		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}
		defer svc.Stop(context.Background())

		// Try to start again
		if err := svc.Start(ctx); err == nil {
			t.Error("Expected error when starting already running service")
		}
	})

	t.Run("Stop Not Started", func(t *testing.T) {
		svc := NewGRPCService("test-grpc", ":9094", nil)
		ctx := context.Background()

		// Stop without starting should not error
		if err := svc.Stop(ctx); err != nil {
			t.Errorf("Stop on non-started service should not error: %v", err)
		}
	})
}

// healthServer is a simple health check implementation for testing
type healthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (s *healthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// TestCleanupHandler tests the cleanup handler functionality.
func TestCleanupHandler(t *testing.T) {
	t.Run("Execute in LIFO order", func(t *testing.T) {
		cleanup := NewCleanupHandler()
		var order []int

		cleanup.Register(func(ctx context.Context) error {
			order = append(order, 1)
			return nil
		})
		cleanup.Register(func(ctx context.Context) error {
			order = append(order, 2)
			return nil
		})
		cleanup.Register(func(ctx context.Context) error {
			order = append(order, 3)
			return nil
		})

		ctx := context.Background()
		if err := cleanup.Execute(ctx); err != nil {
			t.Errorf("Execute should not error: %v", err)
		}

		// Verify LIFO order (3, 2, 1)
		if len(order) != 3 {
			t.Fatalf("Expected 3 cleanup calls, got %d", len(order))
		}
		if order[0] != 3 || order[1] != 2 || order[2] != 1 {
			t.Errorf("Expected LIFO order [3,2,1], got %v", order)
		}
	})

	t.Run("Collect errors but continue", func(t *testing.T) {
		cleanup := NewCleanupHandler()

		cleanup.Register(func(ctx context.Context) error {
			return nil // Success
		})
		cleanup.Register(func(ctx context.Context) error {
			return fmt.Errorf("error 2")
		})
		cleanup.Register(func(ctx context.Context) error {
			return fmt.Errorf("error 3")
		})

		ctx := context.Background()
		err := cleanup.Execute(ctx)

		// Should return first error but continue executing
		if err == nil {
			t.Error("Expected error from cleanup")
		}
		if err.Error() != "error 3" {
			t.Errorf("Expected 'error 3', got '%v'", err)
		}
	})
}

// TestShutdownConfig tests shutdown configuration.
func TestShutdownConfig(t *testing.T) {
	t.Run("Default config", func(t *testing.T) {
		cfg := DefaultShutdownConfig()

		if cfg.Timeout != 30*time.Second {
			t.Errorf("Expected timeout 30s, got %v", cfg.Timeout)
		}

		if len(cfg.Signals) != 2 {
			t.Errorf("Expected 2 signals, got %d", len(cfg.Signals))
		}

		hasInt := false
		hasTerm := false
		for _, sig := range cfg.Signals {
			if sig == syscall.SIGINT {
				hasInt = true
			}
			if sig == syscall.SIGTERM {
				hasTerm = true
			}
		}

		if !hasInt || !hasTerm {
			t.Error("Default signals should include SIGINT and SIGTERM")
		}
	})
}

// TestWithShutdownHandler tests the convenience wrapper.
func TestWithShutdownHandler(t *testing.T) {
	t.Run("Start failure", func(t *testing.T) {
		// Use invalid address to force start failure
		svc := NewHTTPService("test", "invalid:address", nil)
		ctx := context.Background()

		// This should return error from Start
		err := WithShutdownHandler(ctx, svc)
		if err == nil {
			t.Error("Expected error when starting with invalid address")
		}
	})
}

// TestServiceInterface verifies that our implementations satisfy the Service interface.
func TestServiceInterface(t *testing.T) {
	var _ Service = (*HTTPService)(nil)
	var _ Service = (*GRPCService)(nil)
}

// TestHTTPServiceContextCancellation tests that Start respects context cancellation.
func TestHTTPServiceContextCancellation(t *testing.T) {
	svc := NewHTTPService("test", "127.0.0.1:18085", http.NotFoundHandler())
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	err := svc.Start(ctx)
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestGRPCServiceContextCancellation tests that Start respects context cancellation.
func TestGRPCServiceContextCancellation(t *testing.T) {
	svc := NewGRPCService("test", "127.0.0.1:19095", nil)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	err := svc.Start(ctx)
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// BenchmarkHTTPServiceStartStop benchmarks the HTTP service lifecycle.
func BenchmarkHTTPServiceStartStop(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc := NewHTTPService("bench", fmt.Sprintf("127.0.0.1:%d", 20000+i), handler)
		ctx := context.Background()

		if err := svc.Start(ctx); err != nil {
			b.Fatalf("Failed to start: %v", err)
		}

		stopCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		if err := svc.Stop(stopCtx); err != nil {
			b.Fatalf("Failed to stop: %v", err)
		}
		cancel()
	}
}

// TestWaitForShutdownIntegration tests the shutdown signal handling.
// This is a basic test that verifies the mechanism works.
func TestWaitForShutdownIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	svc := NewHTTPService("test", "127.0.0.1:18086", handler)
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Start shutdown handler in background
	done := make(chan struct{})
	go func() {
		WaitForShutdownWithConfig(ctx, ShutdownConfig{
			Timeout: 1 * time.Second,
			Signals: []os.Signal{syscall.SIGUSR1}, // Use SIGUSR1 for testing
		}, svc)
		close(done)
	}()

	// Give it time to set up signal handler
	time.Sleep(100 * time.Millisecond)

	// Send signal
	proc, _ := os.FindProcess(os.Getpid())
	proc.Signal(syscall.SIGUSR1)

	// Wait for shutdown to complete
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not complete in time")
	}

	// Verify service stopped
	if err := svc.Health(); err == nil {
		t.Error("Service should not be healthy after shutdown")
	}
}
