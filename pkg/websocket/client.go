package websocket

// Package websocket provides WebSocket client functionality with auto-reconnect,
// connection pooling, and message handler framework for CQI infrastructure.
// It wraps gorilla/websocket with CQI patterns for observability and reliability.
//
// Example usage:
//
//	cfg := config.WebSocketConfig{
//	    URL:                "wss://api.example.com/ws",
//	    ReconnectMaxAttempts: 5,
//	    ReconnectInitialDelay: 1 * time.Second,
//	    PingInterval:        30 * time.Second,
//	}
//
//	client, err := websocket.New(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Register message handlers
//	client.RegisterHandler("trade", handleTrade)
//	client.RegisterHandler("orderbook", handleOrderBook)
//
//	// Connect
//	if err := client.Connect(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Send message
//	msg := map[string]string{"type": "subscribe", "channel": "trades"}
//	if err := client.Send(ctx, msg); err != nil {
//	    log.Fatal(err)
//	}
import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/gorilla/websocket"
)

// Client provides WebSocket client functionality with auto-reconnect,
// message handling, and connection pooling.
type Client struct {
	config   config.WebSocketConfig
	conn     *Conn
	pool     *Pool
	handlers *HandlerRegistry
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// Message represents a WebSocket message with type information.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Raw     []byte
}

// HandlerFunc is a function that processes WebSocket messages.
type HandlerFunc func(context.Context, *Message) error

// New creates a new WebSocket client with the provided configuration.
func New(ctx context.Context, cfg config.WebSocketConfig) (*Client, error) {
	// Apply defaults
	cfg = applyDefaults(cfg)

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, errors.Wrap(err, "invalid websocket config")
	}

	// Create client context
	clientCtx, cancel := context.WithCancel(ctx)

	client := &Client{
		config:   cfg,
		handlers: NewHandlerRegistry(),
		ctx:      clientCtx,
		cancel:   cancel,
	}

	// Initialize connection pool if enabled
	if cfg.PoolSize > 1 {
		pool, err := NewPool(clientCtx, cfg)
		if err != nil {
			cancel()
			return nil, errors.Wrap(err, "failed to create connection pool")
		}
		client.pool = pool
	}

	return client, nil
}

// Connect establishes a WebSocket connection to the configured URL.
// It will automatically retry on failure according to the reconnect configuration.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If using pool, connect all connections
	if c.pool != nil {
		return c.pool.Connect(ctx)
	}

	// Create single connection
	conn, err := NewConn(ctx, c.config)
	if err != nil {
		return errors.Wrap(err, "failed to create connection")
	}

	c.conn = conn

	// Start message handler
	c.wg.Add(1)
	go c.handleMessages()

	return nil
}

// Close gracefully closes the WebSocket client and all connections.
// It waits for in-flight messages to be processed before closing.
func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error

	// Close pool if exists
	if c.pool != nil {
		if err := c.pool.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Close single connection if exists
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Wrap(errs[0], "failed to close client")
	}

	return nil
}

// Send sends a message to the WebSocket server.
// The message is JSON-encoded before sending.
func (c *Client) Send(ctx context.Context, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	return c.SendRaw(ctx, data)
}

// SendRaw sends raw bytes to the WebSocket server.
func (c *Client) SendRaw(ctx context.Context, data []byte) error {
	c.mu.RLock()
	conn := c.getActiveConn()
	c.mu.RUnlock()

	if conn == nil {
		return errors.NewTemporary("no active connection", nil)
	}

	return conn.Send(ctx, websocket.TextMessage, data)
}

// Receive blocks until a message is received or the context is canceled.
// It returns the raw message data.
func (c *Client) Receive(ctx context.Context) (*Message, error) {
	c.mu.RLock()
	conn := c.getActiveConn()
	c.mu.RUnlock()

	if conn == nil {
		return nil, errors.NewTemporary("no active connection", nil)
	}

	_, data, err := conn.Receive(ctx)
	if err != nil {
		return nil, err
	}

	return parseMessage(data), nil
}

// RegisterHandler registers a message handler for a specific message type.
// When a message with the matching type is received, the handler will be called.
func (c *Client) RegisterHandler(messageType string, handler HandlerFunc) {
	c.handlers.Register(messageType, handler)
}

// UnregisterHandler removes a message handler for a specific message type.
func (c *Client) UnregisterHandler(messageType string) {
	c.handlers.Unregister(messageType)
}

// IsConnected returns true if the client has an active connection.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.getActiveConn() != nil && c.getActiveConn().IsConnected()
}

// handleMessages processes incoming messages and dispatches them to registered handlers.
func (c *Client) handleMessages() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Receive message
		msg, err := c.Receive(c.ctx)
		if err != nil {
			if errors.IsTemporary(err) || c.ctx.Err() != nil {
				// Connection error or context canceled, exit loop
				return
			}
			// Log error and continue
			continue
		}

		// Dispatch to handler
		if err := c.handlers.Handle(c.ctx, msg); err != nil {
			// Log error but don't stop processing
			continue
		}
	}
}

// getActiveConn returns the active connection (from pool or single connection).
// Must be called with read lock held.
func (c *Client) getActiveConn() *Conn {
	if c.pool != nil {
		return c.pool.Get()
	}
	return c.conn
}

// parseMessage parses raw message data into a Message struct.
func parseMessage(data []byte) *Message {
	msg := &Message{Raw: data}

	// Try to parse as JSON with type field
	var typed struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typed); err == nil && typed.Type != "" {
		msg.Type = typed.Type
		msg.Payload = data
	}

	return msg
}

// applyDefaults applies default values to the configuration.
func applyDefaults(cfg config.WebSocketConfig) config.WebSocketConfig {
	if cfg.ReconnectInitialDelay == 0 {
		cfg.ReconnectInitialDelay = 1 * time.Second
	}
	if cfg.ReconnectMaxDelay == 0 {
		cfg.ReconnectMaxDelay = 32 * time.Second
	}
	if cfg.ReconnectMaxAttempts == 0 {
		cfg.ReconnectMaxAttempts = 5
	}
	if cfg.PingInterval == 0 {
		cfg.PingInterval = 30 * time.Second
	}
	if cfg.PongWait == 0 {
		cfg.PongWait = 60 * time.Second
	}
	if cfg.WriteWait == 0 {
		cfg.WriteWait = 10 * time.Second
	}
	if cfg.MessageBufferSize == 0 {
		cfg.MessageBufferSize = 256
	}
	if cfg.ReadBufferSize == 0 {
		cfg.ReadBufferSize = 1024
	}
	if cfg.WriteBufferSize == 0 {
		cfg.WriteBufferSize = 1024
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 1
	}

	return cfg
}

// validateConfig validates the WebSocket configuration.
func validateConfig(cfg config.WebSocketConfig) error {
	if cfg.URL == "" {
		return errors.NewInvalidInput("url", "is required")
	}

	if cfg.ReconnectMaxAttempts < 0 {
		return errors.NewInvalidInput("reconnect_max_attempts", "must be non-negative")
	}

	if cfg.PingInterval < 0 {
		return errors.NewInvalidInput("ping_interval", "must be non-negative")
	}

	if cfg.MessageBufferSize < 0 {
		return errors.NewInvalidInput("message_buffer_size", "must be non-negative")
	}

	if cfg.PoolSize < 1 {
		return errors.NewInvalidInput("pool_size", "must be at least 1")
	}

	return nil
}
