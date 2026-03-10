# Security & Retention Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** API 인증(Agent API Key)과 TimescaleDB 메트릭 자동 삭제 정책을 추가한다.

**Architecture:** 에이전트는 `X-Agent-Key` 헤더로 키를 전송, 서버는 Gin 미들웨어로 `POST /api/metrics`만 검증한다. TimescaleDB retention policy는 마이그레이션으로 관리하며 30일 지난 메트릭을 자동 삭제한다.

**Tech Stack:** Go 1.22+, Gin v1, TimescaleDB 2.x, pgx/v5

---

### Task 1: TimescaleDB 보존 정책 마이그레이션

**Files:**
- Create: `migrations/000004_retention_policy.up.sql`
- Create: `migrations/000004_retention_policy.down.sql`

**Step 1: up 마이그레이션 작성**

`migrations/000004_retention_policy.up.sql`:
```sql
SELECT add_retention_policy('metrics', INTERVAL '30 days');
```

**Step 2: down 마이그레이션 작성**

`migrations/000004_retention_policy.down.sql`:
```sql
SELECT remove_retention_policy('metrics');
```

**Step 3: 마이그레이션 실행**

```bash
cd server && go run cmd/migrate/main.go up
```

Expected: 오류 없음 (`migrating... done`)

**Step 4: Commit**

```bash
git add migrations/
git commit -m "feat(db): metrics 30일 보존 정책 추가"
```

---

### Task 2: 서버 config에 AgentAPIKey 추가

**Files:**
- Modify: `server/config/config.go`
- Modify: `server/.env.example`

**Step 1: Config 구조체에 AgentAPIKey 추가**

`server/config/config.go`의 `Config` 구조체와 `Load()` 함수를 아래로 교체:
```go
type Config struct {
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	ServerPort      string
	SlackWebhookURL string
	AgentAPIKey     string
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
		AgentAPIKey:     getEnv("AGENT_API_KEY", ""),
	}
}
```

**Step 2: .env.example에 항목 추가**

`server/.env.example` 끝에 추가:
```
AGENT_API_KEY=your-secret-key-here
```

**Step 3: 빌드 확인**

```bash
cd server && go build ./...
```

Expected: 오류 없음

**Step 4: Commit**

```bash
git add server/config/config.go server/.env.example
git commit -m "feat(config): AgentAPIKey 환경변수 추가"
```

---

### Task 3: Gin 인증 미들웨어 작성

**Files:**
- Create: `server/api/middleware.go`
- Create: `server/api/middleware_test.go`

**Step 1: 실패하는 테스트 작성**

`server/api/middleware_test.go`:
```go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/alwayson/server/api"
)

func TestAgentAuthMiddleware_NoKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.AgentAuthMiddleware("secret"))
	r.POST("/api/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/metrics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentAuthMiddleware_WrongKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.AgentAuthMiddleware("secret"))
	r.POST("/api/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/metrics", nil)
	req.Header.Set("X-Agent-Key", "wrong")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentAuthMiddleware_CorrectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.AgentAuthMiddleware("secret"))
	r.POST("/api/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/metrics", nil)
	req.Header.Set("X-Agent-Key", "secret")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentAuthMiddleware_EmptyKey_Disabled(t *testing.T) {
	// AGENT_API_KEY가 비어있으면 인증 비활성화
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.AgentAuthMiddleware(""))
	r.POST("/api/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/metrics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
```

**Step 2: 테스트 실패 확인**

```bash
cd server && go test ./api/... -run TestAgentAuthMiddleware -v
```

Expected: FAIL (`AgentAuthMiddleware undefined`)

**Step 3: 미들웨어 구현**

`server/api/middleware.go`:
```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func AgentAuthMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}
		if c.GetHeader("X-Agent-Key") != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
```

**Step 4: 테스트 통과 확인**

```bash
cd server && go test ./api/... -run TestAgentAuthMiddleware -v
```

Expected: 4개 테스트 모두 PASS

**Step 5: Commit**

```bash
git add server/api/middleware.go server/api/middleware_test.go
git commit -m "feat(api): AgentAuthMiddleware 추가"
```

---

### Task 4: router.go에 미들웨어 적용

**Files:**
- Modify: `server/api/router.go`

**Step 1: Handlers 구조체에 APIKey 추가 및 라우터 수정**

`server/api/router.go` 전체 교체:
```go
package api

import "github.com/gin-gonic/gin"

type Handlers struct {
	Metric       *MetricHandler
	ServerH      *ServerHandler
	AlertH       *AlertHandler
	HealthCheckH *HealthCheckHandler
	AgentAPIKey  string
}

func NewRouter(h *Handlers) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	api := r.Group("/api")
	{
		api.POST("/metrics", AgentAuthMiddleware(h.AgentAPIKey), h.Metric.Receive)
		api.GET("/servers", h.ServerH.List)
		api.GET("/servers/:id", h.ServerH.Get)
		api.GET("/servers/:id/metrics", h.ServerH.GetMetrics)
		api.PUT("/servers/:id/thresholds", h.ServerH.UpdateThresholds)
		api.GET("/alerts", h.AlertH.List)
		api.GET("/servers/:id/health-checks", h.HealthCheckH.List)
		api.POST("/servers/:id/health-checks", h.HealthCheckH.Create)
		api.DELETE("/servers/:id/health-checks/:hid", h.HealthCheckH.Delete)
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,X-Agent-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
```

**Step 2: main.go에서 AgentAPIKey 전달 확인**

`server/main.go`에서 `Handlers` 초기화 부분을 찾아 `AgentAPIKey: cfg.AgentAPIKey` 추가:
```go
handlers := &api.Handlers{
    Metric:       api.NewMetricHandler(metricSvc),
    ServerH:      api.NewServerHandler(serverSvc),
    AlertH:       api.NewAlertHandler(alertSvc),
    HealthCheckH: api.NewHealthCheckHandler(healthCheckSvc),
    AgentAPIKey:  cfg.AgentAPIKey,
}
```

**Step 3: 빌드 확인**

```bash
cd server && go build ./...
```

Expected: 오류 없음

**Step 4: 전체 테스트 확인**

```bash
cd server && go test ./...
```

Expected: 모두 PASS

**Step 5: Commit**

```bash
git add server/api/router.go server/main.go
git commit -m "feat(api): POST /api/metrics에 인증 미들웨어 적용"
```

---

### Task 5: 에이전트 - api_key 설정 추가

**Files:**
- Modify: `agent/main.go`
- Modify: `agent/sender.go`
- Modify: `agent/agent.yaml.example`

**Step 1: AgentConfig에 APIKey 필드 추가**

`agent/main.go`의 `AgentConfig` 구조체 수정:
```go
type AgentConfig struct {
	ServerURL string   `yaml:"server_url"`
	Host      string   `yaml:"host"`
	Name      string   `yaml:"name"`
	Interval  int      `yaml:"interval_seconds"`
	Processes []string `yaml:"processes"`
	APIKey    string   `yaml:"api_key"`
}
```

환경변수 지원 추가 (`cfg.ServerURL = os.Getenv(...)` 블록에 추가):
```go
cfg.APIKey = os.Getenv("AGENT_API_KEY")
```

`SendWithRetry` 호출부를 `SendWithRetry(cfg.ServerURL, cfg.APIKey, payload)`로 변경.

**Step 2: sender.go에 APIKey 헤더 추가**

`agent/sender.go` 전체 교체:
```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func SendMetrics(serverURL, apiKey string, payload *MetricPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("POST", serverURL+"/api/metrics", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-Agent-Key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("서버 응답 오류: %d", resp.StatusCode)
	}
	return nil
}

func SendWithRetry(serverURL, apiKey string, payload *MetricPayload) error {
	backoff := time.Second
	const maxAttempts = 5

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := SendMetrics(serverURL, apiKey, payload)
		if err == nil {
			return nil
		}
		if attempt == maxAttempts {
			return fmt.Errorf("메트릭 전송 %d회 실패: %w", maxAttempts, err)
		}
		log.Printf("[재시도 %d/%d] %v — %v 후 재시도", attempt, maxAttempts, err, backoff)
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
	return nil
}
```

**Step 3: agent.yaml.example에 api_key 추가**

`agent/agent.yaml.example`:
```yaml
server_url: "http://모니터링서버IP:8080"
host: "web-server-01"   # 서버 고유 식별자 (중복 불가)
name: "Web Server 01"   # 대시보드 표시명
interval_seconds: 5
api_key: ""             # 서버의 AGENT_API_KEY와 동일하게 설정

# 감시할 프로세스 목록 (없으면 생략)
processes:
  # - nginx
  # - mysql
```

**Step 4: 에이전트 빌드 확인**

```bash
cd agent && go build ./...
```

Expected: 오류 없음

**Step 5: 에이전트 테스트 확인**

```bash
cd agent && go test ./... -v
```

Expected: 모두 PASS

**Step 6: Commit**

```bash
git add agent/main.go agent/sender.go agent/agent.yaml.example
git commit -m "feat(agent): API Key 인증 헤더 추가"
```

---

### Task 6: 최종 검증

**Step 1: 서버 전체 테스트**

```bash
cd server && go test ./...
```

Expected: 모두 PASS

**Step 2: 에이전트 전체 테스트**

```bash
cd agent && go test ./...
```

Expected: PASS

**Step 3: 전체 빌드**

```bash
cd server && go build ./... && cd ../agent && go build ./...
```

Expected: 오류 없음

**Step 4: 태그**

```bash
git tag v0.3.0-security-retention
```
