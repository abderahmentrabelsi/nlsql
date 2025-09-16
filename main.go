package main

import (
	"abdo/config"
	"abdo/middleware"
	"abdo/routes"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// Enable colored output for Gin
	gin.ForceConsoleColor()

	// Connect to database
	config.ConnectDatabase()

	// Create Gin router
	router := gin.New()

	// Add default Gin logging and our JSON response logger
	router.Use(gin.Logger())
	router.Use(middleware.JSONResponseLogger())

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Setup all routes
	routes.SetupRoutes(router)

	// Start server with a nice banner
	log.Println("🚀 Starting Abdo API Server...")
	log.Println("🌐 Server running at: http://localhost:8080")
	log.Println("📚 API Documentation: http://localhost:8080/api/v1")
	log.Println("💻 Ready for development!")

	if err := router.Run(":8080"); err != nil {
		log.Fatal("❌ Failed to start server:", err)
	}
}
