package worker

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/service"
)

type HCConfigRepo interface {
	ListEnabled(ctx context.Context) ([]model.HealthCheckConfig, error)
}

type HCAlertRepo interface {
	Insert(ctx context.Context, a *model.Alert) error
	CanAlert(ctx context.Context, key string, cooldown time.Duration) (bool, error)
	ResolveByServerAndMetric(ctx context.Context, serverID, metric string) (int64, error)
}

type HealthCheckWorker struct {
	configRepo HCConfigRepo
	alertRepo  HCAlertRepo
	notifier   service.Notifier
	httpClient *http.Client
}

func NewHealthCheckWorker(cr HCConfigRepo, ar HCAlertRepo, n service.Notifier) *HealthCheckWorker {
	return &HealthCheckWorker{
		configRepo: cr,
		alertRepo:  ar,
		notifier:   n,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (w *HealthCheckWorker) RunOnce(ctx context.Context) {
	configs, err := w.configRepo.ListEnabled(ctx)
	if err != nil {
		log.Printf("[HealthCheckWorker] 설정 로드 오류: %v", err)
		return
	}
	for _, cfg := range configs {
		go w.check(ctx, cfg)
	}
}

func (w *HealthCheckWorker) check(ctx context.Context, cfg model.HealthCheckConfig) {
	metricKey := "health_check:" + cfg.Name
	var checkErr error

	switch cfg.Type {
	case "http":
		checkErr = w.checkHTTP(cfg)
	case "tcp":
		checkErr = w.checkTCP(cfg.Target)
	}

	if checkErr != nil {
		key := cfg.ServerID + ":critical:" + metricKey
		ok, err := w.alertRepo.CanAlert(ctx, key, cooldownDuration)
		if err != nil || !ok {
			return
		}
		msg := fmt.Sprintf("🚨 [AlwaysOn] 헬스체크 실패\n대상: %s (%s)\n오류: %v", cfg.Name, cfg.Target, checkErr)
		_ = w.alertRepo.Insert(ctx, &model.Alert{
			ServerID: cfg.ServerID,
			Level:    model.LevelCritical,
			Metric:   metricKey,
			Value:    0,
			Message:  msg,
		})
		_ = w.notifier.Send(msg)
		return
	}

	// 성공 시: 기존 미해결 알림 resolve
	affected, err := w.alertRepo.ResolveByServerAndMetric(ctx, cfg.ServerID, metricKey)
	if err == nil && affected > 0 {
		msg := fmt.Sprintf("✅ [AlwaysOn] 헬스체크 복구\n대상: %s (%s)", cfg.Name, cfg.Target)
		_ = w.notifier.Send(msg)
	}
}

func (w *HealthCheckWorker) checkHTTP(cfg model.HealthCheckConfig) error {
	resp, err := w.httpClient.Get(cfg.Target)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != cfg.ExpectedStatus {
		return fmt.Errorf("HTTP %d (기대: %d)", resp.StatusCode, cfg.ExpectedStatus)
	}
	return nil
}

func (w *HealthCheckWorker) checkTCP(target string) error {
	conn, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func (w *HealthCheckWorker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.RunOnce(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}
