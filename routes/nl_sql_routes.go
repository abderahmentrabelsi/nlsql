package routes

import (
	"abdo/controllers"

	"github.com/gin-gonic/gin"
)

func SetupNLSQLRoutes(router *gin.RouterGroup) {
	ctl := controllers.NewNLSQLController()

	nl := router.Group("/nl2sql")
	{
		nl.POST("", ctl.NL2SQL)            // POST /api/v1/nl2sql
		nl.POST("/repair", ctl.Repair)     // POST /api/v1/nl2sql/repair
		nl.POST("/feedback", ctl.Feedback) // POST /api/v1/nl2sql/feedback
		nl.POST("/execute", ctl.Execute)   // POST /api/v1/nl2sql/execute
	}
}
