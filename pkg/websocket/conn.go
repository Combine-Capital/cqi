package websocket

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/gorilla/websocket"
)

// Conn represents a single WebSocket connection with auto-reconnect and ping/pong support.
type Conn struct {
	config     config.WebSocketConfig
	conn       *websocket.Conn
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	reconnectC chan struct{}
	connected  bool
	wg         sync.WaitGroup
}

// NewConn creates a new WebSocket connection with auto-reconnect support.
func NewConn(ctx context.Context, cfg config.WebSocketConfig) (*Conn, error) {
	connCtx, cancel := context.WithCancel(ctx)

	c := &Conn{
		config:     cfg,
		ctx:        connCtx,
		cancel:     cancel,
		reconnectC: make(chan struct{}, 1),
	}

	// Initial connection
	if err := c.connect(); err != nil {
		cancel()
		return nil, errors.Wrap(err, "initial connection failed")
	}

	// Start ping/pong handler
	c.wg.Add(1)
	go c.pingHandler()

	// Start reconnect handler
	c.wg.Add(1)
	go c.reconnectHandler()

	return c, nil
}

// connect establishes a WebSocket connection to the server.
func (c *Conn) connect() error {
	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   c.config.ReadBufferSize,
		WriteBufferSize:  c.config.WriteBufferSize,
	}

	// Build headers
	headers := http.Header{}
	for key, value := range c.config.Headers {
		headers.Set(key, value)
	}

	// Dial connection
	conn, resp, err := dialer.DialContext(c.ctx, c.config.URL, headers)
	if err != nil {
		if resp != nil {
			return errors.NewTemporary("websocket dial failed", err)
		}
		return errors.Wrap(err, "websocket dial failed")
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	// Set read deadline for pong wait
	c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))

	// Set pong handler
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		return nil
	})

	return nil
}

// reconnect attempts to reconnect to the WebSocket server with exponential backoff.
func (c *Conn) reconnect() error {
	attempts := 0
	delay := c.config.ReconnectInitialDelay

	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		default:
		}

		// Check if max attempts reached
		if c.config.ReconnectMaxAttempts > 0 && attempts >= c.config.ReconnectMaxAttempts {
			return errors.NewPermanent("max reconnect attempts reached", nil)
		}

		// Wait before reconnecting
		if attempts > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-c.ctx.Done():
				timer.Stop()
				return c.ctx.Err()
			case <-timer.C:
			}
		}

		// Attempt connection
		if err := c.connect(); err == nil {
			return nil
		}

		// Exponential backoff with max delay
		attempts++
		delay *= 2
		if delay > c.config.ReconnectMaxDelay {
			delay = c.config.ReconnectMaxDelay
		}
	}
}

// reconnectHandler monitors for reconnection requests and handles them.
func (c *Conn) reconnectHandler() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.reconnectC:
			// Close old connection
			c.mu.Lock()
			if c.conn != nil {
				c.conn.Close()
				c.conn = nil
			}
			c.connected = false
			c.mu.Unlock()

			// Attempt reconnection
			c.reconnect()
		}
	}
}

// pingHandler sends ping messages at regular intervals.
func (c *Conn) pingHandler() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				continue
			}

			// Set write deadline
			if err := conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait)); err != nil {
				c.triggerReconnect()
				continue
			}

			// Send ping
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.triggerReconnect()
			}
		}
	}
}

// Send sends a message to the WebSocket server.
func (c *Conn) Send(ctx context.Context, messageType int, data []byte) error {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return errors.NewTemporary("not connected", nil)
	}

	// Set write deadline
	if err := conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait)); err != nil {
		c.triggerReconnect()
		return errors.Wrap(err, "failed to set write deadline")
	}

	// Write message
	if err := conn.WriteMessage(messageType, data); err != nil {
		c.triggerReconnect()
		return errors.Wrap(err, "failed to write message")
	}

	return nil
}

// Receive reads a message from the WebSocket server.
func (c *Conn) Receive(ctx context.Context) (int, []byte, error) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return 0, nil, errors.NewTemporary("not connected", nil)
	}

	// Read message
	messageType, data, err := conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
			c.triggerReconnect()
		}
		return 0, nil, errors.Wrap(err, "failed to read message")
	}

	return messageType, data, nil
}

// Close gracefully closes the WebSocket connection.
func (c *Conn) Close() error {
	c.cancel()
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		// Send close message
		c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

		// Close connection
		err := c.conn.Close()
		c.conn = nil
		c.connected = false
		return err
	}

	return nil
}

// IsConnected returns true if the connection is active.
func (c *Conn) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// triggerReconnect signals the reconnect handler to reconnect.
func (c *Conn) triggerReconnect() {
	select {
	case c.reconnectC <- struct{}{}:
	default:
		// Already a reconnect pending
	}
}
