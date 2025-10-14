package bus

import (
	"context"
	"fmt"
	"sync"

	"github.com/Combine-Capital/cqi/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// MemoryEventBus is an in-memory implementation of EventBus using Go channels.
// It is designed for testing and development, not for production use.
// Messages are delivered synchronously within the same process.
type MemoryEventBus struct {
	mu            sync.RWMutex
	subscriptions map[string][]subscription
	closed        bool
}

// subscription represents a single subscriber to a topic.
type subscription struct {
	handler HandlerFunc
	ch      chan proto.Message
	done    chan struct{}
}

// NewMemory creates a new in-memory event bus.
// This is ideal for testing and development where you don't need distributed messaging.
//
// Example:
//
//	bus := bus.NewMemory()
//	defer bus.Close()
func NewMemory() *MemoryEventBus {
	return &MemoryEventBus{
		subscriptions: make(map[string][]subscription),
	}
}

// Publish sends a message to all subscribers of the given topic.
// The message is delivered synchronously to all handlers.
// Returns an error if the bus is closed.
func (m *MemoryEventBus) Publish(ctx context.Context, topic string, message proto.Message) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return errors.NewPermanent("event bus is closed", nil)
	}

	subs, ok := m.subscriptions[topic]
	if !ok || len(subs) == 0 {
		// No subscribers for this topic, silently succeed
		return nil
	}

	// Clone the message to prevent concurrent modification
	cloned := proto.Clone(message)

	// Send to all subscribers
	for _, sub := range subs {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "publish cancelled")
		case sub.ch <- cloned:
			// Message sent successfully
		case <-sub.done:
			// Subscriber has been closed, skip
			continue
		}
	}

	return nil
}

// Subscribe registers a handler for the given topic.
// Messages published to this topic will be delivered to the handler.
// The handler runs in a separate goroutine.
func (m *MemoryEventBus) Subscribe(ctx context.Context, topic string, handler HandlerFunc, options ...SubscribeOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.NewPermanent("event bus is closed", nil)
	}

	// Apply middleware to handler
	opts := buildOptions(options)
	if len(opts.middlewares) > 0 {
		handler = applyMiddleware(handler, opts.middlewares)
	}

	// Create subscription
	sub := subscription{
		handler: handler,
		ch:      make(chan proto.Message, 100), // Buffer to prevent blocking publishers
		done:    make(chan struct{}),
	}

	// Add to subscriptions
	m.subscriptions[topic] = append(m.subscriptions[topic], sub)

	// Start handler goroutine
	go m.runHandler(ctx, topic, sub)

	return nil
}

// runHandler processes messages for a subscription.
func (m *MemoryEventBus) runHandler(ctx context.Context, topic string, sub subscription) {
	defer close(sub.done)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub.ch:
			if !ok {
				return
			}

			// Invoke handler with error handling
			if err := sub.handler(ctx, msg); err != nil {
				// Log error but continue processing
				// In production, this would be logged via zerolog
				fmt.Printf("handler error for topic %s: %v\n", topic, err)
			}
		}
	}
}

// Close shuts down the event bus and closes all subscriptions.
func (m *MemoryEventBus) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true

	// Close all subscription channels
	for _, subs := range m.subscriptions {
		for _, sub := range subs {
			close(sub.ch)
		}
	}

	// Clear subscriptions
	m.subscriptions = make(map[string][]subscription)

	return nil
}

// Check implements the health.Checker interface for the in-memory event bus.
// The in-memory bus is always healthy unless it has been closed.
//
// Example usage:
//
//	import "github.com/Combine-Capital/cqi/pkg/health"
//
//	h := health.New()
//	h.RegisterChecker("event_bus", memoryBus)
func (m *MemoryEventBus) Check(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return fmt.Errorf("event bus is closed")
	}

	return nil
}
