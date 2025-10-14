package database

import (
	"context"
	"fmt"
)

// BeginTransaction is a convenience function that starts a new transaction from a Pool.
// It's equivalent to calling pool.Begin(ctx).
//
// Example usage:
//
//	tx, err := database.BeginTransaction(ctx, pool)
//	if err != nil {
//	    return err
//	}
//	defer tx.Rollback(ctx) // Rollback if not committed
//
//	// Perform operations
//	_, err = tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
//	if err != nil {
//	    return err
//	}
//
//	// Commit transaction
//	return tx.Commit(ctx)
func BeginTransaction(ctx context.Context, pool *Pool) (Transaction, error) {
	return pool.Begin(ctx)
}

// WithTransaction is a convenience function that executes a function within a transaction.
// It's equivalent to calling pool.WithTransaction(ctx, fn).
//
// The transaction is automatically rolled back if the function returns an error or panics.
// The transaction is committed if the function returns nil.
//
// Example usage:
//
//	err := database.WithTransaction(ctx, pool, func(tx database.Transaction) error {
//	    _, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
//	    if err != nil {
//	        return err
//	    }
//
//	    _, err = tx.Exec(ctx, "INSERT INTO orders (user_id, total) VALUES ($1, $2)", 1, 100.0)
//	    return err
//	})
func WithTransaction(ctx context.Context, pool *Pool, fn TransactionFunc) error {
	return pool.WithTransaction(ctx, fn)
}

// MustCommit commits the transaction and panics if an error occurs.
// This is useful for scenarios where a commit failure is unrecoverable.
//
// Use with caution - prefer proper error handling in production code.
func MustCommit(ctx context.Context, tx Transaction) {
	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Sprintf("failed to commit transaction: %v", err))
	}
}

// MustRollback rolls back the transaction and panics if an error occurs.
// This is useful for cleanup in defer statements where error handling is difficult.
//
// Use with caution - prefer proper error handling in production code.
func MustRollback(ctx context.Context, tx Transaction) {
	if err := tx.Rollback(ctx); err != nil {
		panic(fmt.Sprintf("failed to rollback transaction: %v", err))
	}
}
