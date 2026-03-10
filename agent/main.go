package main

import (
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	ServerURL string   `yaml:"server_url"`
	Host      string   `yaml:"host"`
	Name      string   `yaml:"name"`
	Interval  int      `yaml:"interval_seconds"`
	Processes []string `yaml:"processes"`
	APIKey    string   `yaml:"api_key"`
}

func main() {
	var cfg AgentConfig

	// 환경변수 우선 (Docker 컨테이너용)
	cfg.ServerURL = os.Getenv("AGENT_SERVER_URL")
	cfg.Host = os.Getenv("AGENT_HOST")
	cfg.Name = os.Getenv("AGENT_NAME")
	cfg.APIKey = os.Getenv("AGENT_API_KEY")

	// 환경변수가 없으면 YAML 파일 읽기
	if cfg.ServerURL == "" || cfg.Host == "" {
		configPath := "agent.yaml"
		if len(os.Args) > 1 {
			configPath = os.Args[1]
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatalf("설정 파일 읽기 실패: %v", err)
		}

		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Fatalf("설정 파싱 실패: %v", err)
		}
	}

	if cfg.Interval <= 0 {
		cfg.Interval = 5
	}
	if cfg.Host == "" {
		log.Fatal("AGENT_HOST 환경변수 또는 agent.yaml에 host 설정이 필요합니다")
	}
	if cfg.ServerURL == "" {
		log.Fatal("AGENT_SERVER_URL 환경변수 또는 agent.yaml에 server_url 설정이 필요합니다")
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
			payload.Processes = CollectProcesses(cfg.Processes)
			if err := SendWithRetry(cfg.ServerURL, cfg.APIKey, payload); err != nil {
				log.Printf("[오류] 메트릭 전송 최종 실패: %v", err)
			} else {
				log.Printf("[전송] CPU=%.1f%% MEM=%.1f%% DISK=%.1f%%",
					payload.CPU, payload.Memory, payload.Disk)
			}
		}
		time.Sleep(time.Duration(cfg.Interval) * time.Second)
	}
}
