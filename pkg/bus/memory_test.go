package bus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/cache/testproto"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// createTestMessage creates a test protobuf message for testing.
func createTestMessage(id, name string, value int64) *testproto.TestMessage {
	return &testproto.TestMessage{
		Id:    id,
		Name:  name,
		Value: value,
		Tags:  []string{"test", "event"},
	}
}

func TestMemoryEventBus_PublishSubscribe(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("test_event")

	// Track received messages
	var received atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		testMsg := msg.(*testproto.TestMessage)
		if testMsg.Id != "test-123" {
			t.Errorf("Expected Id=test-123, got %s", testMsg.Id)
		}
		received.Add(1)
		wg.Done()
		return nil
	})

	// Subscribe
	err := bus.Subscribe(ctx, topic, handler)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Give subscriber time to start
	time.Sleep(10 * time.Millisecond)

	// Publish a message
	msg := createTestMessage("test-123", "Test", 42)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for message to be received
	wg.Wait()

	if received.Load() != 1 {
		t.Errorf("Expected 1 message received, got %d", received.Load())
	}
}

func TestMemoryEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("test_event")

	// Track messages received by each subscriber
	var sub1Count, sub2Count atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2)

	handler1 := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		sub1Count.Add(1)
		wg.Done()
		return nil
	})

	handler2 := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		sub2Count.Add(1)
		wg.Done()
		return nil
	})

	// Subscribe both handlers
	err := bus.Subscribe(ctx, topic, handler1)
	if err != nil {
		t.Fatalf("Subscribe 1 failed: %v", err)
	}

	err = bus.Subscribe(ctx, topic, handler2)
	if err != nil {
		t.Fatalf("Subscribe 2 failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish a message
	msg := createTestMessage("test-456", "Multi", 99)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for both subscribers
	wg.Wait()

	if sub1Count.Load() != 1 {
		t.Errorf("Subscriber 1: expected 1 message, got %d", sub1Count.Load())
	}
	if sub2Count.Load() != 1 {
		t.Errorf("Subscriber 2: expected 1 message, got %d", sub2Count.Load())
	}
}

func TestMemoryEventBus_Close(t *testing.T) {
	bus := NewMemory()

	ctx := context.Background()
	topic := TopicName("test_event")

	// Close the bus
	err := bus.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Try to publish - should fail
	msg := createTestMessage("test-closed", "Closed", 0)
	err = bus.Publish(ctx, topic, msg)
	if err == nil {
		t.Error("Expected error when publishing to closed bus")
	}
	if !errors.IsPermanent(err) {
		t.Error("Expected permanent error for closed bus")
	}

	// Try to subscribe - should fail
	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		return nil
	})
	err = bus.Subscribe(ctx, topic, handler)
	if err == nil {
		t.Error("Expected error when subscribing to closed bus")
	}
}

func TestMemoryEventBus_NoSubscribers(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("nonexistent")

	// Publish should succeed even with no subscribers
	msg := createTestMessage("test-999", "NoSubs", 0)
	err := bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Errorf("Publish with no subscribers should succeed, got error: %v", err)
	}
}

func TestMemoryEventBus_HandlerError(t *testing.T) {
	bus := NewMemory()
	defer bus.Close()

	ctx := context.Background()
	topic := TopicName("error_test")

	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		defer wg.Done()
		return errors.NewTemporary("simulated error", nil)
	})

	err := bus.Subscribe(ctx, topic, handler)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Publish - handler will error but bus should continue
	msg := createTestMessage("error-1", "Error", 1)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	wg.Wait()
	// If we get here, error was handled gracefully
}

func TestMemoryEventBus_DoubleClose(t *testing.T) {
	bus := NewMemory()

	// Close once
	err := bus.Close()
	if err != nil {
		t.Fatalf("First close failed: %v", err)
	}

	// Close again - should be idempotent
	err = bus.Close()
	if err != nil {
		t.Errorf("Second close should not error, got: %v", err)
	}
}
