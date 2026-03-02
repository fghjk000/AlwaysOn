package api

import "github.com/gin-gonic/gin"

type Handlers struct {
	Metric  *MetricHandler
	ServerH *ServerHandler
	AlertH  *AlertHandler
}

func NewRouter(h *Handlers) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	api := r.Group("/api")
	{
		api.POST("/metrics", h.Metric.Receive)
		api.GET("/servers", h.ServerH.List)
		api.GET("/servers/:id", h.ServerH.Get)
		api.GET("/servers/:id/metrics", h.ServerH.GetMetrics)
		api.PUT("/servers/:id/thresholds", h.ServerH.UpdateThresholds)
		api.GET("/alerts", h.AlertH.List)
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
