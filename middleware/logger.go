package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
)

// Custom response writer to capture response body
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// PrettyLogger creates a custom logging middleware with beautiful formatting
func PrettyLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		var statusColor, methodColor, resetColor string
		if param.IsOutputColor() {
			statusColor = param.StatusCodeColor()
			methodColor = param.MethodColor()
			resetColor = param.ResetColor()
		}

		// Color status based on code
		statusIcon := "✅"
		if param.StatusCode >= 400 {
			statusIcon = "❌"
		} else if param.StatusCode >= 300 {
			statusIcon = "⚠️"
		}

		return fmt.Sprintf("%s[API]%s %s %s%3d%s %s| %13v | %15s |%s %-7s %s%s\n%s",
			"\033[36m", resetColor, // Cyan for [API]
			statusIcon,
			statusColor, param.StatusCode, resetColor,
			"📊",
			param.Latency,
			param.ClientIP,
			methodColor, param.Method, resetColor,
			param.Path,
			resetColor,
		)
	})
}

// JSONResponseLogger logs pretty-printed JSON responses in development
func JSONResponseLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for health checks and static files
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/favicon.ico" {
			c.Next()
			return
		}

		// Create a custom writer to capture response
		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w

		// Process request
		c.Next()

		// Only log JSON responses
		contentType := c.Writer.Header().Get("Content-Type")
		if contentType == "application/json; charset=utf-8" && w.body.Len() > 0 {
			// Try to pretty print JSON
			var jsonResponse interface{}
			if err := json.Unmarshal(w.body.Bytes(), &jsonResponse); err == nil {
				if prettyJSON, err := json.MarshalIndent(jsonResponse, "", "  "); err == nil {
					fmt.Printf("\n🔥 \033[32mJSON Response:\033[0m\n\033[33m%s\033[0m\n\n", string(prettyJSON))
				}
			}
		}
	}
}

// RequestLogger logs incoming request details
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for health checks
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		// Log request body for POST/PUT/PATCH
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			if c.Request.Body != nil {
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err == nil && len(bodyBytes) > 0 {
					// Restore the body for the actual handler
					c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

					// Pretty print request JSON
					var jsonRequest interface{}
					if err := json.Unmarshal(bodyBytes, &jsonRequest); err == nil {
						if prettyJSON, err := json.MarshalIndent(jsonRequest, "", "  "); err == nil {
							fmt.Printf("\n📥 \033[34mRequest Body:\033[0m\n\033[36m%s\033[0m\n", string(prettyJSON))
						}
					}
				}
			}
		}

		c.Next()
	}
}
