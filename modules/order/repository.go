package order

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines data access for the order module.
// Uses domain types only — no GORM types leak through the interface.
//
// Methods that participate in transactions accept an optional *gorm.DB (tx).
// Pass nil to use the repository's default connection.
type Repository interface {
	// List returns a paginated list of orders matching the given params.
	List(ctx context.Context, params ListParams, limit, offset int32) ([]*Order, error)

	// Count returns the total number of orders matching the given params.
	Count(ctx context.Context, params ListParams) (int64, error)

	// GetByID returns a single order with its items.
	GetByID(ctx context.Context, id uuid.UUID) (*Order, error)

	// Create inserts an order and its items.
	// Accepts a tx parameter so it can participate in an external transaction
	// (e.g., when writing to the outbox table atomically).
	Create(ctx context.Context, tx *gorm.DB, order *Order) (*Order, error)

	// UpdateStatus changes the status of an order with optimistic locking.
	// Accepts a tx parameter for transactional consistency with outbox writes.
	UpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, status OrderStatus, version int) error

	// SoftDelete marks an order as deleted.
	SoftDelete(ctx context.Context, id uuid.UUID) error
}
