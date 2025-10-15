package websocket

import (
	"context"
	"sync"
)

// HandlerRegistry manages message handlers for different message types.
type HandlerRegistry struct {
	handlers map[string]HandlerFunc
	mu       sync.RWMutex
}

// NewHandlerRegistry creates a new handler registry.
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]HandlerFunc),
	}
}

// Register registers a handler for a specific message type.
func (r *HandlerRegistry) Register(messageType string, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[messageType] = handler
}

// Unregister removes a handler for a specific message type.
func (r *HandlerRegistry) Unregister(messageType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, messageType)
}

// Handle dispatches a message to the appropriate handler based on its type.
func (r *HandlerRegistry) Handle(ctx context.Context, msg *Message) error {
	r.mu.RLock()
	handler, ok := r.handlers[msg.Type]
	r.mu.RUnlock()

	if !ok {
		// No handler registered for this message type, silently ignore
		return nil
	}

	return handler(ctx, msg)
}

// Count returns the number of registered handlers.
func (r *HandlerRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}
