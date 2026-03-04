# 모니터링 범위 확장 구현 계획

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** HTTP/포트 헬스체크(서버 사이드 폴링)와 프로세스 감시(에이전트 확장)를 추가한다.

**Architecture:** HealthCheckWorker goroutine이 DB에 등록된 HTTP/TCP 타겟을 30초마다 직접 폴링하고, 에이전트는 `agent.yaml`에 정의된 프로세스 목록을 체크해 메트릭과 함께 전송한다. 알림은 기존 `alerts` + `alert_cooldowns` 테이블을 재활용한다.

**Tech Stack:** Go 1.22+, pgx/v5, gopsutil/v3/process, net/http, net (TCP dial), testify/mock

---

### Task 1: DB 마이그레이션 - health_check_configs 테이블

**Files:**
- Create: `migrations/000003_health_check_configs.up.sql`
- Create: `migrations/000003_health_check_configs.down.sql`

**Step 1: up 마이그레이션 파일 작성**

`migrations/000003_health_check_configs.up.sql`:
```sql
CREATE TABLE health_check_configs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  server_id       UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  name            VARCHAR(255) NOT NULL,
  type            VARCHAR(10)  NOT NULL CHECK (type IN ('http', 'tcp')),
  target          VARCHAR(512) NOT NULL,
  expected_status INT          NOT NULL DEFAULT 200,
  interval_sec    INT          NOT NULL DEFAULT 30,
  enabled         BOOLEAN      NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
```

**Step 2: down 마이그레이션 파일 작성**

`migrations/000003_health_check_configs.down.sql`:
```sql
DROP TABLE IF EXISTS health_check_configs;
```

**Step 3: 마이그레이션 실행**

```bash
cd server && go run cmd/migrate/main.go up
```

Expected: `migrating... done` (오류 없음)

**Step 4: Commit**

```bash
git add migrations/
git commit -m "feat(db): health_check_configs 테이블 추가"
```

---

### Task 2: Model 확장

**Files:**
- Modify: `server/model/metric.go`
- Create: `server/model/health_check.go`

**Step 1: ProcessStatus 추가 + Metric 확장**

`server/model/metric.go` 전체를 아래로 교체:
```go
package model

import "time"

type ProcessStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
}

type Metric struct {
	Time      time.Time       `json:"time"`
	ServerID  string          `json:"server_id"`
	CPU       float64         `json:"cpu"`
	Memory    float64         `json:"memory"`
	Disk      float64         `json:"disk"`
	NetIn     int64           `json:"net_in"`
	NetOut    int64           `json:"net_out"`
	Processes []ProcessStatus `json:"processes,omitempty"`
}
```

**Step 2: HealthCheckConfig 모델 파일 생성**

`server/model/health_check.go`:
```go
package model

import "time"

type HealthCheckConfig struct {
	ID             string    `json:"id"`
	ServerID       string    `json:"server_id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`    // "http" | "tcp"
	Target         string    `json:"target"`  // URL 또는 host:port
	ExpectedStatus int       `json:"expected_status"`
	IntervalSec    int       `json:"interval_sec"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
}
```

**Step 3: 빌드 확인**

```bash
cd server && go build ./...
```

Expected: 오류 없음

**Step 4: Commit**

```bash
git add server/model/
git commit -m "feat(model): ProcessStatus, HealthCheckConfig 구조체 추가"
```

---

### Task 3: AlertRepo 확장 - ResolveByServerAndMetric

기존 `ResolveByServer`는 서버의 모든 알림을 resolve한다. 헬스체크/프로세스는 특정 metric만 resolve해야 한다.

**Files:**
- Modify: `server/repository/alert_repo.go`

**Step 1: 실패하는 테스트 작성**

`server/repository/alert_repo_test.go` (새 파일, 통합 테스트용):
```go
//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertRepo_ResolveByServerAndMetric(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewAlertRepo(pool)
	ctx := context.Background()

	// 서버 upsert
	serverRepo := repository.NewServerRepo(pool)
	server, err := serverRepo.Upsert(ctx, "test-hc-host", "Test HC")
	require.NoError(t, err)

	// 알림 2개 삽입 (다른 metric)
	err = repo.Insert(ctx, &model.Alert{
		ServerID: server.ID, Level: model.LevelCritical,
		Metric: "health_check:Nginx", Value: 0, Message: "fail",
	})
	require.NoError(t, err)
	err = repo.Insert(ctx, &model.Alert{
		ServerID: server.ID, Level: model.LevelCritical,
		Metric: "process:mysql", Value: 0, Message: "fail",
	})
	require.NoError(t, err)

	// Nginx 헬스체크 알림만 resolve
	affected, err := repo.ResolveByServerAndMetric(ctx, server.ID, "health_check:Nginx")
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	// mysql 프로세스 알림은 여전히 열려있어야 함
	alerts, err := repo.GetAll(ctx, 10)
	require.NoError(t, err)
	open := 0
	for _, a := range alerts {
		if a.ServerID == server.ID && a.ResolvedAt == nil {
			open++
		}
	}
	assert.Equal(t, 1, open)
}
```

**Step 2: 테스트 실행 (실패 확인)**

```bash
cd server && INTEGRATION=1 go test ./repository/... -run TestAlertRepo_ResolveByServerAndMetric -v
```

Expected: `FAIL` - `ResolveByServerAndMetric` undefined

**Step 3: ResolveByServerAndMetric 구현**

`server/repository/alert_repo.go`에 다음 메서드 추가:
```go
// ResolveByServerAndMetric: 특정 서버+metric의 미해결 알림을 resolve하고 영향받은 행 수를 반환한다.
func (r *AlertRepo) ResolveByServerAndMetric(ctx context.Context, serverID, metric string) (int64, error) {
	result, err := r.pool.Exec(ctx,
		`UPDATE alerts SET resolved_at = NOW()
		 WHERE server_id = $1 AND metric = $2 AND resolved_at IS NULL`,
		serverID, metric,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
```

**Step 4: 테스트 통과 확인**

```bash
cd server && INTEGRATION=1 go test ./repository/... -run TestAlertRepo_ResolveByServerAndMetric -v
```

Expected: `PASS`

**Step 5: 전체 테스트 확인**

```bash
cd server && go test ./...
```

Expected: 모두 PASS

**Step 6: Commit**

```bash
git add server/repository/
git commit -m "feat(repo): AlertRepo에 ResolveByServerAndMetric 추가"
```

---

### Task 4: HealthCheckRepo CRUD

**Files:**
- Create: `server/repository/health_check_repo.go`

**Step 1: 실패하는 테스트 작성**

`server/repository/health_check_repo_test.go`:
```go
//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheckRepo_CRUD(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewHealthCheckRepo(pool)
	serverRepo := repository.NewServerRepo(pool)
	ctx := context.Background()

	server, err := serverRepo.Upsert(ctx, "hc-test-host", "HC Test")
	require.NoError(t, err)

	// Insert
	cfg := &model.HealthCheckConfig{
		ServerID: server.ID, Name: "Nginx",
		Type: "http", Target: "http://localhost/health",
		ExpectedStatus: 200, IntervalSec: 30, Enabled: true,
	}
	err = repo.Insert(ctx, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.ID)

	// ListByServer
	list, err := repo.ListByServer(ctx, server.ID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "Nginx", list[0].Name)

	// ListEnabled (all enabled)
	all, err := repo.ListEnabled(ctx)
	require.NoError(t, err)
	found := false
	for _, c := range all {
		if c.ID == list[0].ID {
			found = true
		}
	}
	assert.True(t, found)

	// Delete
	err = repo.Delete(ctx, list[0].ID)
	require.NoError(t, err)

	list2, err := repo.ListByServer(ctx, server.ID)
	require.NoError(t, err)
	assert.Len(t, list2, 0)
}
```

**Step 2: 테스트 실행 (실패 확인)**

```bash
cd server && INTEGRATION=1 go test ./repository/... -run TestHealthCheckRepo_CRUD -v
```

Expected: `FAIL` - `NewHealthCheckRepo` undefined

**Step 3: HealthCheckRepo 구현**

`server/repository/health_check_repo.go`:
```go
package repository

import (
	"context"

	"github.com/alwayson/server/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthCheckRepo struct{ pool *pgxpool.Pool }

func NewHealthCheckRepo(pool *pgxpool.Pool) *HealthCheckRepo {
	return &HealthCheckRepo{pool: pool}
}

func (r *HealthCheckRepo) Insert(ctx context.Context, cfg *model.HealthCheckConfig) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO health_check_configs (server_id, name, type, target, expected_status, interval_sec, enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		cfg.ServerID, cfg.Name, cfg.Type, cfg.Target,
		cfg.ExpectedStatus, cfg.IntervalSec, cfg.Enabled,
	).Scan(&cfg.ID, &cfg.CreatedAt)
}

func (r *HealthCheckRepo) ListByServer(ctx context.Context, serverID string) ([]model.HealthCheckConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, server_id, name, type, target, expected_status, interval_sec, enabled, created_at
		 FROM health_check_configs WHERE server_id = $1 ORDER BY created_at`,
		serverID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHealthChecks(rows)
}

func (r *HealthCheckRepo) ListEnabled(ctx context.Context) ([]model.HealthCheckConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT hc.id, hc.server_id, hc.name, hc.type, hc.target,
		        hc.expected_status, hc.interval_sec, hc.enabled, hc.created_at
		 FROM health_check_configs hc
		 JOIN servers s ON s.id = hc.server_id
		 WHERE hc.enabled = true AND s.status != 'down'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHealthChecks(rows)
}

func (r *HealthCheckRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM health_check_configs WHERE id = $1`, id)
	return err
}

func scanHealthChecks(rows interface{ Next() bool; Scan(...any) error; Err() error }) ([]model.HealthCheckConfig, error) {
	var list []model.HealthCheckConfig
	for rows.Next() {
		var c model.HealthCheckConfig
		if err := rows.Scan(&c.ID, &c.ServerID, &c.Name, &c.Type, &c.Target,
			&c.ExpectedStatus, &c.IntervalSec, &c.Enabled, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}
```

**Step 4: 테스트 통과 확인**

```bash
cd server && INTEGRATION=1 go test ./repository/... -run TestHealthCheckRepo_CRUD -v
```

Expected: `PASS`

**Step 5: Commit**

```bash
git add server/repository/health_check_repo.go server/repository/health_check_repo_test.go
git commit -m "feat(repo): HealthCheckRepo CRUD 추가"
```

---

### Task 5: HealthCheckWorker

**Files:**
- Create: `server/worker/health_check_worker.go`
- Create: `server/worker/health_check_worker_test.go`

**Step 1: 실패하는 테스트 작성**

`server/worker/health_check_worker_test.go`:
```go
package worker_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
func (m *mockHCAlertRepo) CanAlert(ctx context.Context, key string, d interface{}) (bool, error) {
	args := m.Called(ctx, key, d)
	return args.Bool(0), args.Error(1)
}
func (m *mockHCAlertRepo) ResolveByServerAndMetric(ctx context.Context, serverID, metric string) (int64, error) {
	args := m.Called(ctx, serverID, metric)
	return args.Get(0).(int64), args.Error(1)
}

func TestHealthCheckWorker_HTTP_Success(t *testing.T) {
	// 테스트 HTTP 서버: 200 반환
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
	// 성공이므로 ResolveByServerAndMetric 호출 (recovered 0)
	alertRepo.On("ResolveByServerAndMetric", mock.Anything, "s-1", "health_check:TestHTTP").
		Return(int64(0), nil)

	w := worker.NewHealthCheckWorker(configRepo, alertRepo, notifier)
	w.RunOnce(context.Background())

	alertRepo.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
	notifier.AssertNotCalled(t, "Send", mock.Anything)
}

func TestHealthCheckWorker_HTTP_Failure(t *testing.T) {
	// 테스트 HTTP 서버: 500 반환
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

	alertRepo.AssertCalled(t, "Insert", mock.Anything, mock.Anything)
	notifier.AssertCalled(t, "Send", mock.Anything)
}
```

**Step 2: 테스트 실행 (실패 확인)**

```bash
cd server && go test ./worker/... -run TestHealthCheckWorker -v
```

Expected: `FAIL` - `NewHealthCheckWorker` undefined

**Step 3: HealthCheckWorker 구현**

`server/worker/health_check_worker.go`:
```go
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
```

**Step 4: 테스트 통과 확인**

```bash
cd server && go test ./worker/... -run TestHealthCheckWorker -v
```

Expected: `PASS`

**Step 5: 전체 테스트**

```bash
cd server && go test ./...
```

Expected: 모두 PASS

**Step 6: Commit**

```bash
git add server/worker/
git commit -m "feat(worker): HealthCheckWorker 추가 (HTTP/TCP 폴링, 알림)"
```

---

### Task 6: 프로세스 감시 - AlertWorker 확장

**Files:**
- Modify: `server/worker/alert_worker.go`
- Modify: `server/worker/alert_worker_test.go`

**Step 1: 실패하는 테스트 작성**

`server/worker/alert_worker_test.go`에 다음 테스트 추가:
```go
func TestAlertWorker_ProcessDown_Alert(t *testing.T) {
	alertRepo := &mockAlertRepo{}
	thresholdRepo := &mockThresholdRepo{}
	serverRepo := &mockAlertServerRepo{}
	notifier := &mockNotifier{}

	thresholdRepo.On("Get", mock.Anything, "server-p").Return(&model.Threshold{
		ServerID: "server-p", CPUWarning: 75, CPUCritical: 90,
		MemWarning: 80, MemCritical: 95, DiskWarning: 80, DiskCritical: 90,
	}, nil)
	// 프로세스 알림용 CanAlert
	alertRepo.On("CanAlert", mock.Anything, mock.MatchedBy(func(key string) bool {
		return key == "server-p:critical:process:mysql"
	}), mock.Anything).Return(true, nil)
	alertRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)
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
	// 정상 프로세스: ResolveByServerAndMetric 호출됨
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
	notifier.AssertCalled(t, "Send", mock.Anything) // 복구 알림
}
```

**주의:** `mockAlertRepo`에 `ResolveByServerAndMetric` 메서드를 추가해야 한다:
```go
func (m *mockAlertRepo) ResolveByServerAndMetric(ctx context.Context, serverID, metric string) (int64, error) {
	args := m.Called(ctx, serverID, metric)
	return args.Get(0).(int64), args.Error(1)
}
```

그리고 `AlertAlertRepo` 인터페이스에도 추가:
```go
type AlertAlertRepo interface {
	Insert(ctx context.Context, a *model.Alert) error
	ResolveByServer(ctx context.Context, serverID string) error
	CanAlert(ctx context.Context, key string, cooldown time.Duration) (bool, error)
	ResolveByServerAndMetric(ctx context.Context, serverID, metric string) (int64, error)
}
```

**Step 2: 테스트 실행 (실패 확인)**

```bash
cd server && go test ./worker/... -run TestAlertWorker_Process -v
```

Expected: `FAIL`

**Step 3: AlertWorker.Check()에 프로세스 감시 로직 추가**

`server/worker/alert_worker.go`의 `Check()` 함수 끝(복구 알림 로직 직전)에 추가:
```go
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
```

**Step 4: 테스트 통과 확인**

```bash
cd server && go test ./worker/... -v
```

Expected: 모두 PASS

**Step 5: Commit**

```bash
git add server/worker/alert_worker.go server/worker/alert_worker_test.go
git commit -m "feat(worker): AlertWorker에 프로세스 감시 로직 추가"
```

---

### Task 7: 에이전트 확장 - 프로세스 수집

**Files:**
- Modify: `agent/main.go` (AgentConfig에 Processes 추가)
- Modify: `agent/collector.go` (ProcessStatus, CollectProcesses 추가, MetricPayload 확장)

**Step 1: 실패하는 테스트 작성**

`agent/collector_test.go`:
```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectProcesses_CurrentProcess(t *testing.T) {
	// 현재 실행 중인 프로세스 이름으로 테스트 (플랫폼마다 다를 수 있음)
	// "go" 프로세스는 테스트 실행 중 반드시 존재
	result := CollectProcesses([]string{"nonexistent-proc-xyz"})

	assert.Len(t, result, 1)
	assert.Equal(t, "nonexistent-proc-xyz", result[0].Name)
	assert.False(t, result[0].Running)
}
```

**Step 2: 테스트 실행 (실패 확인)**

```bash
cd agent && go test ./... -run TestCollectProcesses -v
```

Expected: `FAIL` - `CollectProcesses` undefined

**Step 3: MetricPayload와 CollectProcesses 구현**

`agent/collector.go` 전체 교체:
```go
package main

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type ProcessStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
}

type MetricPayload struct {
	Host      string          `json:"host"`
	Name      string          `json:"name"`
	CPU       float64         `json:"cpu"`
	Memory    float64         `json:"memory"`
	Disk      float64         `json:"disk"`
	NetIn     int64           `json:"net_in"`
	NetOut    int64           `json:"net_out"`
	Processes []ProcessStatus `json:"processes,omitempty"`
}

func CollectMetrics() (*MetricPayload, error) {
	cpuPercents, err := cpu.Percent(0, false)
	if err != nil {
		return nil, err
	}

	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	diskStat, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}

	netStats, err := net.IOCounters(false)
	var netIn, netOut int64
	if err == nil && len(netStats) > 0 {
		netIn = int64(netStats[0].BytesRecv)
		netOut = int64(netStats[0].BytesSent)
	}

	return &MetricPayload{
		CPU:    cpuPercents[0],
		Memory: vmStat.UsedPercent,
		Disk:   diskStat.UsedPercent,
		NetIn:  netIn,
		NetOut: netOut,
	}, nil
}

// CollectProcesses: names 목록의 프로세스 실행 여부를 반환한다.
func CollectProcesses(names []string) []ProcessStatus {
	if len(names) == 0 {
		return nil
	}

	procs, _ := process.Processes()
	running := make(map[string]bool, len(procs))
	for _, p := range procs {
		if n, err := p.Name(); err == nil {
			running[n] = true
		}
	}

	result := make([]ProcessStatus, len(names))
	for i, name := range names {
		result[i] = ProcessStatus{Name: name, Running: running[name]}
	}
	return result
}
```

**Step 4: AgentConfig에 Processes 추가 + main.go 연결**

`agent/main.go`의 `AgentConfig` 구조체를 다음으로 교체:
```go
type AgentConfig struct {
	ServerURL string   `yaml:"server_url"`
	Host      string   `yaml:"host"`
	Name      string   `yaml:"name"`
	Interval  int      `yaml:"interval_seconds"`
	Processes []string `yaml:"processes"`
}
```

`main.go`의 메트릭 전송 루프에서 `CollectMetrics()` 결과에 프로세스 추가:
```go
// 기존: payload.Host = cfg.Host 아래에 추가
payload.Processes = CollectProcesses(cfg.Processes)
```

**Step 5: 테스트 통과 확인**

```bash
cd agent && go test ./... -run TestCollectProcesses -v
```

Expected: `PASS`

**Step 6: 에이전트 빌드 확인**

```bash
cd agent && go build ./...
```

Expected: 오류 없음

**Step 7: agent.yaml.example 업데이트**

`agent/agent.yaml.example`에 다음 추가:
```yaml
# 감시할 프로세스 목록 (없으면 생략)
processes:
  # - nginx
  # - mysql
```

**Step 8: Commit**

```bash
git add agent/
git commit -m "feat(agent): 프로세스 감시 추가 (agent.yaml processes 설정)"
```

---

### Task 8: API 핸들러 - 헬스체크 CRUD + MetricHandler 확장

**Files:**
- Create: `server/api/health_check_handler.go`
- Modify: `server/api/metric_handler.go`
- Modify: `server/api/router.go`

**Step 1: MetricHandler processes 파싱 추가**

`server/api/metric_handler.go`의 `metricRequest` 구조체에 필드 추가:
```go
type metricRequest struct {
	Host      string                `json:"host" binding:"required"`
	Name      string                `json:"name"`
	CPU       float64               `json:"cpu"`
	Memory    float64               `json:"memory"`
	Disk      float64               `json:"disk"`
	NetIn     int64                 `json:"net_in"`
	NetOut    int64                 `json:"net_out"`
	Processes []model.ProcessStatus `json:"processes,omitempty"`
}
```

`Receive()` 함수의 `svc.Process()` 호출에 Processes 전달:
```go
_, err := h.svc.Process(c.Request.Context(), &service.MetricInput{
	Host: req.Host, Name: req.Name,
	CPU: req.CPU, Memory: req.Memory, Disk: req.Disk,
	NetIn: req.NetIn, NetOut: req.NetOut,
	Processes: req.Processes,
})
```

`server/api/metric_handler.go` 상단에 import 추가:
```go
import (
	"net/http"

	"github.com/alwayson/server/model"
	"github.com/alwayson/server/service"
	"github.com/gin-gonic/gin"
)
```

**Step 2: MetricService MetricInput에 Processes 추가**

`server/service/metric_service.go`의 `MetricInput`에 추가:
```go
type MetricInput struct {
	Host      string
	Name      string
	CPU       float64
	Memory    float64
	Disk      float64
	NetIn     int64
	NetOut    int64
	Processes []model.ProcessStatus
}
```

`Process()` 함수에서 metric 생성 시 Processes 포함:
```go
metric := &model.Metric{
	Time:      now,
	ServerID:  server.ID,
	CPU:       input.CPU,
	Memory:    input.Memory,
	Disk:      input.Disk,
	NetIn:     input.NetIn,
	NetOut:    input.NetOut,
	Processes: input.Processes,
}
```

**Step 3: HealthCheck 핸들러 작성**

`server/api/health_check_handler.go`:
```go
package api

import (
	"context"
	"net/http"

	"github.com/alwayson/server/model"
	"github.com/gin-gonic/gin"
)

type HealthCheckRepository interface {
	Insert(ctx context.Context, cfg *model.HealthCheckConfig) error
	ListByServer(ctx context.Context, serverID string) ([]model.HealthCheckConfig, error)
	Delete(ctx context.Context, id string) error
}

type HealthCheckHandler struct {
	repo HealthCheckRepository
}

func NewHealthCheckHandler(repo HealthCheckRepository) *HealthCheckHandler {
	return &HealthCheckHandler{repo: repo}
}

type healthCheckRequest struct {
	Name           string `json:"name" binding:"required"`
	Type           string `json:"type" binding:"required,oneof=http tcp"`
	Target         string `json:"target" binding:"required"`
	ExpectedStatus int    `json:"expected_status"`
	IntervalSec    int    `json:"interval_sec"`
}

func (h *HealthCheckHandler) Create(c *gin.Context) {
	serverID := c.Param("id")
	var req healthCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ExpectedStatus == 0 {
		req.ExpectedStatus = 200
	}
	if req.IntervalSec <= 0 {
		req.IntervalSec = 30
	}
	cfg := &model.HealthCheckConfig{
		ServerID:       serverID,
		Name:           req.Name,
		Type:           req.Type,
		Target:         req.Target,
		ExpectedStatus: req.ExpectedStatus,
		IntervalSec:    req.IntervalSec,
		Enabled:        true,
	}
	if err := h.repo.Insert(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cfg)
}

func (h *HealthCheckHandler) List(c *gin.Context) {
	serverID := c.Param("id")
	list, err := h.repo.ListByServer(c.Request.Context(), serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []model.HealthCheckConfig{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *HealthCheckHandler) Delete(c *gin.Context) {
	id := c.Param("hid")
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
```

**Step 4: Router에 헬스체크 라우트 등록**

`server/api/router.go`의 `Handlers` 구조체에 추가:
```go
type Handlers struct {
	Metric        *MetricHandler
	ServerH       *ServerHandler
	AlertH        *AlertHandler
	HealthCheckH  *HealthCheckHandler
}
```

`NewRouter()`의 라우트 그룹에 추가:
```go
api.GET("/servers/:id/health-checks", h.HealthCheckH.List)
api.POST("/servers/:id/health-checks", h.HealthCheckH.Create)
api.DELETE("/servers/:id/health-checks/:hid", h.HealthCheckH.Delete)
```

CORS 미들웨어의 `Allow-Methods`에 DELETE 추가:
```go
c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
```

**Step 5: 빌드 확인**

```bash
cd server && go build ./...
```

Expected: 오류 없음

**Step 6: 전체 테스트**

```bash
cd server && go test ./...
```

Expected: 모두 PASS

**Step 7: Commit**

```bash
git add server/api/ server/service/metric_service.go
git commit -m "feat(api): 헬스체크 CRUD 핸들러 추가, MetricHandler processes 파싱 추가"
```

---

### Task 9: server/main.go - HealthCheckWorker 등록

**Files:**
- Modify: `server/main.go`

**Step 1: main.go에 HealthCheckWorker 추가**

기존 `downDetector.Start()` 이후에 추가:
```go
healthCheckRepo := repository.NewHealthCheckRepo(pool)
healthCheckWorker := worker.NewHealthCheckWorker(healthCheckRepo, alertRepo, notifier)
healthCheckWorker.Start(context.Background())
log.Println("HealthCheckWorker 시작 (30초 간격)")
```

`Handlers` 초기화에도 추가:
```go
handlers := &api.Handlers{
	Metric:       api.NewMetricHandler(metricSvc),
	ServerH:      api.NewServerHandler(serverRepo, metricRepo, thresholdRepo),
	AlertH:       api.NewAlertHandler(alertRepo),
	HealthCheckH: api.NewHealthCheckHandler(healthCheckRepo),
}
```

**Step 2: 빌드 확인**

```bash
cd server && go build ./...
```

Expected: 오류 없음

**Step 3: Commit**

```bash
git add server/main.go
git commit -m "feat: main.go에 HealthCheckWorker 등록"
```

---

### Task 10: Frontend - 헬스체크 패널 + 프로세스 뱃지

**Files:**
- Modify: `frontend/src/api/index.js` (또는 .ts)
- Modify: `frontend/src/pages/ServerDetail.jsx` (또는 .tsx)

**Step 1: API 함수 추가**

`frontend/src/api/index.js`에 추가:
```js
export const getHealthChecks = (serverId) =>
  fetch(`/api/servers/${serverId}/health-checks`).then(r => r.json());

export const createHealthCheck = (serverId, data) =>
  fetch(`/api/servers/${serverId}/health-checks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  }).then(r => r.json());

export const deleteHealthCheck = (serverId, hid) =>
  fetch(`/api/servers/${serverId}/health-checks/${hid}`, {
    method: 'DELETE',
  }).then(r => r.json());
```

**Step 2: ServerDetail 페이지에 프로세스 뱃지 추가**

서버 상세 페이지의 최신 메트릭 데이터에 프로세스 정보가 포함되어 있으면 아래 컴포넌트를 렌더링:

```jsx
{/* 프로세스 상태 */}
{latestMetric?.processes?.length > 0 && (
  <div className="bg-white rounded-lg shadow p-4 mb-4">
    <h3 className="font-semibold text-gray-700 mb-2">프로세스</h3>
    <div className="flex flex-wrap gap-2">
      {latestMetric.processes.map(p => (
        <span
          key={p.name}
          className={`px-3 py-1 rounded-full text-sm font-medium ${
            p.running
              ? 'bg-green-100 text-green-700'
              : 'bg-red-100 text-red-700'
          }`}
        >
          {p.running ? '●' : '○'} {p.name}
        </span>
      ))}
    </div>
  </div>
)}
```

**Step 3: 헬스체크 패널 추가**

ServerDetail 페이지에 useEffect로 헬스체크 목록 로드 + 렌더링:

```jsx
// state
const [healthChecks, setHealthChecks] = useState([]);
const [showHCForm, setShowHCForm] = useState(false);
const [hcForm, setHcForm] = useState({ name: '', type: 'http', target: '', expected_status: 200 });

// 로드
useEffect(() => {
  getHealthChecks(serverId).then(setHealthChecks);
}, [serverId]);

// 렌더링
<div className="bg-white rounded-lg shadow p-4 mb-4">
  <div className="flex justify-between items-center mb-2">
    <h3 className="font-semibold text-gray-700">헬스체크</h3>
    <button
      onClick={() => setShowHCForm(!showHCForm)}
      className="text-sm text-blue-600 hover:underline"
    >
      + 추가
    </button>
  </div>

  {showHCForm && (
    <form onSubmit={async (e) => {
      e.preventDefault();
      const created = await createHealthCheck(serverId, hcForm);
      setHealthChecks(prev => [...prev, created]);
      setShowHCForm(false);
    }} className="mb-3 flex gap-2 flex-wrap">
      <input
        placeholder="이름 (예: Nginx)"
        value={hcForm.name}
        onChange={e => setHcForm(f => ({ ...f, name: e.target.value }))}
        className="border rounded px-2 py-1 text-sm flex-1"
        required
      />
      <select
        value={hcForm.type}
        onChange={e => setHcForm(f => ({ ...f, type: e.target.value }))}
        className="border rounded px-2 py-1 text-sm"
      >
        <option value="http">HTTP</option>
        <option value="tcp">TCP</option>
      </select>
      <input
        placeholder="http://... 또는 host:port"
        value={hcForm.target}
        onChange={e => setHcForm(f => ({ ...f, target: e.target.value }))}
        className="border rounded px-2 py-1 text-sm flex-1"
        required
      />
      <button type="submit" className="bg-blue-500 text-white px-3 py-1 rounded text-sm">
        저장
      </button>
    </form>
  )}

  {healthChecks.length === 0 ? (
    <p className="text-gray-400 text-sm">등록된 헬스체크 없음</p>
  ) : (
    <ul className="divide-y">
      {healthChecks.map(hc => (
        <li key={hc.id} className="flex justify-between items-center py-1 text-sm">
          <span>
            <span className="font-mono text-gray-500 mr-2">[{hc.type.toUpperCase()}]</span>
            {hc.name} — {hc.target}
          </span>
          <button
            onClick={async () => {
              await deleteHealthCheck(serverId, hc.id);
              setHealthChecks(prev => prev.filter(h => h.id !== hc.id));
            }}
            className="text-red-400 hover:text-red-600 ml-2"
          >
            삭제
          </button>
        </li>
      ))}
    </ul>
  )}
</div>
```

**Step 4: 프론트엔드 빌드 확인**

```bash
cd frontend && npm run build
```

Expected: 오류 없음

**Step 5: Commit**

```bash
git add frontend/
git commit -m "feat(frontend): 헬스체크 패널, 프로세스 뱃지 추가"
```

---

### Task 11: 최종 검증

**Step 1: 전체 서버 테스트**

```bash
cd server && go test ./...
```

Expected: 모두 PASS

**Step 2: 에이전트 테스트**

```bash
cd agent && go test ./... -v
```

Expected: 모두 PASS

**Step 3: 빌드 최종 확인**

```bash
cd server && go build ./... && cd ../agent && go build ./... && cd ../frontend && npm run build
```

Expected: 모두 오류 없음

**Step 4: 완료 커밋**

```bash
git tag v0.2.0-monitoring-expansion
```
