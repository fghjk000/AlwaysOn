package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/worker"
	"github.com/stretchr/testify/mock"
)

type mockAlertRepo struct{ mock.Mock }

func (m *mockAlertRepo) Insert(ctx context.Context, a *model.Alert) error {
	return m.Called(ctx, a).Error(0)
}
func (m *mockAlertRepo) ResolveByServer(ctx context.Context, serverID string) error {
	return m.Called(ctx, serverID).Error(0)
}

type mockThresholdRepo struct{ mock.Mock }

func (m *mockThresholdRepo) Get(ctx context.Context, serverID string) (*model.Threshold, error) {
	args := m.Called(ctx, serverID)
	return args.Get(0).(*model.Threshold), args.Error(1)
}

type mockAlertServerRepo struct{ mock.Mock }

func (m *mockAlertServerRepo) UpdateStatus(ctx context.Context, id string, s model.ServerStatus) error {
	return m.Called(ctx, id, s).Error(0)
}

type mockNotifier struct{ mock.Mock }

func (m *mockNotifier) Send(msg string) error {
	return m.Called(msg).Error(0)
}

func TestAlertWorker_Check_TriggersWarning(t *testing.T) {
	alertRepo := &mockAlertRepo{}
	thresholdRepo := &mockThresholdRepo{}
	serverRepo := &mockAlertServerRepo{}
	notifier := &mockNotifier{}

	thresholdRepo.On("Get", mock.Anything, "server-1").Return(&model.Threshold{
		ServerID: "server-1", CPUWarning: 75, CPUCritical: 90,
		MemWarning: 80, MemCritical: 95, DiskWarning: 80, DiskCritical: 90,
	}, nil)
	alertRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)
	serverRepo.On("UpdateStatus", mock.Anything, "server-1", model.StatusWarning).Return(nil)
	notifier.On("Send", mock.Anything).Return(nil)

	w := worker.NewAlertWorker(serverRepo, alertRepo, thresholdRepo, notifier)
	server := &model.Server{ID: "server-1", Name: "Test", Status: model.StatusNormal}
	metric := &model.Metric{Time: time.Now(), ServerID: "server-1", CPU: 80, Memory: 50, Disk: 50}

	w.Check(context.Background(), server, metric)

	notifier.AssertCalled(t, "Send", mock.MatchedBy(func(msg string) bool { return len(msg) > 0 }))
	serverRepo.AssertCalled(t, "UpdateStatus", mock.Anything, "server-1", model.StatusWarning)
}

func TestAlertWorker_Check_RecoveryAlert(t *testing.T) {
	alertRepo := &mockAlertRepo{}
	thresholdRepo := &mockThresholdRepo{}
	serverRepo := &mockAlertServerRepo{}
	notifier := &mockNotifier{}

	thresholdRepo.On("Get", mock.Anything, "server-2").Return(&model.Threshold{
		ServerID: "server-2", CPUWarning: 75, CPUCritical: 90,
		MemWarning: 80, MemCritical: 95, DiskWarning: 80, DiskCritical: 90,
	}, nil)
	alertRepo.On("ResolveByServer", mock.Anything, "server-2").Return(nil)
	serverRepo.On("UpdateStatus", mock.Anything, "server-2", model.StatusNormal).Return(nil)
	notifier.On("Send", mock.Anything).Return(nil)

	w := worker.NewAlertWorker(serverRepo, alertRepo, thresholdRepo, notifier)
	// 이전 상태가 warning인 서버가 정상 메트릭을 보내면 복구 알림
	server := &model.Server{ID: "server-2", Name: "Test2", Status: model.StatusWarning}
	metric := &model.Metric{Time: time.Now(), ServerID: "server-2", CPU: 10, Memory: 20, Disk: 30}

	w.Check(context.Background(), server, metric)

	notifier.AssertCalled(t, "Send", mock.MatchedBy(func(msg string) bool {
		return len(msg) > 0
	}))
	alertRepo.AssertCalled(t, "ResolveByServer", mock.Anything, "server-2")
}

func TestAlertWorker_Check_NormalNoAlert(t *testing.T) {
	alertRepo := &mockAlertRepo{}
	thresholdRepo := &mockThresholdRepo{}
	serverRepo := &mockAlertServerRepo{}
	notifier := &mockNotifier{}

	thresholdRepo.On("Get", mock.Anything, "server-3").Return(&model.Threshold{
		ServerID: "server-3", CPUWarning: 75, CPUCritical: 90,
		MemWarning: 80, MemCritical: 95, DiskWarning: 80, DiskCritical: 90,
	}, nil)

	w := worker.NewAlertWorker(serverRepo, alertRepo, thresholdRepo, notifier)
	server := &model.Server{ID: "server-3", Name: "Test3", Status: model.StatusNormal}
	metric := &model.Metric{Time: time.Now(), ServerID: "server-3", CPU: 10, Memory: 20, Disk: 30}

	w.Check(context.Background(), server, metric)

	notifier.AssertNotCalled(t, "Send", mock.Anything)
	alertRepo.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
}
