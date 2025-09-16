package controllers

import (
	"net/http"
	"strconv"
	"abdo/models"
	"abdo/services"
	
	"github.com/gin-gonic/gin"
)

type OrderController struct {
	orderService *services.OrderService
}

func NewOrderController() *OrderController {
	return &OrderController{
		orderService: services.NewOrderService(),
	}
}

// CreateOrder handles POST /orders
func (oc *OrderController) CreateOrder(c *gin.Context) {
	var order models.Order
	
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid input data",
			"details": err.Error(),
		})
		return
	}
	
	if err := oc.orderService.CreateOrder(&order); err != nil {
		if err.Error() == "user not found" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create order",
				"details": err.Error(),
			})
		}
		return
	}
	
	// Get the created order with user details
	createdOrder, _ := oc.orderService.GetOrderByID(order.ID)
	
	c.JSON(http.StatusCreated, gin.H{
		"message": "Order created successfully",
		"order": createdOrder,
	})
}

// GetOrder handles GET /orders/:id
func (oc *OrderController) GetOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid order ID",
		})
		return
	}
	
	order, err := oc.orderService.GetOrderByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"order": order,
	})
}

// GetAllOrders handles GET /orders
func (oc *OrderController) GetAllOrders(c *gin.Context) {
	// Check for query parameters
	status := c.Query("status")
	userIDStr := c.Query("user_id")
	
	var orders []models.Order
	var err error
	
	if status != "" {
		orders, err = oc.orderService.GetOrdersByStatus(status)
	} else if userIDStr != "" {
		userID, parseErr := strconv.ParseUint(userIDStr, 10, 32)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid user ID",
			})
			return
		}
		orders, err = oc.orderService.GetOrdersByUserID(uint(userID))
	} else {
		orders, err = oc.orderService.GetAllOrders()
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve orders",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"count": len(orders),
	})
}

// UpdateOrder handles PUT /orders/:id
func (oc *OrderController) UpdateOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid order ID",
		})
		return
	}
	
	var updatedOrder models.Order
	if err := c.ShouldBindJSON(&updatedOrder); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid input data",
			"details": err.Error(),
		})
		return
	}
	
	order, err := oc.orderService.UpdateOrder(uint(id), &updatedOrder)
	if err != nil {
		if err.Error() == "order not found" || err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update order",
				"details": err.Error(),
			})
		}
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Order updated successfully",
		"order": order,
	})
}

// UpdateOrderStatus handles PATCH /orders/:id/status
func (oc *OrderController) UpdateOrderStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid order ID",
		})
		return
	}
	
	var statusUpdate struct {
		Status string `json:"status" binding:"required,oneof=pending processing shipped delivered cancelled"`
	}
	
	if err := c.ShouldBindJSON(&statusUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid status data",
			"details": err.Error(),
		})
		return
	}
	
	order, err := oc.orderService.UpdateOrderStatus(uint(id), statusUpdate.Status)
	if err != nil {
		if err.Error() == "order not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update order status",
				"details": err.Error(),
			})
		}
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Order status updated successfully",
		"order": order,
	})
}

// DeleteOrder handles DELETE /orders/:id
func (oc *OrderController) DeleteOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid order ID",
		})
		return
	}
	
	if err := oc.orderService.DeleteOrder(uint(id)); err != nil {
		if err.Error() == "order not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to delete order",
				"details": err.Error(),
			})
		}
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Order deleted successfully",
	})
}