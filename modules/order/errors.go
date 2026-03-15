package order

import (
	"net/http"

	"github.com/ntthienan0507-web/go-api-template/pkg/apperror"
)

// Order module error codes.
// Namespace: "order.*"
var (
	ErrOrderNotFound  = apperror.New(http.StatusNotFound, "order.not_found", "Order with the given ID does not exist")
	ErrInvalidOrderID = apperror.New(http.StatusBadRequest, "order.invalid_id", "Invalid order ID format")
	ErrEmptyCart      = apperror.New(http.StatusBadRequest, "order.empty_cart", "Order must have at least one item")
	ErrNotCancellable = apperror.New(http.StatusConflict, "order.not_cancellable", "Only pending or confirmed orders can be cancelled")
	ErrInvalidStatus  = apperror.New(http.StatusBadRequest, "order.invalid_status", "Invalid order status transition")
)
