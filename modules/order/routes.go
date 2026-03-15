package order

import (
	"github.com/gin-gonic/gin"

	"github.com/ntthienan0507-web/go-api-template/pkg/auth"
	"github.com/ntthienan0507-web/go-api-template/pkg/middleware"
)

// Routes wraps the controller for route registration.
type Routes struct {
	controller *Controller
}

// NewRoutes creates a Routes instance.
func NewRoutes(c *Controller) *Routes {
	return &Routes{controller: c}
}

// Register registers all order module routes under rg.
func (r *Routes) Register(rg *gin.RouterGroup, authProvider auth.Provider) {
	orders := rg.Group("/orders")
	orders.Use(middleware.Auth(authProvider))
	{
		orders.GET("", r.controller.List)
		orders.POST("", r.controller.Create)
		orders.GET("/:id", r.controller.GetByID)
		orders.POST("/:id/cancel", r.controller.Cancel)
	}
}
