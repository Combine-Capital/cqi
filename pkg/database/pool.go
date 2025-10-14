package database

import (
	"context"
	"fmt"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolInterface defines the interface for a connection pool.
// This allows for easier testing with mock implementations.
type PoolInterface interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Ping(ctx context.Context) error
	Close()
	Stat() *pgxpool.Stat
}

// Pool wraps pgxpool.Pool and implements the Database interface.
// It manages a pool of connections to PostgreSQL with automatic reconnection
// and configurable connection limits.
type Pool struct {
	pool PoolInterface
}

// NewPool creates a new connection pool from the provided configuration.
// It establishes connections to PostgreSQL with the configured limits,
// timeouts, and SSL settings.
//
// The context is used for the initial connection attempt. If the context
// is canceled or times out before the connection is established, an error
// is returned.
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*Pool, error) {
	// Build connection string
	connStr := buildConnString(cfg)

	// Parse config to customize pool settings
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	// Configure connection pool limits
	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = int32(cfg.MaxConns)
	}
	if cfg.MinConns > 0 {
		poolConfig.MinConns = int32(cfg.MinConns)
	}

	// Configure connection lifetimes
	if cfg.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	}

	// Configure connection timeout
	if cfg.ConnectTimeout > 0 {
		poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}

	// Create the pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Ping to verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Pool{pool: pool}, nil
}

// buildConnString constructs a PostgreSQL connection string from the config.
func buildConnString(cfg config.DatabaseConfig) string {
	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s",
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.User,
		cfg.Password,
	)

	// Add SSL mode if specified
	if cfg.SSLMode != "" {
		connStr += fmt.Sprintf(" sslmode=%s", cfg.SSLMode)
	}

	// Add connect timeout if specified
	if cfg.ConnectTimeout > 0 {
		connStr += fmt.Sprintf(" connect_timeout=%d", int(cfg.ConnectTimeout.Seconds()))
	}

	return connStr
}

// Query executes a query that returns rows, typically a SELECT.
func (p *Pool) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}

// QueryRow executes a query that is expected to return at most one row.
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

// Exec executes a query that doesn't return rows, typically INSERT, UPDATE, or DELETE.
func (p *Pool) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, sql, args...)
}

// Begin starts a new transaction.
// The returned Transaction must be committed or rolled back.
func (p *Pool) Begin(ctx context.Context) (Transaction, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &txWrapper{tx: tx}, nil
}

// WithTransaction executes the provided function within a transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
// This ensures proper cleanup even if the function panics.
func (p *Pool) WithTransaction(ctx context.Context, fn TransactionFunc) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}

	// Ensure rollback on panic or error
	defer func() {
		if p := recover(); p != nil {
			// Rollback on panic, but re-panic after
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	// Execute the transaction function
	if err := fn(tx); err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction (original error: %w): %v", err, rbErr)
		}
		return err
	}

	// Commit on success
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Ping verifies a connection to the database is still alive.
func (p *Pool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// Close closes all connections in the pool.
// After calling Close, the pool should not be used.
func (p *Pool) Close() {
	p.pool.Close()
}

// Stats returns connection pool statistics.
func (p *Pool) Stats() *pgxpool.Stat {
	return p.pool.Stat()
}

// Ensure pgxpool.Pool implements PoolInterface at compile time.
var _ PoolInterface = (*pgxpool.Pool)(nil)

// txWrapper wraps pgx.Tx to implement the Transaction interface.
type txWrapper struct {
	tx pgx.Tx
}

// Query executes a query within the transaction.
func (t *txWrapper) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

// QueryRow executes a query within the transaction that returns at most one row.
func (t *txWrapper) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

// Exec executes a query within the transaction that doesn't return rows.
func (t *txWrapper) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, args...)
}

// Commit commits the transaction.
func (t *txWrapper) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

// Rollback aborts the transaction.
func (t *txWrapper) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}
