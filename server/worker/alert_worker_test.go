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
func (m *mockAlertRepo) CanAlert(ctx context.Context, key string, cooldown time.Duration) (bool, error) {
	args := m.Called(ctx, key, cooldown)
	return args.Bool(0), args.Error(1)
}
func (m *mockAlertRepo) ResolveByServerAndMetric(ctx context.Context, serverID, metric string) (int64, error) {
	args := m.Called(ctx, serverID, metric)
	return args.Get(0).(int64), args.Error(1)
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
	alertRepo.On("CanAlert", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
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

func TestAlertWorker_ProcessDown_Alert(t *testing.T) {
	alertRepo := &mockAlertRepo{}
	thresholdRepo := &mockThresholdRepo{}
	serverRepo := &mockAlertServerRepo{}
	notifier := &mockNotifier{}

	thresholdRepo.On("Get", mock.Anything, "server-p").Return(&model.Threshold{
		ServerID: "server-p", CPUWarning: 75, CPUCritical: 90,
		MemWarning: 80, MemCritical: 95, DiskWarning: 80, DiskCritical: 90,
	}, nil)
	alertRepo.On("CanAlert", mock.Anything, mock.MatchedBy(func(key string) bool {
		return key == "server-p:critical:process:mysql"
	}), mock.Anything).Return(true, nil)
	alertRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)
	// nginx는 running=true이므로 ResolveByServerAndMetric 호출됨
	alertRepo.On("ResolveByServerAndMetric", mock.Anything, "server-p", "process:nginx").
		Return(int64(0), nil)
	notifier.On("Send", mock.Anything).Return(nil)

	w := worker.NewAlertWorker(serverRepo, alertRepo, thresholdRepo, notifier)
	server := &model.Server{ID: "server-p", Name: "TestProc", Status: model.StatusNormal}
	metric := &model.Metric{
		Time: time.Now(), ServerID: "server-p",
		CPU: 10, Memory: 20, Disk: 30,
		Processes: []model.ProcessStatus{
			{Name: "nginx", Running: true},
			{Name: "mysql", Running: false},
		},
	}

	w.Check(context.Background(), server, metric)

	notifier.AssertCalled(t, "Send", mock.MatchedBy(func(msg string) bool { return len(msg) > 0 }))
	alertRepo.AssertCalled(t, "Insert", mock.Anything, mock.MatchedBy(func(a *model.Alert) bool {
		return a.Metric == "process:mysql"
	}))
}

func TestAlertWorker_ProcessRecovered(t *testing.T) {
	alertRepo := &mockAlertRepo{}
	thresholdRepo := &mockThresholdRepo{}
	serverRepo := &mockAlertServerRepo{}
	notifier := &mockNotifier{}

	thresholdRepo.On("Get", mock.Anything, "server-pr").Return(&model.Threshold{
		ServerID: "server-pr", CPUWarning: 75, CPUCritical: 90,
		MemWarning: 80, MemCritical: 95, DiskWarning: 80, DiskCritical: 90,
	}, nil)
	alertRepo.On("ResolveByServerAndMetric", mock.Anything, "server-pr", "process:nginx").
		Return(int64(1), nil)
	notifier.On("Send", mock.Anything).Return(nil)

	w := worker.NewAlertWorker(serverRepo, alertRepo, thresholdRepo, notifier)
	server := &model.Server{ID: "server-pr", Name: "ProcRecov", Status: model.StatusNormal}
	metric := &model.Metric{
		Time: time.Now(), ServerID: "server-pr",
		CPU: 10, Memory: 20, Disk: 30,
		Processes: []model.ProcessStatus{
			{Name: "nginx", Running: true},
		},
	}

	w.Check(context.Background(), server, metric)

	alertRepo.AssertCalled(t, "ResolveByServerAndMetric", mock.Anything, "server-pr", "process:nginx")
	notifier.AssertCalled(t, "Send", mock.Anything)
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
