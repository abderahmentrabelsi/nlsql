package controllers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"abdo/services"

	"github.com/gin-gonic/gin"
)

type NL2SQLController struct {
	svc *services.NL2SQLService
}

func NewNL2SQLController() *NL2SQLController {
	return &NL2SQLController{
		svc: services.NewNL2SQLService(),
	}
}

type NL2SQLRequest struct {
	Question        string          `json:"question" binding:"required"`
	DSN             *string         `json:"dsn,omitempty"`
	SchemaJSON      json.RawMessage `json:"schema_json,omitempty"`
	SchemaPath      *string         `json:"schema_path,omitempty"`
	TablesWhitelist []string        `json:"tables_whitelist,omitempty"`
	Hints           []string        `json:"hints,omitempty"`
	MaxTables       *int            `json:"max_tables,omitempty"`
	MaxColsPerTable *int            `json:"max_cols_per_table,omitempty"`
}

type NL2SQLResponse struct {
	SQL     string `json:"sql"`
	Model   string `json:"model"`
	UsedDSN string `json:"used_dsn"`
	Safe    bool   `json:"safe"`
	Note    string `json:"note,omitempty"`
}

func (ctl *NL2SQLController) GenerateSQL(c *gin.Context) {
	var req NL2SQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	maxTables := 40
	if req.MaxTables != nil && *req.MaxTables > 0 {
		maxTables = *req.MaxTables
	}
	maxCols := 128
	if req.MaxColsPerTable != nil && *req.MaxColsPerTable > 0 {
		maxCols = *req.MaxColsPerTable
	}

	// Resolve schema source: prefer inline schema_json; else schema_path; else DSN/env
	schemaJSONBytes := []byte(req.SchemaJSON)
	if len(schemaJSONBytes) == 0 && req.SchemaPath != nil && strings.TrimSpace(*req.SchemaPath) != "" {
		p := strings.TrimSpace(*req.SchemaPath)
		if strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid schema_path",
				"details": "only relative paths within the project are allowed",
			})
			return
		}
		b, err := os.ReadFile(p)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "failed to read schema_path",
				"details": err.Error(),
			})
			return
		}
		schemaJSONBytes = b
	}

	sqlText, usedDSN, note, err := ctl.svc.GenerateSQL(
		c.Request.Context(),
		req.Question,
		req.DSN,
		schemaJSONBytes,
		req.TablesWhitelist,
		req.Hints,
		maxTables,
		maxCols,
	)
	if err != nil {
		// Classify client vs server errors
		msg := err.Error()
		status := http.StatusInternalServerError
		if strings.Contains(msg, "question is required") ||
			strings.Contains(msg, "only SELECT statements") ||
			strings.Contains(msg, "unsafe token detected") ||
			strings.Contains(msg, "empty model output") ||
			strings.Contains(msg, "must include a database") ||
			strings.Contains(msg, "invalid schema_json") ||
			strings.Contains(msg, "unrecognized schema_json format") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{
			"error":   "failed to generate SQL",
			"details": msg,
		})
		return
	}

	c.JSON(http.StatusOK, NL2SQLResponse{
		SQL:     sqlText,
		Model:   ctl.svc.Model(),
		UsedDSN: usedDSN,
		Safe:    true,
		Note:    note,
	})
}
