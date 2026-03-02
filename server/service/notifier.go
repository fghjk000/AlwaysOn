package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier interface {
	Send(message string) error
}

type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (n *SlackNotifier) Send(message string) error {
	if n.webhookURL == "" {
		fmt.Printf("[Slack 비활성화] %s\n", message)
		return nil
	}

	payload := map[string]string{"text": message}
	data, _ := json.Marshal(payload)

	resp, err := n.client.Post(n.webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Slack 응답 오류: %d", resp.StatusCode)
	}
	return nil
}
