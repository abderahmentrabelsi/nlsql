package routes

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine) {
	// API v1 group
	api := router.Group("/api/v1")
	{
		// Setup user routes
		SetupUserRoutes(api)

		// Setup order routes
		SetupOrderRoutes(api)

		// Setup NL→SQL routes
		SetupNL2SQLRoutes(api)
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "OK",
			"message": "API is running",
		})
	})
}
