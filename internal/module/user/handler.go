package user

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/chungnguyen/go-api-template/internal/response"
)

// Handler is thin: parse request, call service, write response.
// No business logic here — fixes DataCentral fat controllers.
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler creates a user handler with injected service.
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List handles GET /users?page=1&page_size=20&q=search&role=admin
func (h *Handler) List(ctx *gin.Context) {
	var pParams response.PaginationParams
	if err := ctx.ShouldBindQuery(&pParams); err != nil {
		response.BadRequest(ctx, "Invalid pagination params")
		return
	}
	pParams, _, _ = response.NormalizePaginationParams(pParams)

	var query struct {
		Search string `form:"q"`
		Role   string `form:"role"`
	}
	if err := ctx.ShouldBindQuery(&query); err != nil {
		response.BadRequest(ctx, err.Error())
		return
	}

	result, err := h.service.List(ctx.Request.Context(), ListParams{
		Search:   query.Search,
		Role:     query.Role,
		Page:     pParams.Page,
		PageSize: pParams.PageSize,
	})
	if err != nil {
		h.logger.Error("list users", zap.Error(err))
		response.DBError(ctx, err)
		return
	}

	response.Success(ctx, result)
}

// GetByID handles GET /users/:id
func (h *Handler) GetByID(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		response.BadRequest(ctx, "Invalid user ID")
		return
	}

	user, err := h.service.GetByID(ctx.Request.Context(), id)
	if err != nil {
		response.DBError(ctx, err)
		return
	}

	response.Success(ctx, user)
}

// Create handles POST /users
func (h *Handler) Create(ctx *gin.Context) {
	var params CreateParams
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.BadRequest(ctx, err.Error())
		return
	}

	user, err := h.service.Create(ctx.Request.Context(), params)
	if err != nil {
		h.logger.Error("create user", zap.Error(err))
		response.DBError(ctx, err)
		return
	}

	response.Created(ctx, user)
}

// Update handles PUT /users/:id
func (h *Handler) Update(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		response.BadRequest(ctx, "Invalid user ID")
		return
	}

	var params UpdateParams
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.BadRequest(ctx, err.Error())
		return
	}

	user, err := h.service.Update(ctx.Request.Context(), id, params)
	if err != nil {
		response.DBError(ctx, err)
		return
	}

	response.Success(ctx, user)
}

// Delete handles DELETE /users/:id
func (h *Handler) Delete(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		response.BadRequest(ctx, "Invalid user ID")
		return
	}

	if err := h.service.Delete(ctx.Request.Context(), id); err != nil {
		response.DBError(ctx, err)
		return
	}

	response.NoContent(ctx)
}
