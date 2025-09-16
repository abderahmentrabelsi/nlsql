package handlers

import (
	"net/http"
	"abdo/config"
	"abdo/models"
	
	"github.com/gin-gonic/gin"
)

// CreateUser handles POST /users
func CreateUser(c *gin.Context) {
	var user models.User
	
	// Bind JSON to user struct
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid input data",
			"details": err.Error(),
		})
		return
	}
	
	// Create user in database
	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create user",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user": user,
	})
}

// CreateOrder handles POST /orders
func CreateOrder(c *gin.Context) {
	var order models.Order
	
	// Bind JSON to order struct
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid input data",
			"details": err.Error(),
		})
		return
	}
	
	// Check if user exists
	var user models.User
	if err := config.DB.First(&user, order.UserID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User not found",
			"details": "User with provided ID does not exist",
		})
		return
	}
	
	// Create order in database
	if err := config.DB.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create order",
			"details": err.Error(),
		})
		return
	}
	
	// Load user data for response
	config.DB.Preload("User").First(&order, order.ID)
	
	c.JSON(http.StatusCreated, gin.H{
		"message": "Order created successfully",
		"order": order,
	})
}