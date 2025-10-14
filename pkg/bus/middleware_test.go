package bus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/Combine-Capital/cqi/pkg/logging"
	"google.golang.org/protobuf/proto"
)

func TestMiddleware_WithRetry(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("retry_test")

	// Track attempt count
	var attempts atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		count := attempts.Add(1)
		if count < 3 {
			// Fail first 2 attempts with temporary error
			return errors.NewTemporary("simulated failure", nil)
		}
		// Succeed on 3rd attempt
		wg.Done()
		return nil
	})

	// Subscribe with retry middleware
	err := bus.Subscribe(ctx, topic, handler, WithRetry(5, 10*time.Millisecond))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish
	msg := createTestMessage("retry-1", "Retry", 1)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for success
	wg.Wait()

	if attempts.Load() != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts.Load())
	}
}

func TestMiddleware_WithLogging(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("logging_test")

	// Create logger
	logger := logging.New(config.LogConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})

	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		wg.Done()
		return nil
	})

	// Subscribe with logging middleware
	err := bus.Subscribe(ctx, topic, handler, WithLogging(logger))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish
	msg := createTestMessage("log-1", "Logging", 1)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	wg.Wait()
}

func TestMiddleware_WithLogging_Error(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("logging_error_test")

	// Create logger
	logger := logging.New(config.LogConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})

	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		defer wg.Done()
		return errors.NewTemporary("test error", nil)
	})

	// Subscribe with logging middleware
	err := bus.Subscribe(ctx, topic, handler, WithLogging(logger))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish
	msg := createTestMessage("log-error-1", "LoggingError", 1)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	wg.Wait()
}

func TestMiddleware_WithRecovery(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("recovery_test")

	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		defer wg.Done()
		panic("simulated panic")
	})

	// Subscribe with recovery middleware
	err := bus.Subscribe(ctx, topic, handler, WithRecovery())
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish - should not crash despite panic
	msg := createTestMessage("panic-1", "Panic", 1)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	wg.Wait()
	// If we get here, recovery worked
}

func TestMiddleware_WithTimeout(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("timeout_test")

	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		defer wg.Done()
		// Sleep longer than timeout
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	// Subscribe with 50ms timeout
	err := bus.Subscribe(ctx, topic, handler, WithTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish
	msg := createTestMessage("timeout-1", "Timeout", 1)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	wg.Wait()
	// Handler should timeout - this is expected behavior
}

func TestMiddleware_Chaining(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("chain_test")

	logger := logging.New(config.LogConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})

	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		wg.Done()
		return nil
	})

	// Subscribe with multiple middleware
	err := bus.Subscribe(ctx, topic, handler,
		WithRecovery(),
		WithLogging(logger),
		WithRetry(3, 10*time.Millisecond),
		WithTimeout(1*time.Second),
	)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish
	msg := createTestMessage("chain-1", "Chain", 1)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	wg.Wait()
}

func TestMiddleware_WithErrorHandler(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("error_handler_test")

	var handledErrors atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		defer wg.Done()
		return errors.NewNotFound("test", "resource not found")
	})

	// Subscribe with error handler that ignores NotFound errors
	errorHandler := func(ctx context.Context, msg proto.Message, err error) error {
		if errors.IsNotFound(err) {
			handledErrors.Add(1)
			return nil // Ignore NotFound errors
		}
		return err
	}

	err := bus.Subscribe(ctx, topic, handler, WithErrorHandler(errorHandler))
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish
	msg := createTestMessage("error-handler-1", "ErrorHandler", 1)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	wg.Wait()

	if handledErrors.Load() != 1 {
		t.Errorf("Expected 1 handled error, got %d", handledErrors.Load())
	}
}

func TestMiddleware_WithMetrics(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("metrics_test")

	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		wg.Done()
		return nil
	})

	// Subscribe with metrics middleware
	err := bus.Subscribe(ctx, topic, handler, WithMetrics())
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish
	msg := createTestMessage("metrics-1", "Metrics", 1)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	wg.Wait()
}
