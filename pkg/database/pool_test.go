package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/pashagolub/pgxmock/v4"
)

// TestBuildConnString tests connection string construction
func TestBuildConnString(t *testing.T) {
	tests := []struct {
		name   string
		cfg    config.DatabaseConfig
		expect string
	}{
		{
			name: "Basic connection string",
			cfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				User:     "testuser",
				Password: "testpass",
			},
			expect: "host=localhost port=5432 dbname=testdb user=testuser password=testpass",
		},
		{
			name: "With SSL mode",
			cfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "require",
			},
			expect: "host=localhost port=5432 dbname=testdb user=testuser password=testpass sslmode=require",
		},
		{
			name: "With connect timeout",
			cfg: config.DatabaseConfig{
				Host:           "localhost",
				Port:           5432,
				Database:       "testdb",
				User:           "testuser",
				Password:       "testpass",
				ConnectTimeout: 10 * time.Second,
			},
			expect: "host=localhost port=5432 dbname=testdb user=testuser password=testpass connect_timeout=10",
		},
		{
			name: "Complete configuration",
			cfg: config.DatabaseConfig{
				Host:           "db.example.com",
				Port:           5433,
				Database:       "proddb",
				User:           "admin",
				Password:       "securepass",
				SSLMode:        "verify-full",
				ConnectTimeout: 30 * time.Second,
			},
			expect: "host=db.example.com port=5433 dbname=proddb user=admin password=securepass sslmode=verify-full connect_timeout=30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildConnString(tt.cfg)
			if result != tt.expect {
				t.Errorf("buildConnString() = %v, want %v", result, tt.expect)
			}
		})
	}
}

// TestPoolQuery tests Query method
func TestPoolQuery(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	pool := &Pool{pool: mock}

	t.Run("Successful query", func(t *testing.T) {
		rows := pgxmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice").
			AddRow(2, "Bob")

		mock.ExpectQuery("SELECT (.+) FROM users").
			WillReturnRows(rows)

		ctx := context.Background()
		result, err := pool.Query(ctx, "SELECT id, name FROM users")
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		defer result.Close()

		count := 0
		for result.Next() {
			count++
		}

		if count != 2 {
			t.Errorf("Query() returned %d rows, want 2", count)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Query with parameters", func(t *testing.T) {
		rows := pgxmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice")

		mock.ExpectQuery("SELECT (.+) FROM users WHERE active").
			WithArgs(true).
			WillReturnRows(rows)

		ctx := context.Background()
		result, err := pool.Query(ctx, "SELECT id, name FROM users WHERE active = $1", true)
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		defer result.Close()

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Query error", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM users").
			WillReturnError(errors.New("connection lost"))

		ctx := context.Background()
		_, err := pool.Query(ctx, "SELECT id, name FROM users")
		if err == nil {
			t.Error("Query() expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM users").
			WillReturnError(context.Canceled)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := pool.Query(ctx, "SELECT id, name FROM users")
		if err == nil {
			t.Error("Query() expected error from cancelled context, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

// TestPoolQueryRow tests QueryRow method
func TestPoolQueryRow(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	pool := &Pool{pool: mock}

	t.Run("Successful query row", func(t *testing.T) {
		rows := pgxmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice")

		mock.ExpectQuery("SELECT (.+) FROM users WHERE id").
			WithArgs(1).
			WillReturnRows(rows)

		ctx := context.Background()
		var id int
		var name string
		err := pool.QueryRow(ctx, "SELECT id, name FROM users WHERE id = $1", 1).Scan(&id, &name)
		if err != nil {
			t.Fatalf("QueryRow() error = %v", err)
		}

		if id != 1 || name != "Alice" {
			t.Errorf("QueryRow() got id=%d, name=%s, want id=1, name=Alice", id, name)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("No rows found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM users WHERE id").
			WithArgs(999).
			WillReturnError(errors.New("no rows"))

		ctx := context.Background()
		var id int
		var name string
		err := pool.QueryRow(ctx, "SELECT id, name FROM users WHERE id = $1", 999).Scan(&id, &name)
		if err == nil {
			t.Error("QueryRow() expected error for non-existent row, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

// TestPoolExec tests Exec method
func TestPoolExec(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	pool := &Pool{pool: mock}

	t.Run("Successful insert", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO users").
			WithArgs("Alice", "alice@example.com").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		ctx := context.Background()
		tag, err := pool.Exec(ctx, "INSERT INTO users (name, email) VALUES ($1, $2)", "Alice", "alice@example.com")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}

		if tag.RowsAffected() != 1 {
			t.Errorf("Exec() affected %d rows, want 1", tag.RowsAffected())
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Successful update", func(t *testing.T) {
		mock.ExpectExec("UPDATE users SET").
			WithArgs("alice@newemail.com", 1).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		ctx := context.Background()
		tag, err := pool.Exec(ctx, "UPDATE users SET email = $1 WHERE id = $2", "alice@newemail.com", 1)
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}

		if tag.RowsAffected() != 1 {
			t.Errorf("Exec() affected %d rows, want 1", tag.RowsAffected())
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Exec error", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO users").
			WithArgs("Alice", "alice@example.com").
			WillReturnError(errors.New("constraint violation"))

		ctx := context.Background()
		_, err := pool.Exec(ctx, "INSERT INTO users (name, email) VALUES ($1, $2)", "Alice", "alice@example.com")
		if err == nil {
			t.Error("Exec() expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

// TestPoolBegin tests transaction Begin method
func TestPoolBegin(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	pool := &Pool{pool: mock}

	t.Run("Successful begin", func(t *testing.T) {
		mock.ExpectBegin()

		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		if tx == nil {
			t.Error("Begin() returned nil transaction")
		}

		// Clean up
		mock.ExpectRollback()
		_ = tx.Rollback(ctx)

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Begin error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(errors.New("too many connections"))

		ctx := context.Background()
		_, err := pool.Begin(ctx)
		if err == nil {
			t.Error("Begin() expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

// TestPoolWithTransaction tests WithTransaction helper
func TestPoolWithTransaction(t *testing.T) {
	t.Run("Successful transaction commits", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO users").
			WithArgs("Alice").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()

		ctx := context.Background()
		err = pool.WithTransaction(ctx, func(tx Transaction) error {
			_, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
			return err
		})

		if err != nil {
			t.Fatalf("WithTransaction() error = %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Failed transaction rolls back", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO users").
			WithArgs("Alice").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("INSERT INTO orders").
			WithArgs(1, 100.0).
			WillReturnError(errors.New("constraint violation"))
		mock.ExpectRollback()

		ctx := context.Background()
		err = pool.WithTransaction(ctx, func(tx Transaction) error {
			_, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
			if err != nil {
				return err
			}

			_, err = tx.Exec(ctx, "INSERT INTO orders (user_id, total) VALUES ($1, $2)", 1, 100.0)
			return err
		})

		if err == nil {
			t.Error("WithTransaction() expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Panic in transaction rolls back", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectBegin()
		mock.ExpectRollback()

		ctx := context.Background()

		defer func() {
			if r := recover(); r == nil {
				t.Error("WithTransaction() expected panic, got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %s", err)
			}
		}()

		_ = pool.WithTransaction(ctx, func(tx Transaction) error {
			panic("something went wrong")
		})
	})
}

// TestPoolPing tests Ping method
func TestPoolPing(t *testing.T) {
	t.Run("Successful ping", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectPing()

		ctx := context.Background()
		err = pool.Ping(ctx)
		if err != nil {
			t.Errorf("Ping() error = %v, want nil", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Failed ping", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectPing().WillReturnError(errors.New("connection refused"))

		ctx := context.Background()
		err = pool.Ping(ctx)
		if err == nil {
			t.Error("Ping() expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

// TestPoolClose tests Close method
func TestPoolClose(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}

	pool := &Pool{pool: mock}

	// Close should not panic
	pool.Close()
}

// TestPoolStats tests Stats method
func TestPoolStats(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	pool := &Pool{pool: mock}

	stats := pool.Stats()
	if stats == nil {
		t.Error("Stats() returned nil")
	}
}
