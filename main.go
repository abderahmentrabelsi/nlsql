package main

import (
	"abdo/config"
	"abdo/middleware"
	"abdo/routes"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found, using system environment variables")
	}

	// Enable colored output for Gin
	gin.ForceConsoleColor()

	// Set Gin mode from environment
	if ginMode := os.Getenv("GIN_MODE"); ginMode != "" {
		gin.SetMode(ginMode)
	}

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

	// Get server port from environment
	port := getEnv("SERVER_PORT", "8080")
	serverAddr := ":" + port

	// Start server with a nice banner
	log.Println("🚀 Starting Abdo API Server...")
	log.Printf("🌐 Server running at: http://localhost:%s", port)
	log.Printf("📚 API Documentation: http://localhost:%s/api/v1", port)
	log.Println("💻 Ready for development!")

	if err := router.Run(serverAddr); err != nil {
		log.Fatal("❌ Failed to start server:", err)
	}
}

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
