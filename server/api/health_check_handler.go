package api

import (
	"context"
	"net/http"

	"github.com/alwayson/server/model"
	"github.com/gin-gonic/gin"
)

type HealthCheckRepository interface {
	Insert(ctx context.Context, cfg *model.HealthCheckConfig) error
	ListByServer(ctx context.Context, serverID string) ([]model.HealthCheckConfig, error)
	Delete(ctx context.Context, id string) error
}

type HealthCheckHandler struct {
	repo HealthCheckRepository
}

func NewHealthCheckHandler(repo HealthCheckRepository) *HealthCheckHandler {
	return &HealthCheckHandler{repo: repo}
}

type healthCheckRequest struct {
	Name           string `json:"name" binding:"required"`
	Type           string `json:"type" binding:"required,oneof=http tcp"`
	Target         string `json:"target" binding:"required"`
	ExpectedStatus int    `json:"expected_status"`
	IntervalSec    int    `json:"interval_sec"`
}

func (h *HealthCheckHandler) Create(c *gin.Context) {
	serverID := c.Param("id")
	var req healthCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ExpectedStatus == 0 {
		req.ExpectedStatus = 200
	}
	if req.IntervalSec <= 0 {
		req.IntervalSec = 30
	}
	cfg := &model.HealthCheckConfig{
		ServerID:       serverID,
		Name:           req.Name,
		Type:           req.Type,
		Target:         req.Target,
		ExpectedStatus: req.ExpectedStatus,
		IntervalSec:    req.IntervalSec,
		Enabled:        true,
	}
	if err := h.repo.Insert(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cfg)
}

func (h *HealthCheckHandler) List(c *gin.Context) {
	serverID := c.Param("id")
	list, err := h.repo.ListByServer(c.Request.Context(), serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []model.HealthCheckConfig{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *HealthCheckHandler) Delete(c *gin.Context) {
	id := c.Param("hid")
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
