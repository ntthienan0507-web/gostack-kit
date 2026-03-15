package user

import (
	"github.com/gin-gonic/gin"

	"github.com/ntthienan0507-web/go-api-template/internal/auth"
	"github.com/ntthienan0507-web/go-api-template/internal/middleware"
)

// RegisterRoutes registers all user module routes under rg.
// Single consistent pattern across all modules.
func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authProvider auth.Provider) {
	users := rg.Group("/users")
	users.Use(middleware.Auth(authProvider))
	{
		users.GET("", handler.List)
		users.POST("", middleware.RequireRole("admin"), handler.Create)
		users.GET("/:id", handler.GetByID)
		users.PUT("/:id", handler.Update)
		users.DELETE("/:id", middleware.RequireRole("admin"), handler.Delete)
	}
}
