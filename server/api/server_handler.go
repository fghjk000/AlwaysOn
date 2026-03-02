package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/repository"
	"github.com/gin-gonic/gin"
)

type ServerHandler struct {
	serverRepo    *repository.ServerRepo
	metricRepo    *repository.MetricRepo
	thresholdRepo *repository.ThresholdRepo
}

func NewServerHandler(sr *repository.ServerRepo, mr *repository.MetricRepo, tr *repository.ThresholdRepo) *ServerHandler {
	return &ServerHandler{serverRepo: sr, metricRepo: mr, thresholdRepo: tr}
}

func (h *ServerHandler) List(c *gin.Context) {
	servers, err := h.serverRepo.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if servers == nil {
		servers = []model.Server{}
	}
	c.JSON(http.StatusOK, servers)
}

func (h *ServerHandler) Get(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	latest, _ := h.metricRepo.GetLatest(ctx, id)
	threshold, _ := h.thresholdRepo.Get(ctx, id)

	c.JSON(http.StatusOK, gin.H{
		"id":        id,
		"latest":    latest,
		"threshold": threshold,
	})
}

func (h *ServerHandler) GetMetrics(c *gin.Context) {
	id := c.Param("id")
	hours := 1
	if hStr := c.Query("hours"); hStr != "" {
		if v, err := strconv.Atoi(hStr); err == nil && v > 0 {
			hours = v
		}
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	metrics, err := h.metricRepo.GetRecent(c.Request.Context(), id, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if metrics == nil {
		metrics = []model.Metric{}
	}
	c.JSON(http.StatusOK, metrics)
}

func (h *ServerHandler) UpdateThresholds(c *gin.Context) {
	id := c.Param("id")
	var t model.Threshold
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t.ServerID = id
	if err := h.thresholdRepo.Upsert(context.Background(), &t); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}
