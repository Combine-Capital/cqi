// Package integration provides integration tests for event bus operations.
package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/bus"
	"github.com/Combine-Capital/cqi/pkg/cache/testproto"
	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/logging"
	"google.golang.org/protobuf/proto"
)

// TestMemoryBusPublishSubscribe tests in-memory event bus.
func TestMemoryBusPublishSubscribe(t *testing.T) {
	ctx := context.Background()

	eventBus := bus.NewMemory()
	defer eventBus.Close()

	// Channel to receive messages
	received := make(chan *testproto.TestMessage, 1)

	// Subscribe
	err := eventBus.Subscribe(ctx, "test.topic",
		bus.HandlerFunc(func(ctx context.Context, msg proto.Message) error {
			testMsg := msg.(*testproto.TestMessage)
			received <- testMsg
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish
	testMsg := &testproto.TestMessage{
		Id:    "test-1",
		Name:  "Test Event",
		Value: 42,
	}
	if err := eventBus.Publish(ctx, "test.topic", testMsg); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Wait for message
	select {
	case msg := <-received:
		if msg.Id != testMsg.Id {
			t.Errorf("Expected Id=%s, got Id=%s", testMsg.Id, msg.Id)
		}
		if msg.Name != testMsg.Name {
			t.Errorf("Expected Name=%s, got Name=%s", testMsg.Name, msg.Name)
		}
		if msg.Value != testMsg.Value {
			t.Errorf("Expected Value=%d, got Value=%d", testMsg.Value, msg.Value)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// TestMemoryBusMultipleSubscribers tests multiple subscribers.
func TestMemoryBusMultipleSubscribers(t *testing.T) {
	ctx := context.Background()

	eventBus := bus.NewMemory()
	defer eventBus.Close()

	const numSubscribers = 3
	var wg sync.WaitGroup
	wg.Add(numSubscribers)

	// Create multiple subscribers
	for i := 0; i < numSubscribers; i++ {
		err := eventBus.Subscribe(ctx, "test.multi",
			bus.HandlerFunc(func(ctx context.Context, msg proto.Message) error {
				defer wg.Done()
				return nil
			}),
		)
		if err != nil {
			t.Fatalf("Failed to subscribe %d: %v", i, err)
		}
	}

	// Publish one message
	testMsg := &testproto.TestMessage{
		Id:    "multi-test",
		Name:  "Multi Subscriber Test",
		Value: 123,
	}
	if err := eventBus.Publish(ctx, "test.multi", testMsg); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Wait for all subscribers to receive
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - all subscribers received
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for all subscribers")
	}
}

// TestBusWithLogging tests event bus with logging middleware.
func TestBusWithLogging(t *testing.T) {
	ctx := context.Background()

	eventBus := bus.NewMemory()
	defer eventBus.Close()

	logger := logging.New(config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	// Channel to receive messages
	received := make(chan bool, 1)

	// Subscribe with logging middleware
	err := eventBus.Subscribe(ctx, "test.logging",
		bus.HandlerFunc(func(ctx context.Context, msg proto.Message) error {
			received <- true
			return nil
		}),
		bus.WithLogging(logger),
	)
	if err != nil {
		t.Fatalf("Failed to subscribe with logging: %v", err)
	}

	// Publish
	testMsg := &testproto.TestMessage{
		Id:    "logging-test",
		Name:  "Logging Test",
		Value: 789,
	}
	if err := eventBus.Publish(ctx, "test.logging", testMsg); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Wait for message
	select {
	case <-received:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message with logging")
	}
}

// TestBusWithMetrics tests event bus with metrics middleware.
func TestBusWithMetrics(t *testing.T) {
	ctx := context.Background()

	eventBus := bus.NewMemory()
	defer eventBus.Close()

	// Channel to receive messages
	received := make(chan bool, 1)

	// Subscribe with metrics middleware
	err := eventBus.Subscribe(ctx, "test.metrics",
		bus.HandlerFunc(func(ctx context.Context, msg proto.Message) error {
			received <- true
			return nil
		}),
		bus.WithMetrics(),
	)
	if err != nil {
		t.Fatalf("Failed to subscribe with metrics: %v", err)
	}

	// Publish
	testMsg := &testproto.TestMessage{
		Id:    "metrics-test",
		Name:  "Metrics Test",
		Value: 456,
	}
	if err := eventBus.Publish(ctx, "test.metrics", testMsg); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Wait for message
	select {
	case <-received:
		// Success - metrics should have been recorded
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message with metrics")
	}
}

// TestJetStreamBusPublishSubscribe tests NATS JetStream event bus.
// This test requires NATS server to be running.
func TestJetStreamBusPublishSubscribe(t *testing.T) {
	ctx := context.Background()

	cfg := config.EventBusConfig{
		Backend:      "jetstream",
		Servers:      []string{"nats://localhost:4222"},
		StreamName:   "TEST_STREAM",
		ConsumerName: "test_consumer",
		MaxDeliver:   3,
		AckWait:      30 * time.Second,
	}

	eventBus, err := bus.NewJetStream(ctx, cfg)
	if err != nil {
		t.Skipf("Skipping JetStream test: %v", err)
	}
	defer eventBus.Close()

	// Channel to receive messages
	received := make(chan *testproto.TestMessage, 1)

	// Subscribe
	err = eventBus.Subscribe(ctx, "test.jetstream",
		bus.HandlerFunc(func(ctx context.Context, msg proto.Message) error {
			testMsg := msg.(*testproto.TestMessage)
			received <- testMsg
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("Failed to subscribe to JetStream: %v", err)
	}

	// Give subscriber time to start
	time.Sleep(100 * time.Millisecond)

	// Publish
	testMsg := &testproto.TestMessage{
		Id:    "jetstream-test",
		Name:  "JetStream Test",
		Value: 999,
	}
	if err := eventBus.Publish(ctx, "test.jetstream", testMsg); err != nil {
		t.Fatalf("Failed to publish to JetStream: %v", err)
	}

	// Wait for message
	select {
	case msg := <-received:
		if msg.Id != testMsg.Id {
			t.Errorf("Expected Id=%s, got Id=%s", testMsg.Id, msg.Id)
		}
		if msg.Name != testMsg.Name {
			t.Errorf("Expected Name=%s, got Name=%s", testMsg.Name, msg.Name)
		}
		if msg.Value != testMsg.Value {
			t.Errorf("Expected Value=%d, got Value=%d", testMsg.Value, msg.Value)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for JetStream message")
	}
}
