package order

import (
	"time"

	"github.com/google/uuid"
)

// --- Query Params (domain-level, ORM-agnostic) ---

// ListParams are validated inputs for the List operation.
type ListParams struct {
	Search   string
	Status   string
	Page     int
	PageSize int
}

// --- HTTP Request/Response DTOs ---

// CreateOrderItemRequest is a single line item in a create-order request.
type CreateOrderItemRequest struct {
	ProductID uuid.UUID `json:"product_id" binding:"required"`
	Name      string    `json:"name"       binding:"required,min=1,max=255"`
	Quantity  int       `json:"quantity"    binding:"required,min=1"`
	UnitPrice float64   `json:"unit_price"  binding:"required,gt=0"`
}

// CreateOrderRequest is the JSON body for POST /orders.
type CreateOrderRequest struct {
	UserID   uuid.UUID                `json:"user_id"  binding:"required"`
	Currency string                   `json:"currency" binding:"required,len=3"`
	Note     string                   `json:"note"     binding:"max=500"`
	Items    []CreateOrderItemRequest `json:"items"    binding:"required,min=1,dive"`
}

// UpdateStatusRequest is the JSON body for PATCH /orders/:id/status.
type UpdateStatusRequest struct {
	Status  string `json:"status" binding:"required,oneof=confirmed shipped delivered cancelled"`
	Version *int   `json:"version"`
}

// OrderItemResponse is the client-facing representation of an order item.
type OrderItemResponse struct {
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"product_id"`
	Name      string    `json:"name"`
	Quantity  int       `json:"quantity"`
	UnitPrice float64   `json:"unit_price"`
}

// OrderResponse is the client-facing representation. Never exposes internal fields.
type OrderResponse struct {
	ID         uuid.UUID           `json:"id"`
	UserID     uuid.UUID           `json:"user_id"`
	Status     OrderStatus         `json:"status"`
	TotalPrice float64             `json:"total_price"`
	Currency   string              `json:"currency"`
	Note       string              `json:"note"`
	Version    int                 `json:"version"`
	Items      []OrderItemResponse `json:"items"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// ToResponse converts domain Order to client-facing OrderResponse.
func ToResponse(o *Order) OrderResponse {
	items := make([]OrderItemResponse, len(o.Items))
	for i, item := range o.Items {
		items[i] = OrderItemResponse{
			ID:        item.ID,
			ProductID: item.ProductID,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		}
	}

	return OrderResponse{
		ID:         o.ID,
		UserID:     o.UserID,
		Status:     o.Status,
		TotalPrice: o.TotalPrice,
		Currency:   o.Currency,
		Note:       o.Note,
		Version:    o.Version,
		Items:      items,
		CreatedAt:  o.CreatedAt,
		UpdatedAt:  o.UpdatedAt,
	}
}

// ListResult is returned by Service.List for the controller to format.
type ListResult struct {
	Items []OrderResponse
	Total int64
}
