package main

import (
	"log"
	"abdo/config"
	"abdo/routes"
	
	"github.com/gin-gonic/gin"
)

func main() {
	// Enable colored output for Gin
	gin.ForceConsoleColor()
	
	// Connect to database
	config.ConnectDatabase()
	
	// Create Gin router with custom logger
	router := gin.New()
	
	// Add colored logging middleware
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	}))
	
	// Add recovery middleware
	router.Use(gin.Recovery())
	
	// Setup all routes
	routes.SetupRoutes(router)
	
	// Start server
	log.Println("Starting server on :8080...")
	if err := router.Run(":8080"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}