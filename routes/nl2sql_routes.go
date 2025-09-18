package routes

import (
	"abdo/controllers"

	"github.com/gin-gonic/gin"
)

// SetupNL2SQLRoutes registers the NL→SQL endpoint under /api/v1
func SetupNL2SQLRoutes(router *gin.RouterGroup) {
	nl := controllers.NewNL2SQLController()
	router.POST("/nl2sql", nl.GenerateSQL)
}
