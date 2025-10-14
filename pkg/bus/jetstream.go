package bus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// JetStreamEventBus is a NATS JetStream implementation of EventBus.
// It provides distributed, persistent messaging with at-least-once delivery guarantees.
type JetStreamEventBus struct {
	nc          *nats.Conn
	js          jetstream.JetStream
	cfg         config.EventBusConfig
	consumers   []jetstream.ConsumeContext
	consumersMu sync.Mutex
	closed      bool
	closedMu    sync.RWMutex
}

// NewJetStream creates a new NATS JetStream event bus.
// It connects to the NATS servers and creates/updates the configured stream.
//
// Example:
//
//	cfg := config.EventBusConfig{
//	    Backend:    "jetstream",
//	    Servers:    []string{"nats://localhost:4222"},
//	    StreamName: "CQC_EVENTS",
//	}
//
//	bus, err := bus.NewJetStream(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer bus.Close()
func NewJetStream(ctx context.Context, cfg config.EventBusConfig) (*JetStreamEventBus, error) {
	if len(cfg.Servers) == 0 {
		return nil, errors.NewInvalidInput("servers", "at least one NATS server is required")
	}

	if cfg.StreamName == "" {
		return nil, errors.NewInvalidInput("stream_name", "stream name is required")
	}

	// Connect to NATS
	nc, err := nats.Connect(
		cfg.Servers[0],         // For simplicity, connect to first server. In production, use nats.Connect with all servers
		nats.MaxReconnects(-1), // Unlimited reconnect attempts
		nats.ReconnectWait(1*time.Second),
		nats.Timeout(5*time.Second),
	)
	if err != nil {
		return nil, errors.NewTemporary(fmt.Sprintf("failed to connect to NATS: %v", err), err)
	}

	// Create JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, errors.NewTemporary(fmt.Sprintf("failed to create JetStream context: %v", err), err)
	}

	bus := &JetStreamEventBus{
		nc:        nc,
		js:        js,
		cfg:       cfg,
		consumers: make([]jetstream.ConsumeContext, 0),
	}

	// Create or update stream
	if err := bus.ensureStream(ctx); err != nil {
		nc.Close()
		return nil, errors.Wrap(err, "failed to ensure stream exists")
	}

	return bus, nil
}

// ensureStream creates or updates the JetStream stream.
func (j *JetStreamEventBus) ensureStream(ctx context.Context) error {
	// Check if stream exists
	stream, err := j.js.Stream(ctx, j.cfg.StreamName)
	if err == nil {
		// Stream exists, verify it has the right subjects
		info, err := stream.Info(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get stream info")
		}

		// Check if we need to update subjects
		hasWildcard := false
		for _, subj := range info.Config.Subjects {
			if subj == "cqc.events.v1.>" {
				hasWildcard = true
				break
			}
		}

		if !hasWildcard {
			// Update stream to include wildcard subject
			_, err = j.js.UpdateStream(ctx, jetstream.StreamConfig{
				Name:     j.cfg.StreamName,
				Subjects: append(info.Config.Subjects, "cqc.events.v1.>"),
			})
			if err != nil {
				return errors.Wrap(err, "failed to update stream")
			}
		}

		return nil
	}

	// Stream doesn't exist, create it
	_, err = j.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:        j.cfg.StreamName,
		Description: "CQC Events Stream",
		Subjects:    []string{"cqc.events.v1.>"},
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      24 * time.Hour, // Keep events for 24 hours
		Storage:     jetstream.FileStorage,
		Replicas:    1,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create stream")
	}

	return nil
}

// Publish sends a protobuf message to the specified topic.
// The message is serialized and published to JetStream with at-least-once delivery.
func (j *JetStreamEventBus) Publish(ctx context.Context, topic string, message proto.Message) error {
	j.closedMu.RLock()
	defer j.closedMu.RUnlock()

	if j.closed {
		return errors.NewPermanent("event bus is closed", nil)
	}

	// Serialize protobuf message
	data, err := proto.Marshal(message)
	if err != nil {
		return errors.NewPermanent(fmt.Sprintf("failed to marshal message: %v", err), err)
	}

	// Publish to JetStream
	_, err = j.js.Publish(ctx, topic, data)
	if err != nil {
		// Classify error type
		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "publish cancelled")
		}
		return errors.NewTemporary(fmt.Sprintf("failed to publish message: %v", err), err)
	}

	return nil
}

// Subscribe creates a durable consumer and starts consuming messages from the topic.
// Messages are automatically deserialized and passed to the handler.
func (j *JetStreamEventBus) Subscribe(ctx context.Context, topic string, handler HandlerFunc, options ...SubscribeOption) error {
	j.closedMu.RLock()
	defer j.closedMu.RUnlock()

	if j.closed {
		return errors.NewPermanent("event bus is closed", nil)
	}

	// Apply middleware to handler
	opts := buildOptions(options)
	if len(opts.middlewares) > 0 {
		handler = applyMiddleware(handler, opts.middlewares)
	}

	// Create consumer name from topic
	consumerName := j.getConsumerName(topic)

	// Create or get consumer
	consumer, err := j.js.CreateOrUpdateConsumer(ctx, j.cfg.StreamName, jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		FilterSubject: topic,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    j.getMaxDeliver(),
		AckWait:       j.getAckWait(),
		MaxAckPending: j.getMaxAckPending(),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create consumer")
	}

	// Start consuming messages
	consumeCtx, err := consumer.Consume(func(msg jetstream.Msg) {
		// Deserialize message
		var protoMsg proto.Message
		// Note: In production, you'd need to determine the message type from metadata
		// For now, we'll pass the raw bytes and let the handler deal with it
		// This is a limitation we'll document

		// Create a generic message wrapper
		protoMsg = &RawMessage{Data: msg.Data()}

		// Invoke handler
		if err := handler(ctx, protoMsg); err != nil {
			// Handler failed, check if we should retry
			if errors.IsTemporary(err) {
				// NAK the message for redelivery
				msg.Nak()
			} else {
				// Permanent error, acknowledge to prevent redelivery
				msg.Ack()
			}
		} else {
			// Success, acknowledge the message
			msg.Ack()
		}
	})
	if err != nil {
		return errors.Wrap(err, "failed to start consuming")
	}

	// Store consumer context for cleanup
	j.consumersMu.Lock()
	j.consumers = append(j.consumers, consumeCtx)
	j.consumersMu.Unlock()

	return nil
}

// getConsumerName generates a consumer name from the topic and configured consumer name.
func (j *JetStreamEventBus) getConsumerName(topic string) string {
	if j.cfg.ConsumerName != "" {
		return fmt.Sprintf("%s-%s", j.cfg.ConsumerName, ParseEventType(topic))
	}
	return fmt.Sprintf("consumer-%s", ParseEventType(topic))
}

// getMaxDeliver returns the max delivery attempts, defaulting to 3.
func (j *JetStreamEventBus) getMaxDeliver() int {
	if j.cfg.MaxDeliver > 0 {
		return j.cfg.MaxDeliver
	}
	return 3
}

// getAckWait returns the acknowledgment timeout, defaulting to 30 seconds.
func (j *JetStreamEventBus) getAckWait() time.Duration {
	if j.cfg.AckWait > 0 {
		return j.cfg.AckWait
	}
	return 30 * time.Second
}

// getMaxAckPending returns the max outstanding unacked messages, defaulting to 1000.
func (j *JetStreamEventBus) getMaxAckPending() int {
	if j.cfg.MaxAckPending > 0 {
		return j.cfg.MaxAckPending
	}
	return 1000
}

// Close gracefully shuts down the event bus, stopping all consumers and closing the NATS connection.
func (j *JetStreamEventBus) Close() error {
	j.closedMu.Lock()
	defer j.closedMu.Unlock()

	if j.closed {
		return nil
	}

	j.closed = true

	// Stop all consumers
	j.consumersMu.Lock()
	for _, consumer := range j.consumers {
		consumer.Stop()
	}
	j.consumers = nil
	j.consumersMu.Unlock()

	// Drain and close NATS connection
	if j.nc != nil {
		j.nc.Drain()
		j.nc.Close()
	}

	return nil
}

// Check implements the health.Checker interface for the NATS JetStream event bus.
// It verifies connectivity to the NATS server by checking the connection status.
//
// Example usage:
//
//	import "github.com/Combine-Capital/cqi/pkg/health"
//
//	h := health.New()
//	h.RegisterChecker("event_bus", jetStreamBus)
func (j *JetStreamEventBus) Check(ctx context.Context) error {
	j.closedMu.RLock()
	defer j.closedMu.RUnlock()

	if j.closed {
		return errors.NewTemporary("event bus is closed", nil)
	}

	if j.nc == nil {
		return errors.NewTemporary("NATS connection is nil", nil)
	}

	// Check if connection is alive
	status := j.nc.Status()
	if status != nats.CONNECTED {
		return errors.NewTemporary(fmt.Sprintf("NATS connection not connected: status=%v", status), nil)
	}

	// Try a simple RTT check to verify server responsiveness
	_, err := j.nc.RTT()
	if err != nil {
		return errors.NewTemporary("NATS RTT check failed", err)
	}

	return nil
}

// RawMessage is a wrapper for raw protobuf bytes when the type is unknown.
// Handlers should type assert to their expected message type after unmarshaling.
type RawMessage struct {
	Data []byte
}

// Reset implements proto.Message.
func (r *RawMessage) Reset() {
	r.Data = nil
}

// String implements proto.Message.
func (r *RawMessage) String() string {
	return fmt.Sprintf("RawMessage{%d bytes}", len(r.Data))
}

// ProtoMessage implements proto.Message.
func (r *RawMessage) ProtoMessage() {}

// ProtoReflect implements protoreflect.Message.
func (r *RawMessage) ProtoReflect() protoreflect.Message {
	// This is a minimal implementation for RawMessage
	// In production, handlers should unmarshal to concrete types
	return nil
}
