package api

import (
	"net/http"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/service"
	"github.com/gin-gonic/gin"
)

type MetricHandler struct {
	svc *service.MetricService
}

func NewMetricHandler(svc *service.MetricService) *MetricHandler {
	return &MetricHandler{svc: svc}
}

type metricRequest struct {
	Host      string                `json:"host" binding:"required"`
	Name      string                `json:"name"`
	CPU       float64               `json:"cpu"`
	Memory    float64               `json:"memory"`
	Disk      float64               `json:"disk"`
	NetIn     int64                 `json:"net_in"`
	NetOut    int64                 `json:"net_out"`
	Processes []model.ProcessStatus `json:"processes,omitempty"`
}

func (h *MetricHandler) Receive(c *gin.Context) {
	var req metricRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" {
		req.Name = req.Host
	}

	_, err := h.svc.Process(c.Request.Context(), &service.MetricInput{
		Host: req.Host, Name: req.Name,
		CPU: req.CPU, Memory: req.Memory, Disk: req.Disk,
		NetIn: req.NetIn, NetOut: req.NetOut,
		Processes: req.Processes,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
