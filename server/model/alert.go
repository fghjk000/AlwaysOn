package model

import "time"

type AlertLevel string

const (
	LevelWarning  AlertLevel = "warning"
	LevelCritical AlertLevel = "critical"
	LevelDown     AlertLevel = "down"
)

type Alert struct {
	ID         string     `json:"id"`
	ServerID   string     `json:"server_id"`
	Level      AlertLevel `json:"level"`
	Metric     string     `json:"metric"`
	Value      float64    `json:"value"`
	Message    string     `json:"message"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}
