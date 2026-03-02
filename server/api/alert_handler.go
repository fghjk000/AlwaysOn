package api

import (
	"net/http"
	"strconv"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/repository"
	"github.com/gin-gonic/gin"
)

type AlertHandler struct {
	alertRepo *repository.AlertRepo
}

func NewAlertHandler(ar *repository.AlertRepo) *AlertHandler {
	return &AlertHandler{alertRepo: ar}
}

func (h *AlertHandler) List(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	alerts, err := h.alertRepo.GetAll(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if alerts == nil {
		alerts = []model.Alert{}
	}
	c.JSON(http.StatusOK, alerts)
}
