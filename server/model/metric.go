package model

import "time"

type ProcessStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
}

type Metric struct {
	Time      time.Time       `json:"time"`
	ServerID  string          `json:"server_id"`
	CPU       float64         `json:"cpu"`
	Memory    float64         `json:"memory"`
	Disk      float64         `json:"disk"`
	NetIn     int64           `json:"net_in"`
	NetOut    int64           `json:"net_out"`
	Processes []ProcessStatus `json:"processes,omitempty"`
}
