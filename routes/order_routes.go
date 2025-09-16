package routes

import (
	"abdo/controllers"
	"github.com/gin-gonic/gin"
)

func SetupOrderRoutes(router *gin.RouterGroup) {
	orderController := controllers.NewOrderController()
	
	// Order CRUD routes
	router.POST("/orders", orderController.CreateOrder)              // Create order
	router.GET("/orders", orderController.GetAllOrders)              // Get all orders (supports ?status= and ?user_id= filters)
	router.GET("/orders/:id", orderController.GetOrder)              // Get order by ID
	router.PUT("/orders/:id", orderController.UpdateOrder)           // Update order
	router.PATCH("/orders/:id/status", orderController.UpdateOrderStatus) // Update order status only
	router.DELETE("/orders/:id", orderController.DeleteOrder)        // Delete order
}