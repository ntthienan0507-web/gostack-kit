package user

import (
	"github.com/gin-gonic/gin"

	"github.com/ntthienan0507-web/gostack-kit/pkg/auth"
	"github.com/ntthienan0507-web/gostack-kit/pkg/middleware"
)

// Routes groups the user module route definitions.
type Routes struct {
	controller *Controller
}

// NewRoutes creates a new Routes instance.
func NewRoutes(c *Controller) *Routes {
	return &Routes{controller: c}
}

// Register registers all user module routes under rg.
func (r *Routes) Register(rg *gin.RouterGroup, authProvider auth.Provider) {
	users := rg.Group("/users")
	users.Use(middleware.Auth(authProvider))
	{
		users.GET("", r.controller.List)
		users.POST("", middleware.RequireRole("admin"), r.controller.Create)
		users.GET("/:id", r.controller.GetByID)
		users.PUT("/:id", r.controller.Update)
		users.DELETE("/:id", middleware.RequireRole("admin"), r.controller.Delete)
	}
}
