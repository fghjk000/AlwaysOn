# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 프로젝트 개요

AlwaysOn은 온프레미스 서버 실시간 장애 모니터링 플랫폼이다. 각 서버에 Go 에이전트를 설치해 메트릭을 Push하고, 장애 감지 시 Slack으로 알림을 전송한다.

## 기술스택

- **agent/**: Go 1.22+, gopsutil/v3 (메트릭 수집), YAML 설정
- **server/**: Go 1.22+, Gin v1, pgx/v5, golang-migrate
- **frontend/**: React 18, Vite, Tailwind CSS, recharts, React Router
- **DB**: PostgreSQL 16 + TimescaleDB (metrics 테이블은 hypertable)
- **인프라**: Docker Compose (로컬 개발용 DB)

## 아키텍처

```
[에이전트(Go)] ─POST /api/metrics─▶ [Collector API (Go+Gin)]
                                              │
                                    [PostgreSQL+TimescaleDB]
                                              │
                                    [Alert Worker (goroutine)]
                                              │
                                         [Slack Webhook]

[React Dashboard] ◀──── REST API ────────────┘
```

- **에이전트**: 5초 간격으로 CPU/메모리/디스크/네트워크 수집 후 중앙 서버로 HTTP POST
- **Alert Worker**: 메트릭 수신마다 goroutine으로 임계값 체크, 10분 쿨다운으로 중복 알림 방지
- **서버 자동 등록**: 에이전트가 보내는 `host` 기준으로 서버 upsert (처음 보는 서버 자동 생성)

## 서버 레이어 구조

```
server/
├── api/          # Gin 핸들러 (요청/응답만 처리)
├── service/      # 비즈니스 로직 (인터페이스 기반, mock 가능)
├── repository/   # DB 쿼리 (pgx/v5 직접 사용, ORM 없음)
├── worker/       # Alert Worker goroutine
├── model/        # 공유 데이터 구조체
└── config/       # 환경변수 로드
```

service 레이어는 repository 인터페이스에 의존하므로 mock으로 단위 테스트 가능.

## 공통 명령어

### 개발 환경 시작

```bash
# DB 실행 (최초 1회)
docker-compose up -d

# DB 마이그레이션
cd server && go run cmd/migrate/main.go up
```

### 백엔드 서버

```bash
cd server
cp .env.example .env      # SLACK_WEBHOOK_URL 설정
go run main.go            # 기본 포트 :8080
```

### 에이전트

```bash
cd agent
go build -o alwayson-agent .
cp agent.yaml.example agent.yaml  # server_url, host, name 설정
./alwayson-agent agent.yaml
```

### 프론트엔드

```bash
cd frontend
npm install
npm run dev               # http://localhost:5173
npm run build             # 프로덕션 빌드
```

### 테스트

```bash
# 백엔드 단위 테스트 (mock 기반, DB 불필요)
cd server && go test ./...

# 백엔드 통합 테스트 (DB 필요)
cd server && INTEGRATION=1 go test ./repository/... -v

# 에이전트 테스트
cd agent && go test ./... -v

# 단일 테스트 실행
go test ./worker/... -run TestAlertWorker_Check_TriggersWarning -v
```

### 빌드 확인

```bash
cd server && go build ./...
cd agent && go build ./...
```

## 환경변수 (server/.env)

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `DB_HOST` | `localhost` | PostgreSQL 호스트 |
| `DB_PORT` | `5432` | PostgreSQL 포트 |
| `DB_USER` | `alwayson` | DB 사용자 |
| `DB_PASSWORD` | `alwayson123` | DB 비밀번호 |
| `DB_NAME` | `alwayson` | DB 이름 |
| `SERVER_PORT` | `8080` | API 서버 포트 |
| `SLACK_WEBHOOK_URL` | (비워두면 콘솔 출력) | Slack Incoming Webhook URL |

## API 엔드포인트

```
POST /api/metrics                    ← 에이전트가 메트릭 전송
GET  /api/servers                    ← 서버 목록 + 현재 상태
GET  /api/servers/:id/metrics?hours= ← 시계열 메트릭 (기본 1시간)
GET  /api/alerts?limit=              ← 알림 히스토리
PUT  /api/servers/:id/thresholds     ← 임계값 커스텀 설정
```

## 장애 감지 규칙

| 메트릭 | Warning | Critical |
|--------|---------|----------|
| CPU | 75% | 90% |
| 메모리 | 80% | 95% |
| 디스크 | 80% | 90% |

- `last_seen` > 30초 이상이면 서버 상태 `down`으로 전환
- 동일 서버/동일 레벨/동일 메트릭은 10분 쿨다운 후 재알림
- 상태 정상화 시 복구 알림 자동 전송

## 에이전트 설정 파일 (agent.yaml)

```yaml
server_id: ""          # 비우면 host 기준으로 서버에서 자동 생성
server_url: "http://서버IP:8080"
host: "web-server-01"  # 서버 고유 식별자 (중복 불가)
name: "Web Server 01"  # 대시보드 표시명
interval_seconds: 5
```

## 계획 문서

- `docs/plans/2026-03-02-alwayson-design.md` — 설계 결정 및 아키텍처 근거
- `docs/plans/2026-03-02-alwayson-implementation.md` — 단계별 구현 계획 (14 Tasks)
