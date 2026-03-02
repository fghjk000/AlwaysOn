package model

import "time"

type ServerStatus string

const (
	StatusNormal   ServerStatus = "normal"
	StatusWarning  ServerStatus = "warning"
	StatusCritical ServerStatus = "critical"
	StatusDown     ServerStatus = "down"
)

type Server struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Host     string       `json:"host"`
	Status   ServerStatus `json:"status"`
	LastSeen *time.Time   `json:"last_seen"`
}
