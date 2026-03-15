package database

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	dialector := postgres.New(postgres.Config{
		Conn:       db,
		DriverName: "postgres",
	})

	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm: %v", err)
	}

	return gormDB, mock
}

func TestWithTransaction_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	logger := zap.NewNop()

	mock.ExpectBegin()
	mock.ExpectCommit()

	err := WithTransaction(context.Background(), db, logger, func(tx *gorm.DB) error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_Rollback_OnError(t *testing.T) {
	db, mock := setupMockDB(t)
	logger := zap.NewNop()

	expectedErr := errors.New("operation failed")

	mock.ExpectBegin()
	mock.ExpectRollback()

	err := WithTransaction(context.Background(), db, logger, func(tx *gorm.DB) error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_Rollback_OnPanic(t *testing.T) {
	db, mock := setupMockDB(t)
	logger := zap.NewNop()

	mock.ExpectBegin()
	mock.ExpectRollback()

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic, got none")
		}
		if r != "test panic" {
			t.Errorf("expected panic value 'test panic', got %v", r)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	}()

	_ = WithTransaction(context.Background(), db, logger, func(tx *gorm.DB) error {
		panic("test panic")
	})
}

func TestWithTransactionResult_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	logger := zap.NewNop()

	mock.ExpectBegin()
	mock.ExpectCommit()

	expected := 42
	result, err := WithTransactionResult(context.Background(), db, logger, func(tx *gorm.DB) (int, error) {
		return expected, nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if result != expected {
		t.Errorf("expected result %d, got %d", expected, result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransactionResult_Rollback_OnError(t *testing.T) {
	db, mock := setupMockDB(t)
	logger := zap.NewNop()

	expectedErr := errors.New("operation failed")

	mock.ExpectBegin()
	mock.ExpectRollback()

	result, err := WithTransactionResult(context.Background(), db, logger, func(tx *gorm.DB) (string, error) {
		return "", expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if result != "" {
		t.Errorf("expected empty result on error, got %v", result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransactionResult_Rollback_OnPanic(t *testing.T) {
	db, mock := setupMockDB(t)
	logger := zap.NewNop()

	mock.ExpectBegin()
	mock.ExpectRollback()

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic, got none")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	}()

	_, _ = WithTransactionResult(context.Background(), db, logger, func(tx *gorm.DB) (int, error) {
		panic("test panic")
	})
}
