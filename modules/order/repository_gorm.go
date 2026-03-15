package order

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ntthienan0507-web/go-api-template/pkg/apperror"
)

// --- GORM models (internal to the repository layer) ---

// gormOrder is the GORM model for the orders table.
// Maps to the same PostgreSQL table as the goose migration — schema managed externally.
type gormOrder struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	Status     string         `gorm:"type:varchar(50);not null;default:'pending'"`
	TotalPrice float64        `gorm:"type:decimal(12,2);not null;default:0"`
	Currency   string         `gorm:"type:varchar(3);not null;default:'USD'"`
	Note       string         `gorm:"type:text"`
	Version    int            `gorm:"not null;default:1"`
	CreatedAt  time.Time      `gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	Items      []gormOrderItem `gorm:"foreignKey:OrderID"`
}

// TableName tells GORM the table name (matches goose migration).
func (gormOrder) TableName() string { return "orders" }

// gormOrderItem is the GORM model for the order_items table.
type gormOrderItem struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrderID   uuid.UUID `gorm:"type:uuid;not null;index"`
	ProductID uuid.UUID `gorm:"type:uuid;not null"`
	Name      string    `gorm:"type:varchar(255);not null"`
	Quantity  int       `gorm:"not null;default:1"`
	UnitPrice float64   `gorm:"type:decimal(12,2);not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName tells GORM the table name (matches goose migration).
func (gormOrderItem) TableName() string { return "order_items" }

// --- Repository implementation ---

// gormRepository implements Repository using GORM.
type gormRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a GORM-backed order repository.
func NewGORMRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

// conn returns either the provided transaction or the default db connection.
// This pattern lets repository methods participate in external transactions
// (e.g., when creating an order and writing an outbox event atomically).
func (r *gormRepository) conn(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}

func (r *gormRepository) List(ctx context.Context, params ListParams, limit, offset int32) ([]*Order, error) {
	var rows []gormOrder
	q := r.db.WithContext(ctx).Where("deleted_at IS NULL")

	if params.Search != "" {
		pattern := "%" + params.Search + "%"
		q = q.Where("note ILIKE ?", pattern)
	}
	if params.Status != "" {
		q = q.Where("status = ?", params.Status)
	}

	err := q.Preload("Items").Order("created_at DESC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error
	if err != nil {
		return nil, err
	}

	return mapSlice(rows, fromGormOrder), nil
}

func (r *gormRepository) Count(ctx context.Context, params ListParams) (int64, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&gormOrder{}).Where("deleted_at IS NULL")

	if params.Search != "" {
		pattern := "%" + params.Search + "%"
		q = q.Where("note ILIKE ?", pattern)
	}
	if params.Status != "" {
		q = q.Where("status = ?", params.Status)
	}

	err := q.Count(&count).Error
	return count, err
}

func (r *gormRepository) GetByID(ctx context.Context, id uuid.UUID) (*Order, error) {
	var row gormOrder
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Preload("Items").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return fromGormOrderVal(row), nil
}

func (r *gormRepository) Create(ctx context.Context, tx *gorm.DB, order *Order) (*Order, error) {
	row := toGormOrder(order)

	// Use the provided transaction so the insert is atomic with outbox writes.
	if err := r.conn(tx).WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}

	return fromGormOrderVal(row), nil
}

// UpdateStatus changes the status of an order with optimistic locking.
//
// Optimistic locking: WHERE version = ? ensures no lost updates.
// If another request modified this record first, version won't match -> 409 Conflict.
// Client must retry with the latest version.
func (r *gormRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, status OrderStatus, version int) error {
	result := r.conn(tx).WithContext(ctx).
		Model(&gormOrder{}).
		Where("id = ? AND version = ? AND deleted_at IS NULL", id, version).
		Updates(map[string]interface{}{
			"status":     string(status),
			"updated_at": time.Now(),
			"version":    gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperror.ErrStaleVersion
	}
	return nil
}

func (r *gormRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&gormOrder{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

// --- Mapping: gormOrder ↔ domain Order ---

func toGormOrder(o *Order) gormOrder {
	items := make([]gormOrderItem, len(o.Items))
	for i, item := range o.Items {
		items[i] = gormOrderItem{
			ID:        item.ID,
			OrderID:   o.ID,
			ProductID: item.ProductID,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		}
	}

	return gormOrder{
		ID:         o.ID,
		UserID:     o.UserID,
		Status:     string(o.Status),
		TotalPrice: o.TotalPrice,
		Currency:   o.Currency,
		Note:       o.Note,
		Version:    o.Version,
		Items:      items,
	}
}

func fromGormOrderVal(row gormOrder) *Order {
	items := make([]OrderItem, len(row.Items))
	for i, item := range row.Items {
		items[i] = OrderItem{
			ID:        item.ID,
			OrderID:   item.OrderID,
			ProductID: item.ProductID,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			CreatedAt: item.CreatedAt,
		}
	}

	o := &Order{
		ID:         row.ID,
		UserID:     row.UserID,
		Status:     OrderStatus(row.Status),
		TotalPrice: row.TotalPrice,
		Currency:   row.Currency,
		Note:       row.Note,
		Version:    row.Version,
		Items:      items,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
	if row.DeletedAt.Valid {
		o.DeletedAt = &row.DeletedAt.Time
	}
	return o
}

// fromGormOrder adapts gormOrder (value) for mapSlice.
func fromGormOrder(row gormOrder) *Order {
	return fromGormOrderVal(row)
}

// mapSlice applies fn to each element, returning a new slice.
func mapSlice[T any, U any](items []T, fn func(T) U) []U {
	result := make([]U, len(items))
	for i, item := range items {
		result[i] = fn(item)
	}
	return result
}
