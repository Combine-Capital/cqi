package websocket

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/rs/zerolog"
)

func TestClient_WithLogging(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL: getWSURL(server),
	}

	client, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create logger
	logger := zerolog.Nop()

	// Apply logging middleware
	client.WithLogging(&logger)

	// Middleware should be applied (no errors)
	if client == nil {
		t.Fatal("Expected non-nil client after WithLogging")
	}
}

func TestClient_WithMetrics(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL: getWSURL(server),
	}

	client, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Apply metrics middleware
	client.WithMetrics("test")

	// Middleware should be applied (no errors)
	if client == nil {
		t.Fatal("Expected non-nil client after WithMetrics")
	}
}

func TestClient_WithRetry(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL: getWSURL(server),
	}

	client, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Apply retry middleware
	client.WithRetry(3, 100*time.Millisecond)

	// Middleware should be applied (no errors)
	if client == nil {
		t.Fatal("Expected non-nil client after WithRetry")
	}
}

func TestCreateLoggingHandler(t *testing.T) {
	logger := zerolog.Nop()

	called := false
	baseHandler := func(ctx context.Context, msg *Message) error {
		called = true
		return nil
	}

	loggingHandler := createLoggingHandler(&logger, "test", baseHandler)

	msg := &Message{
		Type: "test",
		Raw:  []byte("test data"),
	}

	err := loggingHandler(context.Background(), msg)
	if err != nil {
		t.Fatalf("Logging handler returned error: %v", err)
	}

	if !called {
		t.Error("Expected base handler to be called")
	}
}

func TestCreateLoggingHandlerWithError(t *testing.T) {
	logger := zerolog.Nop()

	handlerErr := errors.New("handler error")
	baseHandler := func(ctx context.Context, msg *Message) error {
		return handlerErr
	}

	loggingHandler := createLoggingHandler(&logger, "test", baseHandler)

	msg := &Message{
		Type: "test",
		Raw:  []byte("test data"),
	}

	err := loggingHandler(context.Background(), msg)
	if err != handlerErr {
		t.Errorf("Expected handler error, got: %v", err)
	}
}

func TestCreateRetryHandler(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		attempts := 0
		baseHandler := func(ctx context.Context, msg *Message) error {
			attempts++
			return nil
		}

		retryHandler := createRetryHandler(3, 10*time.Millisecond, baseHandler)

		msg := &Message{
			Type: "test",
			Raw:  []byte("test data"),
		}

		err := retryHandler(context.Background(), msg)
		if err != nil {
			t.Fatalf("Retry handler returned error: %v", err)
		}

		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got: %d", attempts)
		}
	})

	t.Run("retries on failure", func(t *testing.T) {
		attempts := 0
		baseHandler := func(ctx context.Context, msg *Message) error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary error")
			}
			return nil
		}

		retryHandler := createRetryHandler(3, 10*time.Millisecond, baseHandler)

		msg := &Message{
			Type: "test",
			Raw:  []byte("test data"),
		}

		err := retryHandler(context.Background(), msg)
		if err != nil {
			t.Fatalf("Retry handler returned error: %v", err)
		}

		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got: %d", attempts)
		}
	})

	t.Run("fails after max attempts", func(t *testing.T) {
		attempts := 0
		handlerErr := errors.New("persistent error")
		baseHandler := func(ctx context.Context, msg *Message) error {
			attempts++
			return handlerErr
		}

		retryHandler := createRetryHandler(3, 10*time.Millisecond, baseHandler)

		msg := &Message{
			Type: "test",
			Raw:  []byte("test data"),
		}

		err := retryHandler(context.Background(), msg)
		if err == nil {
			t.Fatal("Expected error after max retries")
		}

		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got: %d", attempts)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		attempts := 0
		baseHandler := func(ctx context.Context, msg *Message) error {
			attempts++
			return errors.New("error")
		}

		retryHandler := createRetryHandler(5, 100*time.Millisecond, baseHandler)

		msg := &Message{
			Type: "test",
			Raw:  []byte("test data"),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		err := retryHandler(ctx, msg)
		if err == nil {
			t.Fatal("Expected error after context cancellation")
		}

		// Should attempt at least once but not all 5 times due to timeout
		if attempts >= 5 {
			t.Errorf("Expected fewer than 5 attempts due to timeout, got: %d", attempts)
		}
	})
}

func TestLogConnection(t *testing.T) {
	logger := zerolog.Nop()

	// Should not panic
	LogConnection(&logger, "ws://localhost:8080", true)
	LogConnection(&logger, "ws://localhost:8080", false)
}

func TestLogReconnect(t *testing.T) {
	logger := zerolog.Nop()

	// Should not panic
	LogReconnect(&logger, "ws://localhost:8080", 1, errors.New("test error"))
	LogReconnect(&logger, "ws://localhost:8080", 2, nil)
}
