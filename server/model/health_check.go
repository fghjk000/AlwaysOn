package model

import "time"

type HealthCheckConfig struct {
	ID             string    `json:"id"`
	ServerID       string    `json:"server_id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`    // "http" | "tcp"
	Target         string    `json:"target"`  // URL 또는 host:port
	ExpectedStatus int       `json:"expected_status"`
	IntervalSec    int       `json:"interval_sec"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
}
