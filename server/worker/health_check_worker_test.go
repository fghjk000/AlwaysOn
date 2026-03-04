package worker_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/worker"
	"github.com/stretchr/testify/mock"
)

type mockHCConfigRepo struct{ mock.Mock }

func (m *mockHCConfigRepo) ListEnabled(ctx context.Context) ([]model.HealthCheckConfig, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.HealthCheckConfig), args.Error(1)
}

type mockHCAlertRepo struct{ mock.Mock }

func (m *mockHCAlertRepo) Insert(ctx context.Context, a *model.Alert) error {
	return m.Called(ctx, a).Error(0)
}
func (m *mockHCAlertRepo) CanAlert(ctx context.Context, key string, cooldown time.Duration) (bool, error) {
	args := m.Called(ctx, key, cooldown)
	return args.Bool(0), args.Error(1)
}
func (m *mockHCAlertRepo) ResolveByServerAndMetric(ctx context.Context, serverID, metric string) (int64, error) {
	args := m.Called(ctx, serverID, metric)
	return args.Get(0).(int64), args.Error(1)
}

func TestHealthCheckWorker_HTTP_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	configRepo := &mockHCConfigRepo{}
	alertRepo := &mockHCAlertRepo{}
	notifier := &mockNotifier{}

	configRepo.On("ListEnabled", mock.Anything).Return([]model.HealthCheckConfig{
		{ID: "hc-1", ServerID: "s-1", Name: "TestHTTP",
			Type: "http", Target: srv.URL, ExpectedStatus: 200},
	}, nil)
	alertRepo.On("ResolveByServerAndMetric", mock.Anything, "s-1", "health_check:TestHTTP").
		Return(int64(0), nil)

	w := worker.NewHealthCheckWorker(configRepo, alertRepo, notifier)
	w.RunOnce(context.Background())

	// goroutine이 완료될 시간 대기
	time.Sleep(100 * time.Millisecond)

	alertRepo.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
	notifier.AssertNotCalled(t, "Send", mock.Anything)
}

func TestHealthCheckWorker_HTTP_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	configRepo := &mockHCConfigRepo{}
	alertRepo := &mockHCAlertRepo{}
	notifier := &mockNotifier{}

	configRepo.On("ListEnabled", mock.Anything).Return([]model.HealthCheckConfig{
		{ID: "hc-2", ServerID: "s-2", Name: "FailHTTP",
			Type: "http", Target: srv.URL, ExpectedStatus: 200},
	}, nil)
	alertRepo.On("CanAlert", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	alertRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)
	notifier.On("Send", mock.Anything).Return(nil)

	w := worker.NewHealthCheckWorker(configRepo, alertRepo, notifier)
	w.RunOnce(context.Background())

	time.Sleep(100 * time.Millisecond)

	alertRepo.AssertCalled(t, "Insert", mock.Anything, mock.Anything)
	notifier.AssertCalled(t, "Send", mock.Anything)
}
