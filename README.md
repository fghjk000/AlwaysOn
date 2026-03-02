# AlwaysOn

온프레미스 서버의 상태를 실시간으로 수집하고, 장애를 감지하여 Slack으로 알림을 보내는 모니터링 플랫폼.

## 주요 기능

- **에이전트 기반 수집**: 각 서버에 Go 바이너리 에이전트를 설치하여 5초 간격으로 CPU, 메모리, 디스크, 네트워크 메트릭 Push
- **3단계 장애 감지**: Warning → Critical → Down 단계별 감지 및 Slack 알림
- **자동 복구 감지**: 상태 정상화 시 복구 알림 자동 전송 (중복 알림 10분 쿨다운)
- **실시간 대시보드**: 서버 상태 카드, 시계열 메트릭 차트, 알림 히스토리

## 아키텍처

```
[Agent (Go)] ──POST /api/metrics──▶ [Collector API (Go + Gin)]
                                              │
                                    [PostgreSQL + TimescaleDB]
                                              │
                                    [Alert Worker (goroutine)]
                                              │
                                         [Slack Webhook]

[React Dashboard] ◀────── REST API ──────────┘
```

## 기술 스택

| 구성요소 | 기술 |
|---------|------|
| 에이전트 | Go, gopsutil |
| 백엔드 | Go, Gin, pgx/v5 |
| DB | PostgreSQL 16 + TimescaleDB |
| 프론트엔드 | React 18, Vite, Tailwind CSS, recharts |
| 알림 | Slack Incoming Webhook |
| 인프라 | Docker Compose |

## 빠른 시작

### 1. 저장소 클론

```bash
git clone https://github.com/fghjk000/AlwaysOn.git
cd AlwaysOn
```

### 2. DB 실행

```bash
docker-compose up -d
```

### 3. 마이그레이션

```bash
cd server
go run cmd/migrate/main.go up
```

### 4. 백엔드 서버 실행

```bash
cd server
cp .env.example .env
# .env에서 SLACK_WEBHOOK_URL 설정
go run main.go
```

### 5. 에이전트 설치 (모니터링 대상 서버마다)

```bash
cd agent
go build -o alwayson-agent .
cp agent.yaml.example agent.yaml
# agent.yaml 수정: server_url, host, name 설정
./alwayson-agent agent.yaml
```

### 6. 대시보드 실행

```bash
cd frontend
npm install
npm run dev
# http://localhost:5173 접속
```

## 장애 감지 임계값

| 메트릭 | Warning | Critical |
|--------|---------|----------|
| CPU | 75% | 90% |
| 메모리 | 80% | 95% |
| 디스크 | 80% | 90% |
| 미응답 | — | Down (30초 이상) |

> 서버별 임계값 커스텀 설정: `PUT /api/servers/:id/thresholds`

## 환경변수 (server/.env)

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=alwayson
DB_PASSWORD=alwayson123
DB_NAME=alwayson
SERVER_PORT=8080
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXXXXXX
```

`SLACK_WEBHOOK_URL`을 비워두면 알림이 콘솔에 출력됩니다.

## API

```
POST /api/metrics                    에이전트 메트릭 수신
GET  /api/servers                    서버 목록 + 현재 상태
GET  /api/servers/:id/metrics?hours= 시계열 메트릭 조회
GET  /api/alerts?limit=              알림 히스토리
PUT  /api/servers/:id/thresholds     임계값 설정
```

## 에이전트 설정 (agent.yaml)

```yaml
server_url: "http://모니터링서버IP:8080"
host: "web-server-01"   # 서버 고유 식별자
name: "Web Server 01"   # 대시보드 표시명
interval_seconds: 5
```
