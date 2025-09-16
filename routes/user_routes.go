package routes

import (
	"abdo/controllers"
	"github.com/gin-gonic/gin"
)

func SetupUserRoutes(router *gin.RouterGroup) {
	userController := controllers.NewUserController()
	
	// User CRUD routes
	router.POST("/users", userController.CreateUser)           // Create user
	router.GET("/users", userController.GetAllUsers)           // Get all users
	router.GET("/users/:id", userController.GetUser)           // Get user by ID
	router.PUT("/users/:id", userController.UpdateUser)        // Update user
	router.DELETE("/users/:id", userController.DeleteUser)     // Delete user
}