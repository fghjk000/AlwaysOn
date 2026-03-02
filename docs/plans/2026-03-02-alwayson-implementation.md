# AlwaysOn Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 온프레미스 서버에 Go 에이전트를 설치하여 메트릭을 Push 방식으로 수집하고, 장애를 감지해 Slack으로 알림을 전송하는 실시간 모니터링 플랫폼 구축

**Architecture:** 각 서버에 Go 에이전트를 설치해 5초 간격으로 중앙 서버에 메트릭을 Push. 중앙 서버(Go+Gin)는 메트릭을 TimescaleDB에 저장하고, Alert Worker goroutine이 임계값을 체크하여 Slack Webhook으로 알림 전송. React+Vite 대시보드가 REST API로 데이터 조회.

**Tech Stack:** Go 1.22+, Gin v1, pgx/v5, golang-migrate, gopsutil/v3, React 18, Vite, recharts, Tailwind CSS, PostgreSQL 16 + TimescaleDB 2.x, Docker Compose

---

## Task 1: 프로젝트 초기 설정

**Files:**
- Create: `docker-compose.yml`
- Create: `server/go.mod`
- Create: `agent/go.mod`

**Step 1: docker-compose.yml 작성**

```yaml
# docker-compose.yml
version: '3.8'
services:
  db:
    image: timescale/timescaledb:latest-pg16
    environment:
      POSTGRES_DB: alwayson
      POSTGRES_USER: alwayson
      POSTGRES_PASSWORD: alwayson123
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

**Step 2: Docker Compose 실행 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
docker-compose up -d
docker-compose ps
```
Expected: db 컨테이너 상태 `Up`

**Step 3: Go 서버 모듈 초기화**

```bash
mkdir -p server agent
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go mod init github.com/alwayson/server
go get github.com/gin-gonic/gin@v1.9.1
go get github.com/jackc/pgx/v5@v5.5.4
go get github.com/golang-migrate/migrate/v4@v4.17.0
go get github.com/golang-migrate/migrate/v4/database/pgx/v5
go get github.com/golang-migrate/migrate/v4/source/file
go get github.com/google/uuid@v1.6.0
go get github.com/stretchr/testify@v1.9.0
```

**Step 4: Go 에이전트 모듈 초기화**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/agent
go mod init github.com/alwayson/agent
go get github.com/shirou/gopsutil/v3@v3.24.1
go get gopkg.in/yaml.v3@v3.0.1
```

**Step 5: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add docker-compose.yml server/go.mod server/go.sum agent/go.mod agent/go.sum
git commit -m "chore: 프로젝트 초기 설정 및 Go 모듈 초기화"
```

---

## Task 2: DB 마이그레이션

**Files:**
- Create: `migrations/000001_init.up.sql`
- Create: `migrations/000001_init.down.sql`

**Step 1: up 마이그레이션 작성**

```sql
-- migrations/000001_init.up.sql
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE servers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    host       VARCHAR(255) NOT NULL UNIQUE,
    status     VARCHAR(20) NOT NULL DEFAULT 'normal',
    last_seen  TIMESTAMPTZ
);

CREATE TABLE metrics (
    time       TIMESTAMPTZ NOT NULL,
    server_id  UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    cpu        DOUBLE PRECISION NOT NULL,
    memory     DOUBLE PRECISION NOT NULL,
    disk       DOUBLE PRECISION NOT NULL,
    net_in     BIGINT NOT NULL DEFAULT 0,
    net_out    BIGINT NOT NULL DEFAULT 0
);

SELECT create_hypertable('metrics', 'time');

CREATE TABLE alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id   UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    level       VARCHAR(20) NOT NULL,
    metric      VARCHAR(50) NOT NULL,
    value       DOUBLE PRECISION NOT NULL,
    message     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE TABLE thresholds (
    server_id       UUID PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    cpu_warning     DOUBLE PRECISION NOT NULL DEFAULT 75,
    cpu_critical    DOUBLE PRECISION NOT NULL DEFAULT 90,
    mem_warning     DOUBLE PRECISION NOT NULL DEFAULT 80,
    mem_critical    DOUBLE PRECISION NOT NULL DEFAULT 95,
    disk_warning    DOUBLE PRECISION NOT NULL DEFAULT 80,
    disk_critical   DOUBLE PRECISION NOT NULL DEFAULT 90
);
```

**Step 2: down 마이그레이션 작성**

```sql
-- migrations/000001_init.down.sql
DROP TABLE IF EXISTS thresholds;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS metrics;
DROP TABLE IF EXISTS servers;
```

**Step 3: 마이그레이션 실행 테스트**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go run -tags 'pgx5' cmd/migrate/main.go up
```

(이 파일은 Task 3에서 작성)

**Step 4: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add migrations/
git commit -m "feat: DB 스키마 마이그레이션 (TimescaleDB hypertable 포함)"
```

---

## Task 3: 백엔드 Config + 모델 + 마이그레이션 실행기

**Files:**
- Create: `server/config/config.go`
- Create: `server/model/server.go`
- Create: `server/model/metric.go`
- Create: `server/model/alert.go`
- Create: `server/model/threshold.go`
- Create: `server/cmd/migrate/main.go`
- Create: `server/.env.example`

**Step 1: Config 작성**

```go
// server/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	ServerPort string
	SlackWebhookURL string
}

func Load() *Config {
	return &Config{
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBUser:          getEnv("DB_USER", "alwayson"),
		DBPassword:      getEnv("DB_PASSWORD", "alwayson123"),
		DBName:          getEnv("DB_NAME", "alwayson"),
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		SlackWebhookURL: getEnv("SLACK_WEBHOOK_URL", ""),
	}
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
```

**Step 2: 모델 파일 작성**

```go
// server/model/server.go
package model

import "time"

type ServerStatus string

const (
	StatusNormal   ServerStatus = "normal"
	StatusWarning  ServerStatus = "warning"
	StatusCritical ServerStatus = "critical"
	StatusDown     ServerStatus = "down"
)

type Server struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Host     string       `json:"host"`
	Status   ServerStatus `json:"status"`
	LastSeen *time.Time   `json:"last_seen"`
}
```

```go
// server/model/metric.go
package model

import "time"

type Metric struct {
	Time     time.Time `json:"time"`
	ServerID string    `json:"server_id"`
	CPU      float64   `json:"cpu"`
	Memory   float64   `json:"memory"`
	Disk     float64   `json:"disk"`
	NetIn    int64     `json:"net_in"`
	NetOut   int64     `json:"net_out"`
}
```

```go
// server/model/alert.go
package model

import "time"

type AlertLevel string

const (
	LevelWarning  AlertLevel = "warning"
	LevelCritical AlertLevel = "critical"
	LevelDown     AlertLevel = "down"
)

type Alert struct {
	ID         string     `json:"id"`
	ServerID   string     `json:"server_id"`
	Level      AlertLevel `json:"level"`
	Metric     string     `json:"metric"`
	Value      float64    `json:"value"`
	Message    string     `json:"message"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}
```

```go
// server/model/threshold.go
package model

type Threshold struct {
	ServerID     string  `json:"server_id"`
	CPUWarning   float64 `json:"cpu_warning"`
	CPUCritical  float64 `json:"cpu_critical"`
	MemWarning   float64 `json:"mem_warning"`
	MemCritical  float64 `json:"mem_critical"`
	DiskWarning  float64 `json:"disk_warning"`
	DiskCritical float64 `json:"disk_critical"`
}

func DefaultThreshold(serverID string) Threshold {
	return Threshold{
		ServerID:     serverID,
		CPUWarning:   75,
		CPUCritical:  90,
		MemWarning:   80,
		MemCritical:  95,
		DiskWarning:  80,
		DiskCritical: 90,
	}
}
```

**Step 3: 마이그레이션 실행기 작성**

```go
// server/cmd/migrate/main.go
package main

import (
	"flag"
	"log"
	"os"

	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4"
	"github.com/alwayson/server/config"
)

func main() {
	direction := flag.String("direction", "up", "up or down")
	flag.Parse()
	if len(os.Args) > 1 {
		*direction = os.Args[1]
	}

	cfg := config.Load()
	dbURL := "pgx5://" + cfg.DBUser + ":" + cfg.DBPassword + "@" + cfg.DBHost + ":" + cfg.DBPort + "/" + cfg.DBName + "?sslmode=disable"

	src, err := (&file.File{}).Open("file://../../migrations")
	if err != nil {
		log.Fatal(err)
	}

	m, err := migrate.NewWithSourceInstance("file", src, dbURL)
	if err != nil {
		log.Fatal(err)
	}

	switch *direction {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatal(err)
		}
		log.Println("Migration up complete")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatal(err)
		}
		log.Println("Migration down complete")
	}
}
```

**Step 4: .env.example 작성**

```bash
# server/.env.example
DB_HOST=localhost
DB_PORT=5432
DB_USER=alwayson
DB_PASSWORD=alwayson123
DB_NAME=alwayson
SERVER_PORT=8080
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXXXXXX
```

**Step 5: 마이그레이션 실행 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go run cmd/migrate/main.go up
```
Expected: `Migration up complete`

**Step 6: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add server/
git commit -m "feat: 백엔드 config, 모델, 마이그레이션 실행기 추가"
```

---

## Task 4: Repository 레이어 (DB 쿼리)

**Files:**
- Create: `server/repository/db.go`
- Create: `server/repository/server_repo.go`
- Create: `server/repository/metric_repo.go`
- Create: `server/repository/alert_repo.go`
- Create: `server/repository/threshold_repo.go`
- Create: `server/repository/server_repo_test.go`

**Step 1: DB 연결 작성**

```go
// server/repository/db.go
package repository

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/alwayson/server/config"
)

func NewPool(cfg *config.Config) *pgxpool.Pool {
	pool, err := pgxpool.New(context.Background(), cfg.DSN())
	if err != nil {
		log.Fatalf("DB 연결 실패: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("DB ping 실패: %v", err)
	}
	return pool
}
```

**Step 2: ServerRepo 테스트 작성 (TDD)**

```go
// server/repository/server_repo_test.go
package repository_test

import (
	"context"
	"testing"
	"os"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/alwayson/server/config"
	"github.com/alwayson/server/model"
	"github.com/alwayson/server/repository"
)

func testPool(t *testing.T) *repository.ServerRepo {
	t.Helper()
	cfg := config.Load()
	pool := repository.NewPool(cfg)
	t.Cleanup(func() { pool.Close() })
	return repository.NewServerRepo(pool)
}

func TestServerRepo_UpsertAndGet(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("INTEGRATION 환경변수 필요")
	}
	repo := testPool(t)
	ctx := context.Background()

	server, err := repo.Upsert(ctx, "test-host-01", "Test Server 01")
	require.NoError(t, err)
	assert.NotEmpty(t, server.ID)
	assert.Equal(t, "test-host-01", server.Host)

	got, err := repo.GetByHost(ctx, "test-host-01")
	require.NoError(t, err)
	assert.Equal(t, server.ID, got.ID)

	// 정리
	_ = repo.Delete(ctx, server.ID)
}
```

**Step 3: 테스트 실패 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
INTEGRATION=1 go test ./repository/... -run TestServerRepo_UpsertAndGet -v
```
Expected: FAIL - `repository.NewServerRepo` 없음

**Step 4: ServerRepo 구현**

```go
// server/repository/server_repo.go
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/alwayson/server/model"
)

type ServerRepo struct{ pool *pgxpool.Pool }

func NewServerRepo(pool *pgxpool.Pool) *ServerRepo {
	return &ServerRepo{pool: pool}
}

func (r *ServerRepo) Upsert(ctx context.Context, host, name string) (*model.Server, error) {
	sql := `
		INSERT INTO servers (name, host, status, last_seen)
		VALUES ($1, $2, 'normal', NOW())
		ON CONFLICT (host) DO UPDATE SET last_seen = NOW()
		RETURNING id, name, host, status, last_seen`
	var s model.Server
	err := r.pool.QueryRow(ctx, sql, name, host).Scan(
		&s.ID, &s.Name, &s.Host, &s.Status, &s.LastSeen,
	)
	return &s, err
}

func (r *ServerRepo) GetByHost(ctx context.Context, host string) (*model.Server, error) {
	sql := `SELECT id, name, host, status, last_seen FROM servers WHERE host = $1`
	var s model.Server
	err := r.pool.QueryRow(ctx, sql, host).Scan(
		&s.ID, &s.Name, &s.Host, &s.Status, &s.LastSeen,
	)
	return &s, err
}

func (r *ServerRepo) GetAll(ctx context.Context) ([]model.Server, error) {
	sql := `SELECT id, name, host, status, last_seen FROM servers ORDER BY name`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []model.Server
	for rows.Next() {
		var s model.Server
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Status, &s.LastSeen); err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (r *ServerRepo) UpdateStatus(ctx context.Context, id string, status model.ServerStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE servers SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *ServerRepo) UpdateLastSeen(ctx context.Context, id string, t time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE servers SET last_seen = $1 WHERE id = $2`, t, id)
	return err
}

func (r *ServerRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM servers WHERE id = $1`, id)
	return err
}
```

**Step 5: MetricRepo 구현**

```go
// server/repository/metric_repo.go
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/alwayson/server/model"
)

type MetricRepo struct{ pool *pgxpool.Pool }

func NewMetricRepo(pool *pgxpool.Pool) *MetricRepo {
	return &MetricRepo{pool: pool}
}

func (r *MetricRepo) Insert(ctx context.Context, m *model.Metric) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO metrics (time, server_id, cpu, memory, disk, net_in, net_out)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		m.Time, m.ServerID, m.CPU, m.Memory, m.Disk, m.NetIn, m.NetOut,
	)
	return err
}

func (r *MetricRepo) GetRecent(ctx context.Context, serverID string, since time.Time) ([]model.Metric, error) {
	sql := `
		SELECT time, server_id, cpu, memory, disk, net_in, net_out
		FROM metrics
		WHERE server_id = $1 AND time >= $2
		ORDER BY time ASC`
	rows, err := r.pool.Query(ctx, sql, serverID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []model.Metric
	for rows.Next() {
		var m model.Metric
		if err := rows.Scan(&m.Time, &m.ServerID, &m.CPU, &m.Memory, &m.Disk, &m.NetIn, &m.NetOut); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (r *MetricRepo) GetLatest(ctx context.Context, serverID string) (*model.Metric, error) {
	sql := `
		SELECT time, server_id, cpu, memory, disk, net_in, net_out
		FROM metrics WHERE server_id = $1
		ORDER BY time DESC LIMIT 1`
	var m model.Metric
	err := r.pool.QueryRow(ctx, sql, serverID).Scan(
		&m.Time, &m.ServerID, &m.CPU, &m.Memory, &m.Disk, &m.NetIn, &m.NetOut,
	)
	return &m, err
}
```

**Step 6: AlertRepo 구현**

```go
// server/repository/alert_repo.go
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/alwayson/server/model"
)

type AlertRepo struct{ pool *pgxpool.Pool }

func NewAlertRepo(pool *pgxpool.Pool) *AlertRepo {
	return &AlertRepo{pool: pool}
}

func (r *AlertRepo) Insert(ctx context.Context, a *model.Alert) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO alerts (id, server_id, level, metric, value, message, created_at)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW())`,
		a.ServerID, a.Level, a.Metric, a.Value, a.Message,
	)
	return err
}

func (r *AlertRepo) GetAll(ctx context.Context, limit int) ([]model.Alert, error) {
	sql := `
		SELECT id, server_id, level, metric, value, message, created_at, resolved_at
		FROM alerts ORDER BY created_at DESC LIMIT $1`
	rows, err := r.pool.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []model.Alert
	for rows.Next() {
		var a model.Alert
		if err := rows.Scan(&a.ID, &a.ServerID, &a.Level, &a.Metric, &a.Value,
			&a.Message, &a.CreatedAt, &a.ResolvedAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (r *AlertRepo) ResolveByServer(ctx context.Context, serverID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE alerts SET resolved_at = $1
		 WHERE server_id = $2 AND resolved_at IS NULL`,
		time.Now(), serverID,
	)
	return err
}
```

**Step 7: ThresholdRepo 구현**

```go
// server/repository/threshold_repo.go
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/alwayson/server/model"
)

type ThresholdRepo struct{ pool *pgxpool.Pool }

func NewThresholdRepo(pool *pgxpool.Pool) *ThresholdRepo {
	return &ThresholdRepo{pool: pool}
}

func (r *ThresholdRepo) Get(ctx context.Context, serverID string) (*model.Threshold, error) {
	sql := `
		SELECT server_id, cpu_warning, cpu_critical, mem_warning, mem_critical,
		       disk_warning, disk_critical
		FROM thresholds WHERE server_id = $1`
	var t model.Threshold
	err := r.pool.QueryRow(ctx, sql, serverID).Scan(
		&t.ServerID, &t.CPUWarning, &t.CPUCritical,
		&t.MemWarning, &t.MemCritical,
		&t.DiskWarning, &t.DiskCritical,
	)
	if err != nil {
		// 커스텀 설정 없으면 기본값 반환
		d := model.DefaultThreshold(serverID)
		return &d, nil
	}
	return &t, nil
}

func (r *ThresholdRepo) Upsert(ctx context.Context, t *model.Threshold) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO thresholds (server_id, cpu_warning, cpu_critical, mem_warning, mem_critical, disk_warning, disk_critical)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (server_id) DO UPDATE SET
		   cpu_warning=$2, cpu_critical=$3,
		   mem_warning=$4, mem_critical=$5,
		   disk_warning=$6, disk_critical=$7`,
		t.ServerID, t.CPUWarning, t.CPUCritical,
		t.MemWarning, t.MemCritical,
		t.DiskWarning, t.DiskCritical,
	)
	return err
}
```

**Step 8: 테스트 통과 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
INTEGRATION=1 go test ./repository/... -run TestServerRepo_UpsertAndGet -v
```
Expected: PASS

**Step 9: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add server/repository/
git commit -m "feat: DB repository 레이어 구현 (Server/Metric/Alert/Threshold)"
```

---

## Task 5: Go 에이전트 구현

**Files:**
- Create: `agent/collector.go`
- Create: `agent/sender.go`
- Create: `agent/main.go`
- Create: `agent/agent.yaml.example`
- Create: `agent/collector_test.go`

**Step 1: Collector 테스트 작성**

```go
// agent/collector_test.go
package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCollectMetrics_ReturnsValidRanges(t *testing.T) {
	m, err := CollectMetrics()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, m.CPU, 0.0)
	assert.LessOrEqual(t, m.CPU, 100.0)
	assert.GreaterOrEqual(t, m.Memory, 0.0)
	assert.LessOrEqual(t, m.Memory, 100.0)
	assert.GreaterOrEqual(t, m.Disk, 0.0)
	assert.LessOrEqual(t, m.Disk, 100.0)
}
```

**Step 2: 테스트 실패 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/agent
go test ./... -v
```
Expected: FAIL - `CollectMetrics` 없음

**Step 3: collector.go 구현**

```go
// agent/collector.go
package main

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type MetricPayload struct {
	ServerID string  `json:"server_id"`
	Host     string  `json:"host"`
	Name     string  `json:"name"`
	CPU      float64 `json:"cpu"`
	Memory   float64 `json:"memory"`
	Disk     float64 `json:"disk"`
	NetIn    int64   `json:"net_in"`
	NetOut   int64   `json:"net_out"`
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

	netStat, err := net.IOCounters(false)
	var netIn, netOut int64
	if err == nil && len(netStat) > 0 {
		netIn = int64(netStat[0].BytesRecv)
		netOut = int64(netStat[0].BytesSent)
	}

	return &MetricPayload{
		CPU:    cpuPercents[0],
		Memory: vmStat.UsedPercent,
		Disk:   diskStat.UsedPercent,
		NetIn:  netIn,
		NetOut: netOut,
	}, nil
}
```

**Step 4: sender.go 구현**

```go
// agent/sender.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func SendMetrics(serverURL string, payload *MetricPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(serverURL+"/api/metrics", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("서버 응답 오류: %d", resp.StatusCode)
	}
	return nil
}
```

**Step 5: main.go 구현**

```go
// agent/main.go
package main

import (
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	ServerID  string `yaml:"server_id"`
	ServerURL string `yaml:"server_url"`
	Host      string `yaml:"host"`
	Name      string `yaml:"name"`
	Interval  int    `yaml:"interval_seconds"`
}

func main() {
	configPath := "agent.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("설정 파일 읽기 실패: %v", err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("설정 파싱 실패: %v", err)
	}

	if cfg.Interval <= 0 {
		cfg.Interval = 5
	}

	log.Printf("AlwaysOn Agent 시작 - 서버: %s, 수집 간격: %ds", cfg.ServerURL, cfg.Interval)

	for {
		payload, err := CollectMetrics()
		if err != nil {
			log.Printf("메트릭 수집 오류: %v", err)
		} else {
			payload.ServerID = cfg.ServerID
			payload.Host = cfg.Host
			payload.Name = cfg.Name
			if err := SendMetrics(cfg.ServerURL, payload); err != nil {
				log.Printf("메트릭 전송 오류: %v", err)
			}
		}
		time.Sleep(time.Duration(cfg.Interval) * time.Second)
	}
}
```

**Step 6: agent.yaml.example 작성**

```yaml
# agent/agent.yaml.example
server_id: ""          # 비워두면 서버에서 host 기준으로 자동 생성
server_url: "http://알림서버IP:8080"
host: "web-server-01"  # 이 서버의 고유 식별자
name: "Web Server 01"  # 대시보드에 표시될 이름
interval_seconds: 5
```

**Step 7: 테스트 통과 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/agent
go test ./... -v
```
Expected: PASS

**Step 8: 에이전트 빌드 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/agent
go build -o alwayson-agent .
ls -la alwayson-agent
```
Expected: 바이너리 파일 생성됨

**Step 9: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add agent/
git commit -m "feat: Go 에이전트 구현 (gopsutil 메트릭 수집 + HTTP Push)"
```

---

## Task 6: Collector API (메트릭 수신)

**Files:**
- Create: `server/api/router.go`
- Create: `server/api/metric_handler.go`
- Create: `server/service/metric_service.go`
- Create: `server/service/metric_service_test.go`
- Create: `server/main.go`

**Step 1: MetricService 테스트 작성**

```go
// server/service/metric_service_test.go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/alwayson/server/model"
	"github.com/alwayson/server/service"
)

// Mock ServerRepo
type mockServerRepo struct{ mock.Mock }
func (m *mockServerRepo) Upsert(ctx context.Context, host, name string) (*model.Server, error) {
	args := m.Called(ctx, host, name)
	return args.Get(0).(*model.Server), args.Error(1)
}
func (m *mockServerRepo) UpdateLastSeen(ctx context.Context, id string, t time.Time) error {
	return m.Called(ctx, id, t).Error(0)
}

// Mock MetricRepo
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
}
```

**Step 2: 테스트 실패 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go test ./service/... -v
```
Expected: FAIL

**Step 3: MetricService 인터페이스 및 구현**

```go
// server/service/metric_service.go
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
		go s.alertProcessor.Check(ctx, server, metric)
	}

	return metric, nil
}
```

**Step 4: 테스트 통과 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go test ./service/... -v
```
Expected: PASS

**Step 5: Metric 핸들러 구현**

```go
// server/api/metric_handler.go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/alwayson/server/service"
)

type MetricHandler struct {
	svc *service.MetricService
}

func NewMetricHandler(svc *service.MetricService) *MetricHandler {
	return &MetricHandler{svc: svc}
}

type metricRequest struct {
	Host   string  `json:"host" binding:"required"`
	Name   string  `json:"name" binding:"required"`
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	Disk   float64 `json:"disk"`
	NetIn  int64   `json:"net_in"`
	NetOut int64   `json:"net_out"`
}

func (h *MetricHandler) Receive(c *gin.Context) {
	var req metricRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := h.svc.Process(c.Request.Context(), &service.MetricInput{
		Host: req.Host, Name: req.Name,
		CPU: req.CPU, Memory: req.Memory, Disk: req.Disk,
		NetIn: req.NetIn, NetOut: req.NetOut,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

**Step 6: Router 구현**

```go
// server/api/router.go
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/alwayson/server/repository"
)

type Handlers struct {
	Metric    *MetricHandler
	ServerH   *ServerHandler
	AlertH    *AlertHandler
}

func NewRouter(h *Handlers) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	api := r.Group("/api")
	{
		api.POST("/metrics", h.Metric.Receive)
		api.GET("/servers", h.ServerH.List)
		api.GET("/servers/:id", h.ServerH.Get)
		api.GET("/servers/:id/metrics", h.ServerH.GetMetrics)
		api.PUT("/servers/:id/thresholds", h.ServerH.UpdateThresholds)
		api.GET("/alerts", h.AlertH.List)
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
```

**Step 7: main.go 구현**

```go
// server/main.go
package main

import (
	"log"

	"github.com/alwayson/server/api"
	"github.com/alwayson/server/config"
	"github.com/alwayson/server/repository"
	"github.com/alwayson/server/service"
	"github.com/alwayson/server/worker"
)

func main() {
	cfg := config.Load()
	pool := repository.NewPool(cfg)
	defer pool.Close()

	serverRepo := repository.NewServerRepo(pool)
	metricRepo := repository.NewMetricRepo(pool)
	alertRepo := repository.NewAlertRepo(pool)
	thresholdRepo := repository.NewThresholdRepo(pool)

	notifier := service.NewSlackNotifier(cfg.SlackWebhookURL)
	alertWorker := worker.NewAlertWorker(serverRepo, alertRepo, thresholdRepo, notifier)

	metricSvc := service.NewMetricService(serverRepo, metricRepo, alertWorker)

	handlers := &api.Handlers{
		Metric:  api.NewMetricHandler(metricSvc),
		ServerH: api.NewServerHandler(serverRepo, metricRepo, thresholdRepo),
		AlertH:  api.NewAlertHandler(alertRepo),
	}

	r := api.NewRouter(handlers)
	log.Printf("AlwaysOn Server 시작 - :%s", cfg.ServerPort)
	r.Run(":" + cfg.ServerPort)
}
```

**Step 8: 빌드 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go build ./...
```
Expected: 빌드 성공 (모든 의존성 존재해야 함)

> NOTE: Task 7,8 구현 전까지는 build 에러가 날 수 있음. `worker.NewAlertWorker`, `service.NewSlackNotifier`, `api.NewServerHandler`, `api.NewAlertHandler` 를 placeholder로 먼저 생성해두거나 Task 7 완료 후 빌드.

**Step 9: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add server/
git commit -m "feat: Collector API (POST /api/metrics) 및 MetricService 구현"
```

---

## Task 7: Slack Notifier 서비스

**Files:**
- Create: `server/service/notifier.go`
- Create: `server/service/notifier_test.go`

**Step 1: Notifier 테스트 작성**

```go
// server/service/notifier_test.go
package service_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/alwayson/server/service"
)

func TestSlackNotifier_Send(t *testing.T) {
	received := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		received = string(buf[:n])
		w.WriteHeader(200)
	}))
	defer ts.Close()

	n := service.NewSlackNotifier(ts.URL)
	err := n.Send("🚨 테스트 알림 메시지")
	assert.NoError(t, err)
	assert.Contains(t, received, "테스트 알림 메시지")
}
```

**Step 2: 테스트 실패 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go test ./service/... -run TestSlackNotifier -v
```
Expected: FAIL

**Step 3: Notifier 구현**

```go
// server/service/notifier.go
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier interface {
	Send(message string) error
}

type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (n *SlackNotifier) Send(message string) error {
	if n.webhookURL == "" {
		fmt.Printf("[Slack 비활성화] %s\n", message)
		return nil
	}

	payload := map[string]string{"text": message}
	data, _ := json.Marshal(payload)

	resp, err := n.client.Post(n.webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Slack 응답 오류: %d", resp.StatusCode)
	}
	return nil
}
```

**Step 4: 테스트 통과 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go test ./service/... -run TestSlackNotifier -v
```
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add server/service/notifier.go server/service/notifier_test.go
git commit -m "feat: Slack Notifier 서비스 구현"
```

---

## Task 8: Alert Worker (장애 감지 + 알림)

**Files:**
- Create: `server/worker/alert_worker.go`
- Create: `server/worker/alert_worker_test.go`

**Step 1: AlertWorker 테스트 작성**

```go
// server/worker/alert_worker_test.go
package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/alwayson/server/model"
	"github.com/alwayson/server/worker"
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

type mockServerRepo struct{ mock.Mock }
func (m *mockServerRepo) UpdateStatus(ctx context.Context, id string, s model.ServerStatus) error {
	return m.Called(ctx, id, s).Error(0)
}

type mockNotifier struct{ mock.Mock }
func (m *mockNotifier) Send(msg string) error {
	return m.Called(msg).Error(0)
}

func TestAlertWorker_Check_TriggersWarning(t *testing.T) {
	alertRepo := &mockAlertRepo{}
	thresholdRepo := &mockThresholdRepo{}
	serverRepo := &mockServerRepo{}
	notifier := &mockNotifier{}

	thresholdRepo.On("Get", mock.Anything, "server-1").Return(&model.Threshold{
		ServerID: "server-1",
		CPUWarning: 75, CPUCritical: 90,
		MemWarning: 80, MemCritical: 95,
		DiskWarning: 80, DiskCritical: 90,
	}, nil)
	alertRepo.On("Insert", mock.Anything, mock.Anything).Return(nil)
	serverRepo.On("UpdateStatus", mock.Anything, "server-1", model.StatusWarning).Return(nil)
	notifier.On("Send", mock.Anything).Return(nil)

	w := worker.NewAlertWorker(serverRepo, alertRepo, thresholdRepo, notifier)
	server := &model.Server{ID: "server-1", Name: "Test Server", Status: model.StatusNormal}
	metric := &model.Metric{
		Time: time.Now(), ServerID: "server-1",
		CPU: 80, Memory: 50, Disk: 50,
	}

	w.Check(context.Background(), server, metric)

	notifier.AssertCalled(t, "Send", mock.MatchedBy(func(msg string) bool {
		return len(msg) > 0
	}))
	serverRepo.AssertCalled(t, "UpdateStatus", mock.Anything, "server-1", model.StatusWarning)
}
```

**Step 2: 테스트 실패 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go test ./worker/... -v
```
Expected: FAIL

**Step 3: AlertWorker 구현**

```go
// server/worker/alert_worker.go
package worker

import (
	"context"
	"fmt"
	"sync"
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
	mu            sync.Mutex
	lastAlerted   map[string]time.Time // key: "serverID:level:metric"
}

func NewAlertWorker(sr AlertServerRepo, ar AlertAlertRepo, tr AlertThresholdRepo, n service.Notifier) *AlertWorker {
	return &AlertWorker{
		serverRepo:    sr,
		alertRepo:     ar,
		thresholdRepo: tr,
		notifier:      n,
		lastAlerted:   make(map[string]time.Time),
	}
}

func (w *AlertWorker) Check(ctx context.Context, server *model.Server, m *model.Metric) {
	th, err := w.thresholdRepo.Get(ctx, server.ID)
	if err != nil {
		return
	}

	type check struct {
		metric string
		value  float64
		warn   float64
		crit   float64
	}

	checks := []check{
		{"cpu", m.CPU, th.CPUWarning, th.CPUCritical},
		{"memory", m.Memory, th.MemWarning, th.MemCritical},
		{"disk", m.Disk, th.DiskWarning, th.DiskCritical},
	}

	highestLevel := model.StatusNormal
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

		if w.comparePriority(status) > w.comparePriority(highestLevel) {
			highestLevel = status
		}

		msg := fmt.Sprintf("🚨 [AlwaysOn] *%s* - %s %s\n서버: %s\n현재값: %.1f%%\n임계값: %.1f%%",
			string(level), server.Name, c.metric, server.Host, c.value,
			map[model.AlertLevel]float64{model.LevelWarning: c.warn, model.LevelCritical: c.crit}[level],
		)

		if w.canAlert(server.ID, string(level), c.metric) {
			_ = w.alertRepo.Insert(ctx, &model.Alert{
				ServerID: server.ID,
				Level:    level,
				Metric:   c.metric,
				Value:    c.value,
				Message:  msg,
			})
			_ = w.notifier.Send(msg)
		}
	}

	if highestLevel == model.StatusNormal && server.Status != model.StatusNormal {
		_ = w.alertRepo.ResolveByServer(ctx, server.ID)
		msg := fmt.Sprintf("✅ [AlwaysOn] *%s* 서버가 정상 상태로 복구되었습니다.", server.Name)
		_ = w.notifier.Send(msg)
	}

	if highestLevel != server.Status {
		_ = w.serverRepo.UpdateStatus(ctx, server.ID, highestLevel)
	}
}

func (w *AlertWorker) canAlert(serverID, level, metric string) bool {
	key := serverID + ":" + level + ":" + metric
	w.mu.Lock()
	defer w.mu.Unlock()
	last, exists := w.lastAlerted[key]
	if !exists || time.Since(last) >= cooldownDuration {
		w.lastAlerted[key] = time.Now()
		return true
	}
	return false
}

func (w *AlertWorker) comparePriority(s model.ServerStatus) int {
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
```

**Step 4: 테스트 통과 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go test ./worker/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add server/worker/
git commit -m "feat: Alert Worker 구현 (3단계 장애 감지 + 쿨다운 + 복구 알림)"
```

---

## Task 9: 대시보드 REST API 핸들러

**Files:**
- Create: `server/api/server_handler.go`
- Create: `server/api/alert_handler.go`

**Step 1: Server 핸들러 구현**

```go
// server/api/server_handler.go
package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/alwayson/server/model"
	"github.com/alwayson/server/repository"
)

type ServerHandlerServerRepo interface {
	GetAll(ctx context.Context) ([]model.Server, error)
}

type ServerHandlerMetricRepo interface {
	GetLatest(ctx context.Context, serverID string) (*model.Metric, error)
	GetRecent(ctx context.Context, serverID string, since time.Time) ([]model.Metric, error)
}

type ServerHandlerThresholdRepo interface {
	Get(ctx context.Context, serverID string) (*model.Threshold, error)
	Upsert(ctx context.Context, t *model.Threshold) error
}

type ServerHandler struct {
	serverRepo    ServerHandlerServerRepo
	metricRepo    ServerHandlerMetricRepo
	thresholdRepo ServerHandlerThresholdRepo
}

func NewServerHandler(sr *repository.ServerRepo, mr *repository.MetricRepo, tr *repository.ThresholdRepo) *ServerHandler {
	return &ServerHandler{serverRepo: sr, metricRepo: mr, thresholdRepo: tr}
}

func (h *ServerHandler) List(c *gin.Context) {
	servers, err := h.serverRepo.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if servers == nil {
		servers = []model.Server{}
	}
	c.JSON(http.StatusOK, servers)
}

func (h *ServerHandler) Get(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	latest, _ := h.metricRepo.GetLatest(ctx, id)
	threshold, _ := h.thresholdRepo.Get(ctx, id)

	c.JSON(http.StatusOK, gin.H{
		"id":        id,
		"latest":    latest,
		"threshold": threshold,
	})
}

func (h *ServerHandler) GetMetrics(c *gin.Context) {
	id := c.Param("id")
	hours := 1
	if h := c.Query("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil {
			hours = v
		}
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	metrics, err := h.metricRepo.GetRecent(c.Request.Context(), id, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if metrics == nil {
		metrics = []model.Metric{}
	}
	c.JSON(http.StatusOK, metrics)
}

func (h *ServerHandler) UpdateThresholds(c *gin.Context) {
	id := c.Param("id")
	var t model.Threshold
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t.ServerID = id
	if err := h.thresholdRepo.Upsert(c.Request.Context(), &t); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}
```

**Step 2: Alert 핸들러 구현**

```go
// server/api/alert_handler.go
package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/alwayson/server/model"
	"github.com/alwayson/server/repository"
)

type AlertHandlerRepo interface {
	GetAll(ctx context.Context, limit int) ([]model.Alert, error)
}

type AlertHandler struct {
	alertRepo AlertHandlerRepo
}

func NewAlertHandler(ar *repository.AlertRepo) *AlertHandler {
	return &AlertHandler{alertRepo: ar}
}

func (h *AlertHandler) List(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}

	alerts, err := h.alertRepo.GetAll(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if alerts == nil {
		alerts = []model.Alert{}
	}
	c.JSON(http.StatusOK, alerts)
}
```

**Step 3: 서버 전체 빌드 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go build ./...
```
Expected: 빌드 성공

**Step 4: 서버 실행 및 엔드포인트 테스트**

```bash
# 터미널 1: 서버 실행
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go run main.go

# 터미널 2: 메트릭 전송 테스트
curl -X POST http://localhost:8080/api/metrics \
  -H "Content-Type: application/json" \
  -d '{"host":"test-host","name":"Test Server","cpu":50.5,"memory":60.0,"disk":40.0,"net_in":1000,"net_out":500}'

# 서버 목록 확인
curl http://localhost:8080/api/servers
```
Expected: `{"status":"ok"}` 및 서버 목록 JSON

**Step 5: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add server/api/
git commit -m "feat: 대시보드용 REST API 핸들러 구현 (servers, alerts)"
```

---

## Task 10: React 프론트엔드 초기 설정

**Files:**
- Create: `frontend/` (Vite + React 프로젝트)

**Step 1: Vite React 프로젝트 생성**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
npm create vite@latest frontend -- --template react
cd frontend
npm install
npm install recharts axios react-router-dom
npm install -D tailwindcss postcss autoprefixer
npx tailwindcss init -p
```

**Step 2: Tailwind CSS 설정**

```js
// frontend/tailwind.config.js (생성됨, 수정)
/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: { extend: {} },
  plugins: [],
}
```

```css
/* frontend/src/index.css - 내용 교체 */
@tailwind base;
@tailwind components;
@tailwind utilities;
```

**Step 3: API 클라이언트 작성**

```js
// frontend/src/api/client.js
import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080/api',
})

export const getServers = () => api.get('/servers')
export const getServer = (id) => api.get(`/servers/${id}`)
export const getServerMetrics = (id, hours = 1) =>
  api.get(`/servers/${id}/metrics?hours=${hours}`)
export const getAlerts = (limit = 100) =>
  api.get(`/alerts?limit=${limit}`)
export const updateThresholds = (id, data) =>
  api.put(`/servers/${id}/thresholds`, data)
```

**Step 4: .env.local 작성**

```bash
# frontend/.env.local
VITE_API_URL=http://localhost:8080/api
```

**Step 5: App.jsx 라우터 설정**

```jsx
// frontend/src/App.jsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import ServerList from './pages/ServerList'
import ServerDetail from './pages/ServerDetail'
import AlertHistory from './pages/AlertHistory'
import Navbar from './components/Navbar'

export default function App() {
  return (
    <BrowserRouter>
      <div className="min-h-screen bg-gray-50">
        <Navbar />
        <main className="container mx-auto px-4 py-6">
          <Routes>
            <Route path="/" element={<ServerList />} />
            <Route path="/servers/:id" element={<ServerDetail />} />
            <Route path="/alerts" element={<AlertHistory />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  )
}
```

**Step 6: Navbar 컴포넌트**

```jsx
// frontend/src/components/Navbar.jsx
import { Link } from 'react-router-dom'

export default function Navbar() {
  return (
    <nav className="bg-gray-900 text-white px-6 py-4 flex items-center gap-6">
      <Link to="/" className="text-xl font-bold text-green-400">AlwaysOn</Link>
      <Link to="/" className="text-gray-300 hover:text-white">서버 목록</Link>
      <Link to="/alerts" className="text-gray-300 hover:text-white">알림 히스토리</Link>
    </nav>
  )
}
```

**Step 7: 개발 서버 실행 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/frontend
npm run dev
```
Expected: http://localhost:5173 에서 빈 화면 정상 로딩

**Step 8: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add frontend/
git commit -m "feat: React 프론트엔드 초기 설정 (Vite + Tailwind + React Router)"
```

---

## Task 11: 서버 목록 페이지

**Files:**
- Create: `frontend/src/pages/ServerList.jsx`
- Create: `frontend/src/components/ServerCard.jsx`
- Create: `frontend/src/components/StatusBadge.jsx`

**Step 1: StatusBadge 컴포넌트**

```jsx
// frontend/src/components/StatusBadge.jsx
const STATUS_STYLES = {
  normal:   'bg-green-100 text-green-800',
  warning:  'bg-yellow-100 text-yellow-800',
  critical: 'bg-orange-100 text-orange-800',
  down:     'bg-red-100 text-red-800',
}

const STATUS_LABELS = {
  normal: '정상', warning: '경고', critical: '위험', down: '다운',
}

export default function StatusBadge({ status }) {
  const style = STATUS_STYLES[status] || STATUS_STYLES.normal
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-semibold ${style}`}>
      {STATUS_LABELS[status] || status}
    </span>
  )
}
```

**Step 2: ServerCard 컴포넌트**

```jsx
// frontend/src/components/ServerCard.jsx
import { Link } from 'react-router-dom'
import StatusBadge from './StatusBadge'

const STATUS_BORDER = {
  normal: 'border-green-400',
  warning: 'border-yellow-400',
  critical: 'border-orange-400',
  down: 'border-red-400',
}

export default function ServerCard({ server }) {
  const borderColor = STATUS_BORDER[server.status] || 'border-gray-200'
  const lastSeen = server.last_seen
    ? new Date(server.last_seen).toLocaleString('ko-KR')
    : '없음'

  return (
    <Link to={`/servers/${server.id}`}>
      <div className={`bg-white rounded-lg border-2 ${borderColor} p-4 hover:shadow-md transition-shadow`}>
        <div className="flex justify-between items-start mb-2">
          <h3 className="font-semibold text-gray-800">{server.name}</h3>
          <StatusBadge status={server.status} />
        </div>
        <p className="text-sm text-gray-500">{server.host}</p>
        <p className="text-xs text-gray-400 mt-2">마지막 수신: {lastSeen}</p>
      </div>
    </Link>
  )
}
```

**Step 3: ServerList 페이지**

```jsx
// frontend/src/pages/ServerList.jsx
import { useState, useEffect } from 'react'
import { getServers } from '../api/client'
import ServerCard from '../components/ServerCard'

export default function ServerList() {
  const [servers, setServers] = useState([])
  const [loading, setLoading] = useState(true)

  const fetchServers = async () => {
    try {
      const { data } = await getServers()
      setServers(data)
    } catch (e) {
      console.error(e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchServers()
    const interval = setInterval(fetchServers, 10000) // 10초마다 갱신
    return () => clearInterval(interval)
  }, [])

  const counts = {
    normal: servers.filter(s => s.status === 'normal').length,
    warning: servers.filter(s => s.status === 'warning').length,
    critical: servers.filter(s => s.status === 'critical').length,
    down: servers.filter(s => s.status === 'down').length,
  }

  if (loading) return <div className="text-center py-12 text-gray-500">로딩 중...</div>

  return (
    <div>
      <div className="flex gap-4 mb-6">
        <div className="bg-green-50 border border-green-200 rounded-lg px-4 py-2 text-sm">
          정상 <span className="font-bold text-green-700">{counts.normal}</span>
        </div>
        <div className="bg-yellow-50 border border-yellow-200 rounded-lg px-4 py-2 text-sm">
          경고 <span className="font-bold text-yellow-700">{counts.warning}</span>
        </div>
        <div className="bg-orange-50 border border-orange-200 rounded-lg px-4 py-2 text-sm">
          위험 <span className="font-bold text-orange-700">{counts.critical}</span>
        </div>
        <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-2 text-sm">
          다운 <span className="font-bold text-red-700">{counts.down}</span>
        </div>
      </div>

      {servers.length === 0 ? (
        <div className="text-center py-12 text-gray-400">
          <p>등록된 서버가 없습니다.</p>
          <p className="text-sm mt-2">에이전트를 설치하면 자동으로 등록됩니다.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {servers.map(server => (
            <ServerCard key={server.id} server={server} />
          ))}
        </div>
      )}
    </div>
  )
}
```

**Step 4: 화면 확인**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/frontend
npm run dev
```
브라우저에서 http://localhost:5173 접속 후 서버 카드 그리드 확인

**Step 5: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add frontend/src/
git commit -m "feat: 서버 목록 페이지 구현 (상태별 색상 카드 + 자동 갱신)"
```

---

## Task 12: 서버 상세 페이지 (메트릭 차트)

**Files:**
- Create: `frontend/src/pages/ServerDetail.jsx`
- Create: `frontend/src/components/MetricChart.jsx`

**Step 1: MetricChart 컴포넌트**

```jsx
// frontend/src/components/MetricChart.jsx
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend
} from 'recharts'

function formatTime(timeStr) {
  return new Date(timeStr).toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit' })
}

export default function MetricChart({ data, title }) {
  const chartData = data.map(m => ({
    time: formatTime(m.time),
    CPU: parseFloat(m.cpu.toFixed(1)),
    메모리: parseFloat(m.memory.toFixed(1)),
    디스크: parseFloat(m.disk.toFixed(1)),
  }))

  return (
    <div className="bg-white rounded-lg border p-4">
      <h3 className="text-sm font-semibold text-gray-600 mb-3">{title}</h3>
      <ResponsiveContainer width="100%" height={200}>
        <LineChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
          <XAxis dataKey="time" tick={{ fontSize: 11 }} />
          <YAxis domain={[0, 100]} tick={{ fontSize: 11 }} unit="%" />
          <Tooltip formatter={(v) => `${v}%`} />
          <Legend />
          <Line type="monotone" dataKey="CPU" stroke="#3b82f6" dot={false} strokeWidth={2} />
          <Line type="monotone" dataKey="메모리" stroke="#10b981" dot={false} strokeWidth={2} />
          <Line type="monotone" dataKey="디스크" stroke="#f59e0b" dot={false} strokeWidth={2} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
```

**Step 2: ServerDetail 페이지**

```jsx
// frontend/src/pages/ServerDetail.jsx
import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { getServers, getServerMetrics, getAlerts } from '../api/client'
import MetricChart from '../components/MetricChart'
import StatusBadge from '../components/StatusBadge'

const HOURS_OPTIONS = [
  { label: '1시간', value: 1 },
  { label: '6시간', value: 6 },
  { label: '24시간', value: 24 },
]

export default function ServerDetail() {
  const { id } = useParams()
  const [server, setServer] = useState(null)
  const [metrics, setMetrics] = useState([])
  const [alerts, setAlerts] = useState([])
  const [hours, setHours] = useState(1)

  useEffect(() => {
    const fetchAll = async () => {
      const [{ data: servers }, { data: m }, { data: a }] = await Promise.all([
        getServers(),
        getServerMetrics(id, hours),
        getAlerts(50),
      ])
      setServer(servers.find(s => s.id === id) || null)
      setMetrics(m)
      setAlerts(a.filter(al => al.server_id === id))
    }
    fetchAll()
    const interval = setInterval(fetchAll, 10000)
    return () => clearInterval(interval)
  }, [id, hours])

  if (!server) return <div className="text-center py-12 text-gray-400">로딩 중...</div>

  const latest = metrics[metrics.length - 1]

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <Link to="/" className="text-gray-400 hover:text-gray-600">← 목록으로</Link>
        <h1 className="text-xl font-bold text-gray-800">{server.name}</h1>
        <StatusBadge status={server.status} />
        <span className="text-sm text-gray-400">{server.host}</span>
      </div>

      {latest && (
        <div className="grid grid-cols-3 gap-4 mb-6">
          {[
            { label: 'CPU', value: latest.cpu },
            { label: '메모리', value: latest.memory },
            { label: '디스크', value: latest.disk },
          ].map(({ label, value }) => (
            <div key={label} className="bg-white rounded-lg border p-4 text-center">
              <p className="text-sm text-gray-500">{label}</p>
              <p className="text-2xl font-bold text-gray-800">{value?.toFixed(1)}%</p>
            </div>
          ))}
        </div>
      )}

      <div className="flex gap-2 mb-4">
        {HOURS_OPTIONS.map(opt => (
          <button
            key={opt.value}
            onClick={() => setHours(opt.value)}
            className={`px-3 py-1 rounded text-sm ${
              hours === opt.value
                ? 'bg-blue-600 text-white'
                : 'bg-white border text-gray-600 hover:bg-gray-50'
            }`}
          >
            {opt.label}
          </button>
        ))}
      </div>

      <MetricChart data={metrics} title="메트릭 추이" />

      {alerts.length > 0 && (
        <div className="mt-6">
          <h2 className="text-lg font-semibold text-gray-700 mb-3">알림 히스토리</h2>
          <div className="space-y-2">
            {alerts.slice(0, 10).map(alert => (
              <div key={alert.id} className="bg-white border rounded-lg p-3 text-sm">
                <div className="flex justify-between">
                  <span className={`font-semibold ${
                    alert.level === 'critical' ? 'text-orange-600' :
                    alert.level === 'warning' ? 'text-yellow-600' : 'text-red-600'
                  }`}>{alert.level.toUpperCase()} - {alert.metric}</span>
                  <span className="text-gray-400">
                    {new Date(alert.created_at).toLocaleString('ko-KR')}
                  </span>
                </div>
                <p className="text-gray-600 mt-1">{alert.message}</p>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
```

**Step 3: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add frontend/src/
git commit -m "feat: 서버 상세 페이지 구현 (recharts 메트릭 차트 + 조회 범위 선택)"
```

---

## Task 13: 알림 히스토리 페이지

**Files:**
- Create: `frontend/src/pages/AlertHistory.jsx`

**Step 1: AlertHistory 페이지 구현**

```jsx
// frontend/src/pages/AlertHistory.jsx
import { useState, useEffect } from 'react'
import { getAlerts, getServers } from '../api/client'

const LEVEL_STYLES = {
  warning:  { badge: 'bg-yellow-100 text-yellow-800', label: '경고' },
  critical: { badge: 'bg-orange-100 text-orange-800', label: '위험' },
  down:     { badge: 'bg-red-100 text-red-800', label: '다운' },
}

export default function AlertHistory() {
  const [alerts, setAlerts] = useState([])
  const [servers, setServers] = useState({})
  const [filter, setFilter] = useState('all')

  useEffect(() => {
    const fetchAll = async () => {
      const [{ data: alertData }, { data: serverData }] = await Promise.all([
        getAlerts(200),
        getServers(),
      ])
      setAlerts(alertData)
      setServers(Object.fromEntries(serverData.map(s => [s.id, s])))
    }
    fetchAll()
  }, [])

  const filtered = filter === 'all'
    ? alerts
    : alerts.filter(a => a.level === filter)

  return (
    <div>
      <h1 className="text-xl font-bold text-gray-800 mb-4">알림 히스토리</h1>

      <div className="flex gap-2 mb-4">
        {['all', 'warning', 'critical', 'down'].map(level => (
          <button
            key={level}
            onClick={() => setFilter(level)}
            className={`px-3 py-1 rounded text-sm ${
              filter === level
                ? 'bg-blue-600 text-white'
                : 'bg-white border text-gray-600 hover:bg-gray-50'
            }`}
          >
            {level === 'all' ? '전체' : LEVEL_STYLES[level]?.label || level}
          </button>
        ))}
      </div>

      {filtered.length === 0 ? (
        <div className="text-center py-12 text-gray-400">알림이 없습니다.</div>
      ) : (
        <div className="space-y-2">
          {filtered.map(alert => {
            const serverName = servers[alert.server_id]?.name || alert.server_id
            const style = LEVEL_STYLES[alert.level] || { badge: 'bg-gray-100 text-gray-800', label: alert.level }
            return (
              <div key={alert.id} className="bg-white border rounded-lg p-4">
                <div className="flex justify-between items-start">
                  <div className="flex items-center gap-2">
                    <span className={`px-2 py-1 rounded-full text-xs font-semibold ${style.badge}`}>
                      {style.label}
                    </span>
                    <span className="font-medium text-gray-800">{serverName}</span>
                    <span className="text-sm text-gray-500">{alert.metric}</span>
                    <span className="text-sm font-semibold text-gray-700">
                      {alert.value?.toFixed(1)}%
                    </span>
                  </div>
                  <div className="text-right text-xs text-gray-400">
                    <div>{new Date(alert.created_at).toLocaleString('ko-KR')}</div>
                    {alert.resolved_at && (
                      <div className="text-green-600">
                        복구: {new Date(alert.resolved_at).toLocaleString('ko-KR')}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
```

**Step 2: Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add frontend/src/pages/AlertHistory.jsx
git commit -m "feat: 알림 히스토리 페이지 구현 (레벨별 필터)"
```

---

## Task 14: 통합 확인 및 README

**Files:**
- Create: `README.md`

**Step 1: 전체 통합 동작 확인**

```bash
# 1. DB 실행
cd /Users/kimhanseop/Desktop/AlwaysOn
docker-compose up -d

# 2. DB 마이그레이션
cd server
go run cmd/migrate/main.go up

# 3. 백엔드 서버 실행 (새 터미널)
SLACK_WEBHOOK_URL="" go run main.go

# 4. 에이전트 실행 테스트 (새 터미널)
cd ../agent
cp agent.yaml.example agent.yaml
# agent.yaml에서 host/name 수정 후:
go run . agent.yaml

# 5. 프론트엔드 실행 (새 터미널)
cd ../frontend
npm run dev
```

**Step 2: 전체 Go 테스트 실행**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn/server
go test ./... -v
```
Expected: 모든 단위 테스트 PASS (repository 통합 테스트는 SKIP)

**Step 3: README 작성**

```markdown
# AlwaysOn - 실시간 장애 모니터링 플랫폼

온프레미스 서버 상태를 실시간으로 수집하고 장애 발생 시 Slack으로 알림을 보내는 모니터링 플랫폼.

## 빠른 시작

### 1. DB 실행
\`\`\`bash
docker-compose up -d
\`\`\`

### 2. 마이그레이션
\`\`\`bash
cd server && go run cmd/migrate/main.go up
\`\`\`

### 3. 백엔드 서버
\`\`\`bash
cd server
cp .env.example .env  # SLACK_WEBHOOK_URL 설정
go run main.go
\`\`\`

### 4. 에이전트 설치 (모니터링 대상 서버마다)
\`\`\`bash
cd agent
go build -o alwayson-agent .
cp agent.yaml.example agent.yaml
# agent.yaml 수정: server_url, host, name 설정
./alwayson-agent agent.yaml
\`\`\`

### 5. 대시보드
\`\`\`bash
cd frontend && npm install && npm run dev
\`\`\`
http://localhost:5173 접속

## 장애 레벨 기본 임계값

| 메트릭 | Warning | Critical |
|--------|---------|----------|
| CPU | 75% | 90% |
| 메모리 | 80% | 95% |
| 디스크 | 80% | 90% |
| 미응답 | - | Down (30s) |

서버별 커스텀 임계값: `PUT /api/servers/:id/thresholds`
```

**Step 4: 최종 Commit**

```bash
cd /Users/kimhanseop/Desktop/AlwaysOn
git add README.md
git commit -m "docs: README 작성 - 빠른 시작 가이드"
```

---

## 구현 순서 요약

| Task | 내용 | 예상 결과물 |
|------|------|------------|
| 1 | 프로젝트 초기 설정 | docker-compose, go.mod |
| 2 | DB 마이그레이션 | TimescaleDB 스키마 |
| 3 | 백엔드 기초 | Config, 모델, 마이그레이션 실행기 |
| 4 | Repository 레이어 | DB CRUD |
| 5 | Go 에이전트 | 메트릭 수집 바이너리 |
| 6 | Collector API | POST /api/metrics |
| 7 | Slack Notifier | 알림 전송 서비스 |
| 8 | Alert Worker | 3단계 장애 감지 |
| 9 | 대시보드 API | GET /api/servers, /api/alerts |
| 10 | React 초기 설정 | Vite + Tailwind |
| 11 | 서버 목록 페이지 | 상태 카드 그리드 |
| 12 | 서버 상세 페이지 | recharts 메트릭 차트 |
| 13 | 알림 히스토리 | 필터링 테이블 |
| 14 | 통합 확인 + README | 완성 |
