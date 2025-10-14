// Package bus provides event bus functionality for publish/subscribe messaging with CQC protobuf events.
// It supports multiple backends: in-memory for testing and NATS JetStream for production.
//
// Example usage with in-memory backend:
//
//	bus := bus.NewMemory()
//	defer bus.Close()
//
//	// Publish an event
//	event := &events.AssetCreated{Id: "btc", Name: "Bitcoin"}
//	err := bus.Publish(ctx, "cqc.events.v1.asset_created", event)
//
//	// Subscribe to events
//	err = bus.Subscribe(ctx, "cqc.events.v1.asset_created",
//	    bus.HandlerFunc(func(ctx context.Context, msg proto.Message) error {
//	        event := msg.(*events.AssetCreated)
//	        return processAssetCreated(ctx, event)
//	    }),
//	)
//
// Example usage with NATS JetStream:
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
//
//	// Use middleware for automatic retry and logging
//	err = bus.Subscribe(ctx, "cqc.events.v1.price_updated",
//	    handler,
//	    bus.WithRetry(3, time.Second),
//	    bus.WithLogging(logger),
//	    bus.WithMetrics(),
//	)
package bus

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// EventBus defines the interface for publishing and subscribing to protobuf events.
// All methods respect context cancellation and timeout.
type EventBus interface {
	// Publish sends a protobuf message to the specified topic.
	// The message is automatically serialized to wire format before transmission.
	// Returns an error if serialization or publishing fails.
	Publish(ctx context.Context, topic string, message proto.Message) error

	// Subscribe registers a handler for messages on the specified topic.
	// The handler is invoked for each message received on the topic.
	// Messages are automatically deserialized from wire format before handler invocation.
	// Middleware options can be applied to wrap the handler with retry, logging, metrics, etc.
	// Returns an error if subscription fails.
	Subscribe(ctx context.Context, topic string, handler HandlerFunc, options ...SubscribeOption) error

	// Close releases all resources and gracefully shuts down the event bus.
	// This flushes any pending messages and closes connections.
	Close() error
}

// HandlerFunc is the function signature for event handlers.
// Handlers receive the deserialized protobuf message and must return an error
// if processing fails. Returning a temporary error triggers retry if retry middleware is enabled.
type HandlerFunc func(ctx context.Context, message proto.Message) error

// SubscribeOption is a function that modifies subscription behavior.
// Options can add middleware like retry, logging, and metrics to the handler.
type SubscribeOption func(*subscribeOptions)

// subscribeOptions holds the configuration for a subscription.
type subscribeOptions struct {
	middlewares []Middleware
}

// Middleware wraps a HandlerFunc to add cross-cutting concerns like retry, logging, and metrics.
type Middleware func(HandlerFunc) HandlerFunc

// applyMiddleware applies all middleware to the handler in reverse order
// so that the first middleware added is the outermost wrapper.
func applyMiddleware(handler HandlerFunc, middlewares []Middleware) HandlerFunc {
	// Apply in reverse order so first added middleware is outermost
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// buildOptions constructs subscribeOptions from the provided SubscribeOption functions.
func buildOptions(opts []SubscribeOption) *subscribeOptions {
	options := &subscribeOptions{
		middlewares: make([]Middleware, 0),
	}
	for _, opt := range opts {
		opt(options)
	}
	return options
}
