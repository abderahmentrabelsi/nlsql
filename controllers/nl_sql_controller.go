package controllers

import (
	"abdo/config"
	"abdo/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type NLSQLController struct {
	client *services.NLSQLClient
}

func NewNLSQLController() *NLSQLController {
	return &NLSQLController{
		client: services.NewNLSQLClient(),
	}
}

// POST /nl2sql
func (ctl *NLSQLController) NL2SQL(c *gin.Context) {
	var req services.NLQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}
	resp, err := ctl.client.NL2SQL(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "nl2sql upstream error", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// POST /nl2sql/repair
func (ctl *NLSQLController) Repair(c *gin.Context) {
	var req services.RepairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}
	resp, err := ctl.client.Repair(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "repair upstream error", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// POST /nl2sql/feedback
func (ctl *NLSQLController) Feedback(c *gin.Context) {
	var req services.FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}
	resp, err := ctl.client.Feedback(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "feedback upstream error", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

type executeRequest struct {
	SQL string `json:"sql" binding:"required"`
}

// POST /nl2sql/execute
func (ctl *NLSQLController) Execute(c *gin.Context) {
	var req executeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	if !services.IsSelectOnly(req.SQL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only single-statement SELECT queries are allowed"})
		return
	}

	result, err := services.ExecuteSelect(config.DB, req.SQL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "sql execution failed",
			"details": err.Error(),
			"sql":     req.SQL,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
