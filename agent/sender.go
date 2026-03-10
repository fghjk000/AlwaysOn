package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func SendMetrics(serverURL, apiKey string, payload *MetricPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("POST", serverURL+"/api/metrics", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-Agent-Key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("서버 응답 오류: %d", resp.StatusCode)
	}
	return nil
}

func SendWithRetry(serverURL, apiKey string, payload *MetricPayload) error {
	backoff := time.Second
	const maxAttempts = 5

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := SendMetrics(serverURL, apiKey, payload)
		if err == nil {
			return nil
		}
		if attempt == maxAttempts {
			return fmt.Errorf("메트릭 전송 %d회 실패: %w", maxAttempts, err)
		}
		log.Printf("[재시도 %d/%d] %v — %v 후 재시도", attempt, maxAttempts, err, backoff)
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
	return nil
}
