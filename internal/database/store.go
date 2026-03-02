package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/chungnguyen/go-api-template/db/sqlc"
)

// Store wraps db.Queries and adds transaction support.
// Pattern kept from DataCentral — proven effective.
type Store struct {
	*db.Queries
	pool *pgxpool.Pool
}

// NewStore creates a Store with embedded Queries.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		Queries: db.New(pool),
		pool:    pool,
	}
}

// ExecTx runs fn inside a database transaction. Rolls back on error.
func (s *Store) ExecTx(ctx context.Context, fn func(*db.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	q := db.New(tx)
	if err := fn(q); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx err: %w; rollback err: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit(ctx)
}
