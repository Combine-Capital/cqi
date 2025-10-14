// Package database provides PostgreSQL connection pooling with transaction management
// and health checks. It wraps pgxpool for connection pooling with configurable limits,
// timeouts, and automatic reconnection.
//
// Example usage:
//
//	cfg := config.DatabaseConfig{
//	    Host:     "localhost",
//	    Port:     5432,
//	    Database: "mydb",
//	    User:     "user",
//	    Password: "pass",
//	    MaxConns: 10,
//	}
//
//	pool, err := database.NewPool(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.Close()
//
//	// Execute a query
//	rows, err := pool.Query(ctx, "SELECT id, name FROM users WHERE active = $1", true)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer rows.Close()
//
//	// Use transactions
//	err = pool.WithTransaction(ctx, func(tx Transaction) error {
//	    _, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "John")
//	    return err
//	})
package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Database defines the common interface for database operations.
// Both Pool and Transaction implement this interface, allowing for
// consistent API whether working with the connection pool or within a transaction.
type Database interface {
	// Query executes a query that returns rows, typically a SELECT.
	// The args are for any placeholder parameters in the query.
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)

	// QueryRow executes a query that is expected to return at most one row.
	// QueryRow always returns a non-nil value. Errors are deferred until Row's Scan method is called.
	// If the query selects no rows, the *Row's Scan will return ErrNoRows.
	// Otherwise, the *Row's Scan scans the first selected row and discards the rest.
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row

	// Exec executes a query that doesn't return rows, typically INSERT, UPDATE, or DELETE.
	// The args are for any placeholder parameters in the query.
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// Transaction extends Database with transaction control methods.
// It allows committing or rolling back the transaction.
type Transaction interface {
	Database

	// Commit commits the transaction.
	Commit(ctx context.Context) error

	// Rollback aborts the transaction.
	Rollback(ctx context.Context) error
}

// TransactionFunc is a function that performs database operations within a transaction.
// If it returns an error, the transaction will be rolled back.
// If it returns nil, the transaction will be committed.
type TransactionFunc func(tx Transaction) error
