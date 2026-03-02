package service

import (
	"context"
	"time"

	"github.com/alwayson/server/model"
)

type ServerRepository interface {
	Upsert(ctx context.Context, host, name string) (*model.Server, error)
	UpdateLastSeen(ctx context.Context, id string, t time.Time) error
}

type MetricRepository interface {
	Insert(ctx context.Context, m *model.Metric) error
}

type AlertProcessor interface {
	Check(ctx context.Context, server *model.Server, m *model.Metric)
}

type MetricInput struct {
	Host   string
	Name   string
	CPU    float64
	Memory float64
	Disk   float64
	NetIn  int64
	NetOut int64
}

type MetricService struct {
	serverRepo     ServerRepository
	metricRepo     MetricRepository
	alertProcessor AlertProcessor
}

func NewMetricService(sr ServerRepository, mr MetricRepository, ap AlertProcessor) *MetricService {
	return &MetricService{serverRepo: sr, metricRepo: mr, alertProcessor: ap}
}

func (s *MetricService) Process(ctx context.Context, input *MetricInput) (*model.Metric, error) {
	server, err := s.serverRepo.Upsert(ctx, input.Host, input.Name)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	_ = s.serverRepo.UpdateLastSeen(ctx, server.ID, now)

	metric := &model.Metric{
		Time:     now,
		ServerID: server.ID,
		CPU:      input.CPU,
		Memory:   input.Memory,
		Disk:     input.Disk,
		NetIn:    input.NetIn,
		NetOut:   input.NetOut,
	}

	if err := s.metricRepo.Insert(ctx, metric); err != nil {
		return nil, err
	}

	if s.alertProcessor != nil {
		go s.alertProcessor.Check(context.Background(), server, metric)
	}

	return metric, nil
}
