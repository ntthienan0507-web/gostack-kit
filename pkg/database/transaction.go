package database

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// WithTransaction wraps a function in a GORM database transaction.
// Rolls back on error or panic, commits on success.
func WithTransaction(ctx context.Context, db *gorm.DB, logger *zap.Logger, fn func(tx *gorm.DB) error) error {
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			if rbErr := tx.Rollback().Error; rbErr != nil {
				logger.Error("transaction rollback failed after panic", zap.Error(rbErr))
			}
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback().Error; rbErr != nil {
			logger.Error("transaction rollback failed", zap.Error(rbErr))
		}
		return err
	}

	return tx.Commit().Error
}

// WithTransactionResult wraps a function in a GORM transaction and returns a result.
// Useful when the transaction function needs to return data.
func WithTransactionResult[T any](ctx context.Context, db *gorm.DB, logger *zap.Logger, fn func(tx *gorm.DB) (T, error)) (T, error) {
	var result T

	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return result, tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			if rbErr := tx.Rollback().Error; rbErr != nil {
				logger.Error("transaction rollback failed after panic", zap.Error(rbErr))
			}
			panic(r)
		}
	}()

	var err error
	result, err = fn(tx)
	if err != nil {
		if rbErr := tx.Rollback().Error; rbErr != nil {
			logger.Error("transaction rollback failed", zap.Error(rbErr))
		}
		return result, err
	}

	if err := tx.Commit().Error; err != nil {
		return result, err
	}

	return result, nil
}
