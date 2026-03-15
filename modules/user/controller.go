package user

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/go-api-template/pkg/apperror"
	"github.com/ntthienan0507-web/go-api-template/pkg/response"
)

type Handler struct {
	service *Service
	logger  *zap.Logger
}

func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) List(ctx *gin.Context) {
	var pParams response.PaginationParams
	if err := ctx.ShouldBindQuery(&pParams); err != nil {
		apperror.Respond(ctx, apperror.ErrInvalidParams)
		return
	}
	pParams, _, _ = response.NormalizePaginationParams(pParams)

	var query struct {
		Search string `form:"q"`
		Role   string `form:"role"`
	}
	if err := ctx.ShouldBindQuery(&query); err != nil {
		apperror.Respond(ctx, apperror.ErrValidationFailed)
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
		apperror.HandleError(ctx, err)
		return
	}

	response.OKList(ctx, result.Items, result.Total)
}

func (h *Handler) GetByID(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		apperror.Respond(ctx, ErrInvalidUserID)
		return
	}

	user, err := h.service.GetByID(ctx.Request.Context(), id)
	if err != nil {
		apperror.HandleError(ctx, err)
		return
	}

	response.OK(ctx, user)
}

func (h *Handler) Create(ctx *gin.Context) {
	var req CreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apperror.Respond(ctx, apperror.New(400, "common.validation_failed", err.Error()))
		return
	}

	user, err := h.service.Create(ctx.Request.Context(), req)
	if err != nil {
		h.logger.Error("create user", zap.Error(err))
		apperror.HandleError(ctx, err)
		return
	}

	response.Created(ctx, user)
}

func (h *Handler) Update(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		apperror.Respond(ctx, ErrInvalidUserID)
		return
	}

	var req UpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apperror.Respond(ctx, apperror.New(400, "common.validation_failed", err.Error()))
		return
	}

	user, err := h.service.Update(ctx.Request.Context(), id, req)
	if err != nil {
		apperror.HandleError(ctx, err)
		return
	}

	response.OK(ctx, user)
}

func (h *Handler) Delete(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		apperror.Respond(ctx, ErrInvalidUserID)
		return
	}

	if err := h.service.Delete(ctx.Request.Context(), id); err != nil {
		apperror.HandleError(ctx, err)
		return
	}

	response.NoContent(ctx)
}
