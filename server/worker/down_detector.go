package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/service"
)

// DownThreshold: 이 시간 이상 메트릭이 없으면 down으로 판정
const DownThreshold = 30 * time.Second

type DownServerRepo interface {
	FindStale(ctx context.Context, threshold time.Duration) ([]model.Server, error)
	UpdateStatus(ctx context.Context, id string, status model.ServerStatus) error
}

type DownAlertRepo interface {
	Insert(ctx context.Context, a *model.Alert) error
	ResolveByServer(ctx context.Context, serverID string) error
	CanAlert(ctx context.Context, key string, cooldown time.Duration) (bool, error)
}

type DownDetector struct {
	serverRepo DownServerRepo
	alertRepo  DownAlertRepo
	notifier   service.Notifier
}

func NewDownDetector(sr DownServerRepo, ar DownAlertRepo, n service.Notifier) *DownDetector {
	return &DownDetector{serverRepo: sr, alertRepo: ar, notifier: n}
}

// RunOnce: 한 번 실행 (테스트 가능하도록 분리)
func (d *DownDetector) RunOnce(ctx context.Context) {
	stale, err := d.serverRepo.FindStale(ctx, DownThreshold)
	if err != nil {
		log.Printf("[DownDetector] FindStale 오류: %v", err)
		return
	}

	for _, s := range stale {
		_ = d.serverRepo.UpdateStatus(ctx, s.ID, model.StatusDown)
		_ = d.alertRepo.ResolveByServer(ctx, s.ID)

		key := s.ID + ":down:connection"
		ok, _ := d.alertRepo.CanAlert(ctx, key, cooldownDuration)
		if ok {
			msg := fmt.Sprintf("💀 [AlwaysOn] *%s* 서버가 응답하지 않습니다.\n서버: %s\n마지막 수신: %ds 이상 전",
				s.Name, s.Host, int(DownThreshold.Seconds()))
			_ = d.alertRepo.Insert(ctx, &model.Alert{
				ServerID: s.ID,
				Level:    model.LevelDown,
				Metric:   "connection",
				Value:    0,
				Message:  msg,
			})
			_ = d.notifier.Send(msg)
		}
	}
}

// Start: 15초마다 RunOnce 실행하는 고루틴 시작
func (d *DownDetector) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.RunOnce(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}
