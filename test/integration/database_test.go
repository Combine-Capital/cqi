// Package integration provides integration tests for CQI infrastructure components.
// These tests require real infrastructure services (PostgreSQL, Redis, NATS).
//
// Run with docker-compose:
//
//	cd test/integration
//	docker-compose up -d
//	go test -v ./...
//	docker-compose down
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/database"
	"github.com/Combine-Capital/cqi/pkg/errors"
)

// TestDatabaseConnection tests basic database connectivity.
func TestDatabaseConnection(t *testing.T) {
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:           "localhost",
		Port:           5432,
		Database:       "cqi_test",
		User:           "postgres",
		Password:       "postgres",
		SSLMode:        "disable",
		MaxConns:       10,
		MinConns:       2,
		ConnectTimeout: 10 * time.Second,
		QueryTimeout:   5 * time.Second,
	}

	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create database pool: %v", err)
	}
	defer pool.Close()

	// Test basic connectivity
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}
}

// TestDatabaseQuery tests query operations.
func TestDatabaseQuery(t *testing.T) {
	ctx := context.Background()

	pool := setupDatabase(t, ctx)
	defer pool.Close()

	// Create test table
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_query (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			value INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer pool.Exec(ctx, "DROP TABLE IF EXISTS test_query")

	// Test QueryRow with INSERT RETURNING
	var id int
	err = pool.QueryRow(ctx,
		"INSERT INTO test_query (name, value) VALUES ($1, $2) RETURNING id",
		"test1", 42,
	).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert with RETURNING: %v", err)
	}
	if id != 1 {
		t.Errorf("Expected id=1, got id=%d", id)
	}

	// Test Query with multiple rows
	_, err = pool.Exec(ctx,
		"INSERT INTO test_query (name, value) VALUES ($1, $2), ($3, $4)",
		"test2", 100, "test3", 200,
	)
	if err != nil {
		t.Fatalf("Failed to insert multiple rows: %v", err)
	}

	rows, err := pool.Query(ctx, "SELECT id, name, value FROM test_query ORDER BY id")
	if err != nil {
		t.Fatalf("Failed to query rows: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		var name string
		var value int
		if err := rows.Scan(&id, &name, &value); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("Row iteration error: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 rows, got %d", count)
	}
}

// TestDatabaseExec tests execution operations.
func TestDatabaseExec(t *testing.T) {
	ctx := context.Background()

	pool := setupDatabase(t, ctx)
	defer pool.Close()

	// Create test table
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_exec (
			id SERIAL PRIMARY KEY,
			value INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer pool.Exec(ctx, "DROP TABLE IF EXISTS test_exec")

	// Test Exec with INSERT
	result, err := pool.Exec(ctx, "INSERT INTO test_exec (value) VALUES ($1), ($2), ($3)", 1, 2, 3)
	if err != nil {
		t.Fatalf("Failed to exec insert: %v", err)
	}
	if result.RowsAffected() != 3 {
		t.Errorf("Expected 3 rows affected, got %d", result.RowsAffected())
	}

	// Test Exec with UPDATE
	result, err = pool.Exec(ctx, "UPDATE test_exec SET value = value * 2 WHERE value > $1", 1)
	if err != nil {
		t.Fatalf("Failed to exec update: %v", err)
	}
	if result.RowsAffected() != 2 {
		t.Errorf("Expected 2 rows affected, got %d", result.RowsAffected())
	}

	// Test Exec with DELETE
	result, err = pool.Exec(ctx, "DELETE FROM test_exec WHERE value < $1", 3)
	if err != nil {
		t.Fatalf("Failed to exec delete: %v", err)
	}
	if result.RowsAffected() != 1 {
		t.Errorf("Expected 1 row affected, got %d", result.RowsAffected())
	}
}

// TestDatabaseTransaction tests transaction operations.
func TestDatabaseTransaction(t *testing.T) {
	ctx := context.Background()

	pool := setupDatabase(t, ctx)
	defer pool.Close()

	// Create test table
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_tx (
			id SERIAL PRIMARY KEY,
			value INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer pool.Exec(ctx, "DROP TABLE IF EXISTS test_tx")

	// Test successful transaction
	err = database.WithTransaction(ctx, pool, func(tx database.Transaction) error {
		_, err := tx.Exec(ctx, "INSERT INTO test_tx (value) VALUES ($1), ($2)", 1, 2)
		return err
	})
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify rows were committed
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_tx").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 rows after commit, got %d", count)
	}

	// Test rolled back transaction
	err = database.WithTransaction(ctx, pool, func(tx database.Transaction) error {
		_, err := tx.Exec(ctx, "INSERT INTO test_tx (value) VALUES ($1)", 3)
		if err != nil {
			return err
		}
		return errors.NewPermanent("intentional rollback", nil)
	})
	if err == nil {
		t.Fatal("Expected transaction to fail")
	}

	// Verify rows were NOT committed
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_tx").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 rows after rollback, got %d", count)
	}
}

// TestDatabaseContextCancellation tests context cancellation handling.
func TestDatabaseContextCancellation(t *testing.T) {
	ctx := context.Background()

	pool := setupDatabase(t, ctx)
	defer pool.Close()

	// Create a context with a very short timeout
	queryCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()

	// This query should fail due to context cancellation
	time.Sleep(2 * time.Millisecond) // Ensure timeout happens
	var result int
	err := pool.QueryRow(queryCtx, "SELECT 1").Scan(&result)
	if err == nil {
		t.Fatal("Expected query to fail due to context cancellation")
	}
}

// TestDatabaseHealthCheck tests health checking.
func TestDatabaseHealthCheck(t *testing.T) {
	ctx := context.Background()

	pool := setupDatabase(t, ctx)
	defer pool.Close()

	// Test successful health check
	if err := database.CheckHealth(ctx, pool); err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	// Test health check with timeout
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := database.CheckHealth(healthCtx, pool); err != nil {
		t.Fatalf("Health check with timeout failed: %v", err)
	}
}

// setupDatabase creates a database pool for testing.
func setupDatabase(t *testing.T, ctx context.Context) *database.Pool {
	t.Helper()

	cfg := config.DatabaseConfig{
		Host:           "localhost",
		Port:           5432,
		Database:       "cqi_test",
		User:           "postgres",
		Password:       "postgres",
		SSLMode:        "disable",
		MaxConns:       10,
		MinConns:       2,
		ConnectTimeout: 10 * time.Second,
		QueryTimeout:   30 * time.Second,
	}

	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create database pool: %v", err)
	}

	return pool
}
