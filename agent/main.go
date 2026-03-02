package main

import (
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	ServerURL string `yaml:"server_url"`
	Host      string `yaml:"host"`
	Name      string `yaml:"name"`
	Interval  int    `yaml:"interval_seconds"`
}

func main() {
	configPath := "agent.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("설정 파일 읽기 실패: %v", err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("설정 파싱 실패: %v", err)
	}

	if cfg.Interval <= 0 {
		cfg.Interval = 5
	}
	if cfg.Host == "" {
		log.Fatal("agent.yaml에 host 설정이 필요합니다")
	}
	if cfg.ServerURL == "" {
		log.Fatal("agent.yaml에 server_url 설정이 필요합니다")
	}

	log.Printf("AlwaysOn Agent 시작 [host=%s, server=%s, interval=%ds]",
		cfg.Host, cfg.ServerURL, cfg.Interval)

	for {
		payload, err := CollectMetrics()
		if err != nil {
			log.Printf("[오류] 메트릭 수집 실패: %v", err)
		} else {
			payload.Host = cfg.Host
			payload.Name = cfg.Name
			if err := SendMetrics(cfg.ServerURL, payload); err != nil {
				log.Printf("[오류] 메트릭 전송 실패: %v", err)
			} else {
				log.Printf("[전송] CPU=%.1f%% MEM=%.1f%% DISK=%.1f%%",
					payload.CPU, payload.Memory, payload.Disk)
			}
		}
		time.Sleep(time.Duration(cfg.Interval) * time.Second)
	}
}
