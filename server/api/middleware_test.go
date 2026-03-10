package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/alwayson/server/api"
)

func TestAgentAuthMiddleware_NoKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.AgentAuthMiddleware("secret"))
	r.POST("/api/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/metrics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentAuthMiddleware_WrongKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.AgentAuthMiddleware("secret"))
	r.POST("/api/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/metrics", nil)
	req.Header.Set("X-Agent-Key", "wrong")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentAuthMiddleware_CorrectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.AgentAuthMiddleware("secret"))
	r.POST("/api/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/metrics", nil)
	req.Header.Set("X-Agent-Key", "secret")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentAuthMiddleware_EmptyKey_Disabled(t *testing.T) {
	// AGENT_API_KEY가 비어있으면 인증 비활성화
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.AgentAuthMiddleware(""))
	r.POST("/api/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/metrics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
