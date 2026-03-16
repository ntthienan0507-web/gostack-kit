package order

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
	"github.com/ntthienan0507-web/gostack-kit/pkg/response"
)

// Controller is thin: parse request, call service, write response.
// No business logic, no ORM dependency.
type Controller struct {
	service *Service
	logger  *zap.Logger
}

// NewController creates an order controller with injected service.
func NewController(service *Service, logger *zap.Logger) *Controller {
	return &Controller{service: service, logger: logger}
}

// List godoc
// @Summary      List orders
// @Description  Get paginated list of orders with optional search and status filter
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        page       query  int     false  "Page number"    default(1)
// @Param        page_size  query  int     false  "Page size"      default(20)
// @Param        q          query  string  false  "Search term"
// @Param        status     query  string  false  "Filter by status"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  apperror.AppError
// @Security     BearerAuth
// @Router       /orders [get]
func (c *Controller) List(ctx *gin.Context) {
	var pParams response.PaginationParams
	if err := ctx.ShouldBindQuery(&pParams); err != nil {
		response.Error(ctx, apperror.ErrInvalidParams)
		return
	}
	pParams, _, _ = response.NormalizePaginationParams(pParams)

	var query struct {
		Search string `form:"q"`
		Status string `form:"status"`
	}
	if err := ctx.ShouldBindQuery(&query); err != nil {
		response.Error(ctx, apperror.ErrValidationFailed)
		return
	}

	result, err := c.service.List(ctx.Request.Context(), ListParams{
		Search:   query.Search,
		Status:   query.Status,
		Page:     pParams.Page,
		PageSize: pParams.PageSize,
	})
	if err != nil {
		c.logger.Error("list orders", zap.Error(err))
		response.HandleError(ctx, err)
		return
	}

	response.Success(ctx, result)
}

// GetByID godoc
// @Summary      Get order by ID
// @Description  Get a single order by its UUID
// @Tags         orders
// @Produce      json
// @Param        id   path  string  true  "Order ID (UUID)"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  apperror.AppError
// @Failure      404  {object}  apperror.AppError
// @Security     BearerAuth
// @Router       /orders/{id} [get]
func (c *Controller) GetByID(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		response.Error(ctx, ErrInvalidOrderID)
		return
	}

	order, err := c.service.GetByID(ctx.Request.Context(), id)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}

	response.Success(ctx, order)
}

// Create godoc
// @Summary      Create order
// @Description  Create a new order with items (transaction + outbox + async notification)
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        body  body  CreateOrderRequest  true  "Order data"
// @Success      201   {object}  map[string]interface{}
// @Failure      400   {object}  apperror.AppError
// @Security     BearerAuth
// @Router       /orders [post]
func (c *Controller) Create(ctx *gin.Context) {
	var req CreateOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, apperror.New(400, "common.validation_failed", err.Error()))
		return
	}

	order, err := c.service.Create(ctx.Request.Context(), req)
	if err != nil {
		c.logger.Error("create order", zap.Error(err))
		response.HandleError(ctx, err)
		return
	}

	response.Created(ctx, order)
}

// Cancel godoc
// @Summary      Cancel order
// @Description  Cancel a pending or confirmed order (transaction + outbox)
// @Tags         orders
// @Param        id  path  string  true  "Order ID (UUID)"
// @Success      204
// @Failure      400  {object}  apperror.AppError
// @Failure      404  {object}  apperror.AppError
// @Failure      409  {object}  apperror.AppError
// @Security     BearerAuth
// @Router       /orders/{id}/cancel [post]
func (c *Controller) Cancel(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		response.Error(ctx, ErrInvalidOrderID)
		return
	}

	if err := c.service.Cancel(ctx.Request.Context(), id); err != nil {
		response.HandleError(ctx, err)
		return
	}

	response.NoContent(ctx)
}
