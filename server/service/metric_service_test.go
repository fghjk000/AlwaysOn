package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockServerRepo struct{ mock.Mock }

func (m *mockServerRepo) Upsert(ctx context.Context, host, name string) (*model.Server, error) {
	args := m.Called(ctx, host, name)
	return args.Get(0).(*model.Server), args.Error(1)
}
func (m *mockServerRepo) UpdateLastSeen(ctx context.Context, id string, t time.Time) error {
	return m.Called(ctx, id, t).Error(0)
}

type mockMetricRepo struct{ mock.Mock }

func (m *mockMetricRepo) Insert(ctx context.Context, metric *model.Metric) error {
	return m.Called(ctx, metric).Error(0)
}

func TestMetricService_Process_InsertsMetric(t *testing.T) {
	serverRepo := &mockServerRepo{}
	metricRepo := &mockMetricRepo{}

	fakeServer := &model.Server{ID: "uuid-1", Host: "test-host", Name: "Test"}
	serverRepo.On("Upsert", mock.Anything, "test-host", "Test").Return(fakeServer, nil)
	serverRepo.On("UpdateLastSeen", mock.Anything, "uuid-1", mock.Anything).Return(nil)
	metricRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)

	svc := service.NewMetricService(serverRepo, metricRepo, nil)
	metric, err := svc.Process(context.Background(), &service.MetricInput{
		Host: "test-host", Name: "Test",
		CPU: 50, Memory: 60, Disk: 70,
	})

	assert.NoError(t, err)
	assert.Equal(t, "uuid-1", metric.ServerID)
	assert.Equal(t, 50.0, metric.CPU)
	metricRepo.AssertExpectations(t)
	serverRepo.AssertExpectations(t)
}
