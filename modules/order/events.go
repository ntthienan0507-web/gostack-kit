package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/ntthienan0507-web/gostack-kit/pkg/broker"
)

var (
	TopicOrderCreated   broker.Topic = "order.order.created"
	TopicOrderCancelled broker.Topic = "order.order.cancelled"
)

func init() {
	broker.MustRegisterTopic(TopicOrderCreated, "OrderCreatedEvent", broker.KeyByID)
	broker.MustRegisterTopic(TopicOrderCancelled, "OrderCancelledEvent", broker.KeyByID)
}

type OrderCreatedEvent struct {
	OrderID    uuid.UUID `json:"order_id"`
	UserID     uuid.UUID `json:"user_id"`
	TotalPrice float64   `json:"total_price"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	ItemCount  int       `json:"item_count"`
	CreatedAt  time.Time `json:"created_at"`
}

type OrderCancelledEvent struct {
	OrderID     uuid.UUID `json:"order_id"`
	UserID      uuid.UUID `json:"user_id"`
	CancelledAt time.Time `json:"cancelled_at"`
}
