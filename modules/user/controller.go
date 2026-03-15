package user

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/go-api-template/internal/response"
)

// Handler is thin: parse request, call service, write response.
// No business logic, no ORM dependency.
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler creates a user handler with injected service.
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List godoc
// @Summary      List users
// @Description  Get paginated list of users with optional search and role filter
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        page       query  int     false  "Page number"    default(1)
// @Param        page_size  query  int     false  "Page size"      default(20)
// @Param        q          query  string  false  "Search term"
// @Param        role       query  string  false  "Filter by role"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  response.ErrorBody
// @Security     BearerAuth
// @Router       /users [get]
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

// GetByID godoc
// @Summary      Get user by ID
// @Description  Get a single user by their UUID
// @Tags         users
// @Produce      json
// @Param        id   path  string  true  "User ID (UUID)"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  response.ErrorBody
// @Failure      404  {object}  response.ErrorBody
// @Security     BearerAuth
// @Router       /users/{id} [get]
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

// Create godoc
// @Summary      Create user
// @Description  Create a new user with hashed password
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body  CreateRequest  true  "User data"
// @Success      201   {object}  map[string]interface{}
// @Failure      400   {object}  response.ErrorBody
// @Failure      409   {object}  response.ErrorBody
// @Security     BearerAuth
// @Router       /users [post]
func (h *Handler) Create(ctx *gin.Context) {
	var req CreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequest(ctx, err.Error())
		return
	}

	user, err := h.service.Create(ctx.Request.Context(), req)
	if err != nil {
		h.logger.Error("create user", zap.Error(err))
		response.DBError(ctx, err)
		return
	}

	response.Created(ctx, user)
}

// Update godoc
// @Summary      Update user
// @Description  Update an existing user's details
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path  string         true  "User ID (UUID)"
// @Param        body  body  UpdateRequest  true  "Fields to update"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  response.ErrorBody
// @Failure      404   {object}  response.ErrorBody
// @Security     BearerAuth
// @Router       /users/{id} [put]
func (h *Handler) Update(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		response.BadRequest(ctx, "Invalid user ID")
		return
	}

	var req UpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequest(ctx, err.Error())
		return
	}

	user, err := h.service.Update(ctx.Request.Context(), id, req)
	if err != nil {
		response.DBError(ctx, err)
		return
	}

	response.Success(ctx, user)
}

// Delete godoc
// @Summary      Delete user
// @Description  Soft-delete a user by ID
// @Tags         users
// @Param        id  path  string  true  "User ID (UUID)"
// @Success      204
// @Failure      400  {object}  response.ErrorBody
// @Failure      404  {object}  response.ErrorBody
// @Security     BearerAuth
// @Router       /users/{id} [delete]
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
