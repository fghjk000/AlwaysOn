package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func AgentAuthMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}
		if c.GetHeader("X-Agent-Key") != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
