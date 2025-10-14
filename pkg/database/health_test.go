package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
)

// TestCheckHealth tests the CheckHealth function
func TestCheckHealth(t *testing.T) {
	t.Run("Healthy database", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		rows := pgxmock.NewRows([]string{"result"}).AddRow(1)
		mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

		ctx := context.Background()
		err = CheckHealth(ctx, pool)
		if err != nil {
			t.Errorf("CheckHealth() error = %v, want nil", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Unhealthy database", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectQuery("SELECT 1").WillReturnError(errors.New("connection refused"))

		ctx := context.Background()
		err = CheckHealth(ctx, pool)
		if err == nil {
			t.Error("CheckHealth() expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Health check timeout", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectQuery("SELECT 1").WillDelayFor(10 * time.Second)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = CheckHealth(ctx, pool)
		if err == nil {
			t.Error("CheckHealth() expected timeout error, got nil")
		}
		// Don't check expectations here because the query didn't complete due to timeout
	})

	t.Run("Unexpected result", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		rows := pgxmock.NewRows([]string{"result"}).AddRow(42)
		mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

		ctx := context.Background()
		err = CheckHealth(ctx, pool)
		if err == nil {
			t.Error("CheckHealth() expected error for unexpected result, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Context with existing deadline", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		rows := pgxmock.NewRows([]string{"result"}).AddRow(1)
		mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

		// Create context with a deadline
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = CheckHealth(ctx, pool)
		if err != nil {
			t.Errorf("CheckHealth() error = %v, want nil", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

// TestPingWithTimeout tests the PingWithTimeout method
func TestPingWithTimeout(t *testing.T) {
	t.Run("Successful ping with timeout", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectPing()

		err = pool.PingWithTimeout(context.Background(), 5*time.Second)
		if err != nil {
			t.Errorf("PingWithTimeout() error = %v, want nil", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Ping timeout", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectPing().WillDelayFor(10 * time.Second)

		err = pool.PingWithTimeout(context.Background(), 100*time.Millisecond)
		if err == nil {
			t.Error("PingWithTimeout() expected timeout error, got nil")
		}
		// Don't check expectations because the ping didn't complete
	})

	t.Run("Ping fails", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectPing().WillReturnError(errors.New("connection refused"))

		err = pool.PingWithTimeout(context.Background(), 5*time.Second)
		if err == nil {
			t.Error("PingWithTimeout() expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

// Note: HealthCheck() method is not fully testable with pgxmock because
// the Stats() method requires a real pgxpool.Pool. It would need integration
// tests with a real database connection. The core CheckHealth() function
// is tested above, which covers the main functionality.
