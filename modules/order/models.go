package order

import (
	"time"

	"github.com/google/uuid"
)

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusConfirmed OrderStatus = "confirmed"
	StatusShipped   OrderStatus = "shipped"
	StatusDelivered OrderStatus = "delivered"
	StatusCancelled OrderStatus = "cancelled"
)

// Order is the domain model used across service and repository layers.
// Neither GORM nor any other ORM types leak outside the repository.
type Order struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Status     OrderStatus
	TotalPrice float64
	Currency   string
	Note       string
	Version    int
	Items      []OrderItem
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

// OrderItem is a line item within an order.
type OrderItem struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	ProductID uuid.UUID
	Name      string
	Quantity  int
	UnitPrice float64
	CreatedAt time.Time
}
