package main

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type ProcessStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
}

type MetricPayload struct {
	Host      string          `json:"host"`
	Name      string          `json:"name"`
	CPU       float64         `json:"cpu"`
	Memory    float64         `json:"memory"`
	Disk      float64         `json:"disk"`
	NetIn     int64           `json:"net_in"`
	NetOut    int64           `json:"net_out"`
	Processes []ProcessStatus `json:"processes,omitempty"`
}

func CollectMetrics() (*MetricPayload, error) {
	cpuPercents, err := cpu.Percent(0, false)
	if err != nil {
		return nil, err
	}

	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	diskStat, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}

	netStats, err := net.IOCounters(false)
	var netIn, netOut int64
	if err == nil && len(netStats) > 0 {
		netIn = int64(netStats[0].BytesRecv)
		netOut = int64(netStats[0].BytesSent)
	}

	return &MetricPayload{
		CPU:    cpuPercents[0],
		Memory: vmStat.UsedPercent,
		Disk:   diskStat.UsedPercent,
		NetIn:  netIn,
		NetOut: netOut,
	}, nil
}

// CollectProcesses: names 목록의 프로세스 실행 여부를 반환한다.
func CollectProcesses(names []string) []ProcessStatus {
	if len(names) == 0 {
		return nil
	}

	procs, _ := process.Processes()
	running := make(map[string]bool, len(procs))
	for _, p := range procs {
		if n, err := p.Name(); err == nil {
			running[n] = true
		}
	}

	result := make([]ProcessStatus, len(names))
	for i, name := range names {
		result[i] = ProcessStatus{Name: name, Running: running[name]}
	}
	return result
}
