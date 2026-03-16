package response

import (
	"github.com/gin-gonic/gin"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

// errorBody returns the error payload, stripping Detail in release mode.
func errorBody(err *apperror.AppError) *apperror.AppError {
	if gin.Mode() == gin.ReleaseMode {
		return err.Sanitize()
	}
	return err
}

// Abort writes the AppError as JSON and aborts the gin chain.
func Abort(ctx *gin.Context, err *apperror.AppError) {
	ctx.AbortWithStatusJSON(err.Code, errorBody(err))
}

// Error writes the AppError as JSON without aborting.
func Error(ctx *gin.Context, err *apperror.AppError) {
	ctx.JSON(err.Code, errorBody(err))
}

// HandleError converts err via apperror.FromError and writes the response.
func HandleError(ctx *gin.Context, err error) {
	Error(ctx, apperror.FromError(err))
}
