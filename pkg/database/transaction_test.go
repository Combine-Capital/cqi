package database

import (
	"context"
	"errors"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

// TestTransactionCommitRollback tests basic transaction operations
func TestTransactionCommitRollback(t *testing.T) {
	t.Run("Commit successful transaction", func(t *testing.T) {
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
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		_, err = tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Rollback transaction on error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO users").
			WithArgs("Alice").
			WillReturnError(errors.New("constraint violation"))
		mock.ExpectRollback()

		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		_, err = tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
		if err == nil {
			t.Error("Exec() expected error, got nil")
		}

		err = tx.Rollback(ctx)
		if err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

// TestTransactionQuery tests Query within transaction
func TestTransactionQuery(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	pool := &Pool{pool: mock}

	mock.ExpectBegin()
	rows := pgxmock.NewRows([]string{"id", "name"}).
		AddRow(1, "Alice")
	mock.ExpectQuery("SELECT (.+) FROM users").
		WillReturnRows(rows)
	mock.ExpectRollback()

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	result, err := tx.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	defer result.Close()

	count := 0
	for result.Next() {
		count++
	}

	if count != 1 {
		t.Errorf("Query() returned %d rows, want 1", count)
	}

	_ = tx.Rollback(ctx)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

// TestTransactionQueryRow tests QueryRow within transaction
func TestTransactionQueryRow(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	pool := &Pool{pool: mock}

	mock.ExpectBegin()
	rows := pgxmock.NewRows([]string{"id", "name"}).
		AddRow(1, "Alice")
	mock.ExpectQuery("SELECT (.+) FROM users WHERE id").
		WithArgs(1).
		WillReturnRows(rows)
	mock.ExpectRollback()

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	var id int
	var name string
	err = tx.QueryRow(ctx, "SELECT id, name FROM users WHERE id = $1", 1).Scan(&id, &name)
	if err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}

	if id != 1 || name != "Alice" {
		t.Errorf("QueryRow() got id=%d, name=%s, want id=1, name=Alice", id, name)
	}

	_ = tx.Rollback(ctx)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

// TestBeginTransaction tests the BeginTransaction helper
func TestBeginTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	pool := &Pool{pool: mock}

	mock.ExpectBegin()
	mock.ExpectRollback()

	ctx := context.Background()
	tx, err := BeginTransaction(ctx, pool)
	if err != nil {
		t.Fatalf("BeginTransaction() error = %v", err)
	}

	if tx == nil {
		t.Error("BeginTransaction() returned nil transaction")
	}

	_ = tx.Rollback(ctx)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

// TestWithTransactionHelper tests the WithTransaction helper function
func TestWithTransactionHelper(t *testing.T) {
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
	err = WithTransaction(ctx, pool, func(tx Transaction) error {
		_, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
		return err
	})

	if err != nil {
		t.Fatalf("WithTransaction() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

// TestMustCommit tests the MustCommit panic helper
func TestMustCommit(t *testing.T) {
	t.Run("Successful commit does not panic", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectBegin()
		mock.ExpectCommit()

		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		// Should not panic
		MustCommit(ctx, tx)

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Failed commit panics", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectBegin()
		mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		defer func() {
			if r := recover(); r == nil {
				t.Error("MustCommit() expected panic, got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %s", err)
			}
		}()

		MustCommit(ctx, tx)
	})
}

// TestMustRollback tests the MustRollback panic helper
func TestMustRollback(t *testing.T) {
	t.Run("Successful rollback does not panic", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectBegin()
		mock.ExpectRollback()

		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		// Should not panic
		MustRollback(ctx, tx)

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("Failed rollback panics", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		pool := &Pool{pool: mock}

		mock.ExpectBegin()
		mock.ExpectRollback().WillReturnError(errors.New("rollback failed"))

		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		defer func() {
			if r := recover(); r == nil {
				t.Error("MustRollback() expected panic, got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %s", err)
			}
		}()

		MustRollback(ctx, tx)
	})
}
