package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/service"
)

type AlertServerRepo interface {
	UpdateStatus(ctx context.Context, id string, status model.ServerStatus) error
}

type AlertAlertRepo interface {
	Insert(ctx context.Context, a *model.Alert) error
	ResolveByServer(ctx context.Context, serverID string) error
	CanAlert(ctx context.Context, key string, cooldown time.Duration) (bool, error)
	ResolveByServerAndMetric(ctx context.Context, serverID, metric string) (int64, error)
}

type AlertThresholdRepo interface {
	Get(ctx context.Context, serverID string) (*model.Threshold, error)
}

const cooldownDuration = 10 * time.Minute

type AlertWorker struct {
	serverRepo    AlertServerRepo
	alertRepo     AlertAlertRepo
	thresholdRepo AlertThresholdRepo
	notifier      service.Notifier
}

func NewAlertWorker(sr AlertServerRepo, ar AlertAlertRepo, tr AlertThresholdRepo, n service.Notifier) *AlertWorker {
	return &AlertWorker{
		serverRepo:    sr,
		alertRepo:     ar,
		thresholdRepo: tr,
		notifier:      n,
	}
}

func (w *AlertWorker) Check(ctx context.Context, server *model.Server, m *model.Metric) {
	th, err := w.thresholdRepo.Get(ctx, server.ID)
	if err != nil {
		return
	}

	type checkItem struct {
		metric string
		value  float64
		warn   float64
		crit   float64
	}

	checks := []checkItem{
		{"cpu", m.CPU, th.CPUWarning, th.CPUCritical},
		{"memory", m.Memory, th.MemWarning, th.MemCritical},
		{"disk", m.Disk, th.DiskWarning, th.DiskCritical},
	}

	highestStatus := model.StatusNormal
	for _, c := range checks {
		var level model.AlertLevel
		var status model.ServerStatus

		if c.value >= c.crit {
			level = model.LevelCritical
			status = model.StatusCritical
		} else if c.value >= c.warn {
			level = model.LevelWarning
			status = model.StatusWarning
		} else {
			continue
		}

		if statusPriority(status) > statusPriority(highestStatus) {
			highestStatus = status
		}

		threshold := c.warn
		if level == model.LevelCritical {
			threshold = c.crit
		}
		msg := fmt.Sprintf("🚨 [AlwaysOn] *%s* - %s %s\n서버: %s\n현재값: %.1f%%\n임계값: %.1f%%",
			string(level), server.Name, c.metric, server.Host, c.value, threshold,
		)

		key := server.ID + ":" + string(level) + ":" + c.metric
		ok, err := w.alertRepo.CanAlert(ctx, key, cooldownDuration)
		if err != nil || !ok {
			continue
		}

		_ = w.alertRepo.Insert(ctx, &model.Alert{
			ServerID: server.ID,
			Level:    level,
			Metric:   c.metric,
			Value:    c.value,
			Message:  msg,
		})
		_ = w.notifier.Send(msg)
	}

	// 프로세스 감시
	for _, ps := range m.Processes {
		metricKey := "process:" + ps.Name
		if !ps.Running {
			key := server.ID + ":critical:" + metricKey
			ok, err := w.alertRepo.CanAlert(ctx, key, cooldownDuration)
			if err != nil || !ok {
				continue
			}
			msg := fmt.Sprintf("🚨 [AlwaysOn] 프로세스 미실행\n서버: %s\n프로세스: %s", server.Name, ps.Name)
			_ = w.alertRepo.Insert(ctx, &model.Alert{
				ServerID: server.ID,
				Level:    model.LevelCritical,
				Metric:   metricKey,
				Value:    0,
				Message:  msg,
			})
			_ = w.notifier.Send(msg)
		} else {
			// 실행 중: 기존 알림 resolve
			affected, err := w.alertRepo.ResolveByServerAndMetric(ctx, server.ID, metricKey)
			if err == nil && affected > 0 {
				msg := fmt.Sprintf("✅ [AlwaysOn] 프로세스 복구\n서버: %s\n프로세스: %s", server.Name, ps.Name)
				_ = w.notifier.Send(msg)
			}
		}
	}

	if highestStatus == model.StatusNormal && server.Status != model.StatusNormal {
		_ = w.alertRepo.ResolveByServer(ctx, server.ID)
		msg := fmt.Sprintf("✅ [AlwaysOn] *%s* 서버가 정상 상태로 복구되었습니다.", server.Name)
		_ = w.notifier.Send(msg)
	}

	if highestStatus != server.Status {
		_ = w.serverRepo.UpdateStatus(ctx, server.ID, highestStatus)
	}
}

func statusPriority(s model.ServerStatus) int {
	switch s {
	case model.StatusCritical:
		return 3
	case model.StatusWarning:
		return 2
	case model.StatusNormal:
		return 1
	}
	return 0
}
