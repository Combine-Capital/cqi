package bus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/cache/testproto"
	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/nats-io/nats-server/v2/server"
	"google.golang.org/protobuf/proto"
)

// startTestNATSServer starts an embedded NATS server for testing.
func startTestNATSServer(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1, // Random port
		JetStream: true,
		StoreDir:  t.TempDir(),
	}

	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("Failed to create NATS server: %v", err)
	}

	go s.Start()

	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	return s
}

func TestJetStreamEventBus_CreateStream(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	ctx := context.Background()
	cfg := config.EventBusConfig{
		Backend:    "jetstream",
		Servers:    []string{ns.ClientURL()},
		StreamName: "TEST_EVENTS",
	}

	bus, err := NewJetStream(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJetStream failed: %v", err)
	}
	defer bus.Close()

	// Verify stream was created
	stream, err := bus.js.Stream(ctx, "TEST_EVENTS")
	if err != nil {
		t.Fatalf("Stream not found: %v", err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatalf("Failed to get stream info: %v", err)
	}

	if info.Config.Name != "TEST_EVENTS" {
		t.Errorf("Expected stream name TEST_EVENTS, got %s", info.Config.Name)
	}
}

func TestJetStreamEventBus_PublishSubscribe(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	ctx := context.Background()
	cfg := config.EventBusConfig{
		Backend:      "jetstream",
		Servers:      []string{ns.ClientURL()},
		StreamName:   "TEST_EVENTS",
		ConsumerName: "test_consumer",
		MaxDeliver:   3,
		AckWait:      5 * time.Second,
	}

	bus, err := NewJetStream(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJetStream failed: %v", err)
	}
	defer bus.Close()

	topic := TopicName("asset_created")

	// Track received messages
	var received atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)

	handler := HandlerFunc(func(ctx context.Context, msg proto.Message) error {
		// For JetStream, we receive RawMessage
		rawMsg, ok := msg.(*RawMessage)
		if !ok {
			t.Errorf("Expected RawMessage, got %T", msg)
			return errors.NewPermanent("invalid message type", nil)
		}

		// Unmarshal to test message
		var testMsg testproto.TestMessage
		if err := proto.Unmarshal(rawMsg.Data, &testMsg); err != nil {
			t.Errorf("Failed to unmarshal: %v", err)
			return err
		}

		if testMsg.Id != "jet-123" {
			t.Errorf("Expected Id=jet-123, got %s", testMsg.Id)
		}

		received.Add(1)
		wg.Done()
		return nil
	})

	// Subscribe
	err = bus.Subscribe(ctx, topic, handler)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Give consumer time to start
	time.Sleep(100 * time.Millisecond)

	// Publish a message
	msg := createTestMessage("jet-123", "JetStream", 42)
	err = bus.Publish(ctx, topic, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for message with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}

	if received.Load() != 1 {
		t.Errorf("Expected 1 message received, got %d", received.Load())
	}
}

func TestJetStreamEventBus_InvalidConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("no servers", func(t *testing.T) {
		cfg := config.EventBusConfig{
			Backend:    "jetstream",
			Servers:    []string{},
			StreamName: "TEST",
		}

		_, err := NewJetStream(ctx, cfg)
		if err == nil {
			t.Error("Expected error for empty servers")
		}
		if !errors.IsInvalidInput(err) {
			t.Errorf("Expected InvalidInput error, got: %v", err)
		}
	})

	t.Run("no stream name", func(t *testing.T) {
		cfg := config.EventBusConfig{
			Backend:    "jetstream",
			Servers:    []string{"nats://localhost:4222"},
			StreamName: "",
		}

		_, err := NewJetStream(ctx, cfg)
		if err == nil {
			t.Error("Expected error for empty stream name")
		}
		if !errors.IsInvalidInput(err) {
			t.Errorf("Expected InvalidInput error, got: %v", err)
		}
	})
}

func TestJetStreamEventBus_Close(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	ctx := context.Background()
	cfg := config.EventBusConfig{
		Backend:    "jetstream",
		Servers:    []string{ns.ClientURL()},
		StreamName: "TEST_EVENTS",
	}

	bus, err := NewJetStream(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJetStream failed: %v", err)
	}

	// Close the bus
	err = bus.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Try to publish after close - should fail
	topic := TopicName("test")
	msg := createTestMessage("test-1", "AfterClose", 1)
	err = bus.Publish(ctx, topic, msg)
	if err == nil {
		t.Error("Expected error when publishing to closed bus")
	}
	if !errors.IsPermanent(err) {
		t.Error("Expected permanent error for closed bus")
	}
}

func TestRawMessage(t *testing.T) {
	data := []byte("test data")
	raw := &RawMessage{Data: data}

	// Test String method
	str := raw.String()
	if str != "RawMessage{9 bytes}" {
		t.Errorf("Expected 'RawMessage{9 bytes}', got %s", str)
	}

	// Test Reset method
	raw.Reset()
	if raw.Data != nil {
		t.Error("Expected Data to be nil after Reset")
	}

	// Test ProtoMessage method (just verify it doesn't panic)
	raw.ProtoMessage()

	// Test ProtoReflect method
	_ = raw.ProtoReflect()
}

func TestJetStreamEventBus_DefaultValues(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	ctx := context.Background()
	// Config with no optional values set
	cfg := config.EventBusConfig{
		Backend:    "jetstream",
		Servers:    []string{ns.ClientURL()},
		StreamName: "TEST_EVENTS",
	}

	bus, err := NewJetStream(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJetStream failed: %v", err)
	}
	defer bus.Close()

	// Test default getters
	if bus.getMaxDeliver() != 3 {
		t.Errorf("Expected default MaxDeliver=3, got %d", bus.getMaxDeliver())
	}

	if bus.getAckWait() != 30*time.Second {
		t.Errorf("Expected default AckWait=30s, got %v", bus.getAckWait())
	}

	if bus.getMaxAckPending() != 1000 {
		t.Errorf("Expected default MaxAckPending=1000, got %d", bus.getMaxAckPending())
	}

	// Test consumer name generation
	topic := "cqc.events.v1.test_event"
	consumerName := bus.getConsumerName(topic)
	if consumerName != "consumer-test_event" {
		t.Errorf("Expected consumer name 'consumer-test_event', got '%s'", consumerName)
	}
}

func TestJetStreamEventBus_ConfiguredValues(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	ctx := context.Background()
	// Config with all values set
	cfg := config.EventBusConfig{
		Backend:       "jetstream",
		Servers:       []string{ns.ClientURL()},
		StreamName:    "TEST_EVENTS",
		ConsumerName:  "my_consumer",
		MaxDeliver:    5,
		AckWait:       10 * time.Second,
		MaxAckPending: 500,
	}

	bus, err := NewJetStream(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJetStream failed: %v", err)
	}
	defer bus.Close()

	// Test configured getters
	if bus.getMaxDeliver() != 5 {
		t.Errorf("Expected MaxDeliver=5, got %d", bus.getMaxDeliver())
	}

	if bus.getAckWait() != 10*time.Second {
		t.Errorf("Expected AckWait=10s, got %v", bus.getAckWait())
	}

	if bus.getMaxAckPending() != 500 {
		t.Errorf("Expected MaxAckPending=500, got %d", bus.getMaxAckPending())
	}

	// Test consumer name with prefix
	topic := "cqc.events.v1.test_event"
	consumerName := bus.getConsumerName(topic)
	if consumerName != "my_consumer-test_event" {
		t.Errorf("Expected consumer name 'my_consumer-test_event', got '%s'", consumerName)
	}
}

func TestJetStreamEventBus_DoubleClose(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	ctx := context.Background()
	cfg := config.EventBusConfig{
		Backend:    "jetstream",
		Servers:    []string{ns.ClientURL()},
		StreamName: "TEST_EVENTS",
	}

	bus, err := NewJetStream(ctx, cfg)
	if err != nil {
		t.Fatalf("NewJetStream failed: %v", err)
	}

	// Close once
	err = bus.Close()
	if err != nil {
		t.Fatalf("First close failed: %v", err)
	}

	// Close again - should be idempotent
	err = bus.Close()
	if err != nil {
		t.Errorf("Second close should not error, got: %v", err)
	}
}
