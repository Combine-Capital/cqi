package websocket

import (
	"context"
	"testing"
)

func TestNewHandlerRegistry(t *testing.T) {
	registry := NewHandlerRegistry()

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.Count() != 0 {
		t.Errorf("Expected 0 handlers in new registry, got: %d", registry.Count())
	}
}

func TestHandlerRegistry_Register(t *testing.T) {
	registry := NewHandlerRegistry()

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	registry.Register("test", handler)

	if registry.Count() != 1 {
		t.Errorf("Expected 1 handler after registration, got: %d", registry.Count())
	}
}

func TestHandlerRegistry_Unregister(t *testing.T) {
	registry := NewHandlerRegistry()

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	registry.Register("test", handler)
	registry.Unregister("test")

	if registry.Count() != 0 {
		t.Errorf("Expected 0 handlers after unregister, got: %d", registry.Count())
	}
}

func TestHandlerRegistry_UnregisterNonExistent(t *testing.T) {
	registry := NewHandlerRegistry()

	// Should not panic
	registry.Unregister("nonexistent")

	if registry.Count() != 0 {
		t.Errorf("Expected 0 handlers, got: %d", registry.Count())
	}
}

func TestHandlerRegistry_Handle(t *testing.T) {
	registry := NewHandlerRegistry()

	called := false
	handler := func(ctx context.Context, msg *Message) error {
		called = true
		return nil
	}

	registry.Register("test", handler)

	msg := &Message{
		Type: "test",
		Raw:  []byte("test data"),
	}

	err := registry.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !called {
		t.Error("Expected handler to be called")
	}
}

func TestHandlerRegistry_HandleUnregisteredType(t *testing.T) {
	registry := NewHandlerRegistry()

	msg := &Message{
		Type: "unregistered",
		Raw:  []byte("test data"),
	}

	// Should not error for unregistered type
	err := registry.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("Handle returned error for unregistered type: %v", err)
	}
}

func TestHandlerRegistry_HandleError(t *testing.T) {
	registry := NewHandlerRegistry()

	handlerErr := &testError{msg: "handler error"}
	handler := func(ctx context.Context, msg *Message) error {
		return handlerErr
	}

	registry.Register("test", handler)

	msg := &Message{
		Type: "test",
		Raw:  []byte("test data"),
	}

	err := registry.Handle(context.Background(), msg)
	if err != handlerErr {
		t.Errorf("Expected handler error, got: %v", err)
	}
}

func TestHandlerRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewHandlerRegistry()

	done := make(chan bool)

	// Concurrent registration
	for i := 0; i < 10; i++ {
		go func(idx int) {
			handler := func(ctx context.Context, msg *Message) error {
				return nil
			}
			registry.Register(string(rune('a'+idx)), handler)
			done <- true
		}(i)
	}

	// Wait for all registrations
	for i := 0; i < 10; i++ {
		<-done
	}

	if registry.Count() != 10 {
		t.Errorf("Expected 10 handlers after concurrent registration, got: %d", registry.Count())
	}
}

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
