package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/worker"
	"github.com/stretchr/testify/mock"
)

type mockDownServerRepo struct{ mock.Mock }

func (m *mockDownServerRepo) FindStale(ctx context.Context, threshold time.Duration) ([]model.Server, error) {
	args := m.Called(ctx, threshold)
	return args.Get(0).([]model.Server), args.Error(1)
}
func (m *mockDownServerRepo) UpdateStatus(ctx context.Context, id string, s model.ServerStatus) error {
	return m.Called(ctx, id, s).Error(0)
}

type mockDownAlertRepo struct{ mock.Mock }

func (m *mockDownAlertRepo) Insert(ctx context.Context, a *model.Alert) error {
	return m.Called(ctx, a).Error(0)
}
func (m *mockDownAlertRepo) ResolveByServer(ctx context.Context, serverID string) error {
	return m.Called(ctx, serverID).Error(0)
}
func (m *mockDownAlertRepo) CanAlert(ctx context.Context, key string, cooldown time.Duration) (bool, error) {
	args := m.Called(ctx, key, cooldown)
	return args.Bool(0), args.Error(1)
}

type mockDownNotifier struct{ mock.Mock }

func (m *mockDownNotifier) Send(msg string) error {
	return m.Called(msg).Error(0)
}

func TestDownDetector_MarksStaleServerAsDown(t *testing.T) {
	serverRepo := &mockDownServerRepo{}
	alertRepo := &mockDownAlertRepo{}
	notifier := &mockDownNotifier{}

	staleServer := model.Server{ID: "s1", Name: "Web01", Host: "web01", Status: model.StatusNormal}
	serverRepo.On("FindStale", mock.Anything, worker.DownThreshold).Return([]model.Server{staleServer}, nil)
	serverRepo.On("UpdateStatus", mock.Anything, "s1", model.StatusDown).Return(nil)
	alertRepo.On("ResolveByServer", mock.Anything, "s1").Return(nil)
	alertRepo.On("CanAlert", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	alertRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)
	notifier.On("Send", mock.Anything).Return(nil)

	d := worker.NewDownDetector(serverRepo, alertRepo, notifier)
	d.RunOnce(context.Background())

	serverRepo.AssertCalled(t, "UpdateStatus", mock.Anything, "s1", model.StatusDown)
	notifier.AssertCalled(t, "Send", mock.MatchedBy(func(msg string) bool { return len(msg) > 0 }))
}

func TestDownDetector_SkipsAlreadyDownServer(t *testing.T) {
	serverRepo := &mockDownServerRepo{}
	alertRepo := &mockDownAlertRepo{}
	notifier := &mockDownNotifier{}

	// 이미 down인 서버는 FindStale에서 제외됨 (status != 'down' 조건)
	serverRepo.On("FindStale", mock.Anything, worker.DownThreshold).Return([]model.Server{}, nil)

	d := worker.NewDownDetector(serverRepo, alertRepo, notifier)
	d.RunOnce(context.Background())

	serverRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
	notifier.AssertNotCalled(t, "Send", mock.Anything)
}
