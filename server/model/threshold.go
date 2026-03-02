package model

type Threshold struct {
	ServerID     string  `json:"server_id"`
	CPUWarning   float64 `json:"cpu_warning"`
	CPUCritical  float64 `json:"cpu_critical"`
	MemWarning   float64 `json:"mem_warning"`
	MemCritical  float64 `json:"mem_critical"`
	DiskWarning  float64 `json:"disk_warning"`
	DiskCritical float64 `json:"disk_critical"`
}

func DefaultThreshold(serverID string) Threshold {
	return Threshold{
		ServerID:     serverID,
		CPUWarning:   75,
		CPUCritical:  90,
		MemWarning:   80,
		MemCritical:  95,
		DiskWarning:  80,
		DiskCritical: 90,
	}
}
