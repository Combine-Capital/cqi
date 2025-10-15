package websocket

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
)

// Pool manages multiple WebSocket connections for load distribution and redundancy.
type Pool struct {
	config      config.WebSocketConfig
	connections []*Conn
	nextIdx     uint32
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewPool creates a new connection pool with the specified size.
func NewPool(ctx context.Context, cfg config.WebSocketConfig) (*Pool, error) {
	if cfg.PoolSize < 1 {
		return nil, errors.NewInvalidInput("pool_size", "must be at least 1")
	}

	poolCtx, cancel := context.WithCancel(ctx)

	pool := &Pool{
		config:      cfg,
		connections: make([]*Conn, 0, cfg.PoolSize),
		ctx:         poolCtx,
		cancel:      cancel,
	}

	return pool, nil
}

// Connect establishes all connections in the pool.
func (p *Pool) Connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error

	for i := 0; i < p.config.PoolSize; i++ {
		conn, err := NewConn(ctx, p.config)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		p.connections = append(p.connections, conn)
	}

	if len(p.connections) == 0 {
		return errors.NewPermanent("failed to create any connections", errs[0])
	}

	return nil
}

// Get returns the next available connection using round-robin distribution.
func (p *Pool) Get() *Conn {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.connections) == 0 {
		return nil
	}

	// Round-robin selection
	idx := atomic.AddUint32(&p.nextIdx, 1) % uint32(len(p.connections))
	return p.connections[idx]
}

// GetHealthy returns a healthy connection, skipping disconnected ones.
func (p *Pool) GetHealthy() *Conn {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.connections) == 0 {
		return nil
	}

	// Try to find a connected connection
	startIdx := atomic.AddUint32(&p.nextIdx, 1) % uint32(len(p.connections))

	for i := 0; i < len(p.connections); i++ {
		idx := (int(startIdx) + i) % len(p.connections)
		conn := p.connections[idx]
		if conn.IsConnected() {
			return conn
		}
	}

	// No healthy connections, return first one (it will trigger reconnect)
	return p.connections[0]
}

// Size returns the number of connections in the pool.
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections)
}

// HealthyCount returns the number of connected connections in the pool.
func (p *Pool) HealthyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, conn := range p.connections {
		if conn.IsConnected() {
			count++
		}
	}
	return count
}

// Close closes all connections in the pool.
func (p *Pool) Close() error {
	p.cancel()

	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error

	for _, conn := range p.connections {
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Wrap(errs[0], "failed to close all connections")
	}

	return nil
}
