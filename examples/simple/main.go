// Simple example demonstrating minimal CQI usage with config, logging, and database.
//
// To run:
//  1. Start PostgreSQL: docker run --rm -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:14
//  2. Set environment: export SIMPLE_DATABASE_PASSWORD=postgres
//  3. Run: go run main.go
package main

import (
	"context"
	"log"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/database"
	"github.com/Combine-Capital/cqi/pkg/logging"
)

func main() {
	ctx := context.Background()

	// Load configuration from config.yaml and environment variables with SIMPLE_ prefix
	cfg := config.MustLoad("config.yaml", "SIMPLE")

	// Initialize structured logger with configured log level
	logger := logging.New(cfg.Log)
	logger.Info().Msg("Simple service starting")

	// Initialize database connection pool with context
	db, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Verify database connectivity
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		logger.Error().Err(err).Msg("Database connection failed")
		log.Fatal(err)
	}

	logger.Info().Msg("Database connection successful")

	// Execute a simple query
	var version string
	if err := db.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		logger.Error().Err(err).Msg("Query failed")
		log.Fatal(err)
	}

	logger.Info().Str("version", version).Msg("PostgreSQL version")

	// Create a test table
	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS simple_test (
			id SERIAL PRIMARY KEY,
			message TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		logger.Error().Err(err).Msg("Table creation failed")
		log.Fatal(err)
	}

	// Insert test data
	var id int
	err = db.QueryRow(ctx,
		"INSERT INTO simple_test (message) VALUES ($1) RETURNING id",
		"Hello from CQI!",
	).Scan(&id)
	if err != nil {
		logger.Error().Err(err).Msg("Insert failed")
		log.Fatal(err)
	}

	logger.Info().Int("id", id).Msg("Successfully inserted test record")

	// Query the data back
	var message string
	var createdAt time.Time
	err = db.QueryRow(ctx,
		"SELECT message, created_at FROM simple_test WHERE id = $1",
		id,
	).Scan(&message, &createdAt)
	if err != nil {
		logger.Error().Err(err).Msg("Query failed")
		log.Fatal(err)
	}

	logger.Info().
		Str("message", message).
		Time("created_at", createdAt).
		Msg("Successfully retrieved test record")

	// Demonstrate transaction with WithTransaction helper
	err = database.WithTransaction(ctx, db, func(tx database.Transaction) error {
		// Insert multiple records in a transaction
		_, err := tx.Exec(ctx,
			"INSERT INTO simple_test (message) VALUES ($1), ($2)",
			"Transaction message 1",
			"Transaction message 2",
		)
		return err
	})
	if err != nil {
		logger.Error().Err(err).Msg("Transaction failed")
		log.Fatal(err)
	}

	logger.Info().Msg("Transaction completed successfully")

	// Count all records
	var count int
	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM simple_test").Scan(&count)
	if err != nil {
		logger.Error().Err(err).Msg("Count query failed")
		log.Fatal(err)
	}

	logger.Info().Int("total_records", count).Msg("Simple example completed successfully")
}
