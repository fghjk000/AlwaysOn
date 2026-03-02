# AlwaysOn - 실시간 장애 모니터링 플랫폼 설계 문서

**작성일**: 2026-03-02
**상태**: 승인됨

---

## 개요

온프레미스 서버들의 상태를 실시간으로 수집하고, 장애를 감지하여 Slack으로 알림을 전송하는 모니터링 플랫폼.

---

## 요구사항

### 핵심 기능
- 각 서버에 설치된 **에이전트(Push 방식)**로 메트릭 수집 (5초 간격)
- 수집 메트릭: CPU 사용률, 메모리 사용률, 디스크 사용률, 네트워크 I/O
- 3단계 장애 감지: Warning → Critical → Down
- 장애/복구 시 Slack 알림 전송
- React 기반 실시간 대시보드

### 기술스택
- **에이전트**: Go (gopsutil, 단일 바이너리 배포)
- **백엔드**: Go + Gin
- **DB**: PostgreSQL + TimescaleDB (시계열 메트릭 최적화)
- **프론트엔드**: React + recharts
- **알림**: Slack Webhook

---

## 아키텍처

```
[에이전트(Go)] ─HTTP POST─▶ [Collector API (Go+Gin)]
                                       │
                             [PostgreSQL+TimescaleDB]
                                       │
                             [Alert Worker (goroutine)]
                                       │
                                  [Slack Webhook]

[React Dashboard] ◀──── REST API ─────┘
```

### 컴포넌트 역할

| 컴포넌트 | 역할 |
|---------|------|
| Agent | 대상 서버에 설치, 메트릭 수집 후 중앙 서버로 Push |
| Collector API | 메트릭 수신, REST API 제공, 대시보드 데이터 서빙 |
| Alert Worker | 임계값 체크, 알림 중복 방지, Slack 전송 |
| DB | 메트릭 시계열 저장, 서버/알림/임계값 관리 |
| Dashboard | 서버 목록, 실시간 차트, 알림 히스토리 UI |

---

## API 설계

```
POST /api/metrics                       ← 에이전트 메트릭 수신
GET  /api/servers                       ← 서버 목록 + 현재 상태
GET  /api/servers/:id                   ← 서버 상세
GET  /api/servers/:id/metrics           ← 시계열 메트릭 조회
GET  /api/alerts                        ← 알림 히스토리
PUT  /api/servers/:id/thresholds        ← 임계값 커스텀 설정
```

---

## DB 스키마

```sql
-- 서버 등록 정보
servers (
  id          UUID PRIMARY KEY,
  name        VARCHAR,
  host        VARCHAR,
  status      VARCHAR,   -- normal, warning, critical, down
  last_seen   TIMESTAMP
)

-- 시계열 메트릭 (TimescaleDB hypertable)
metrics (
  time        TIMESTAMPTZ NOT NULL,
  server_id   UUID,
  cpu         FLOAT,
  memory      FLOAT,
  disk        FLOAT,
  net_in      BIGINT,
  net_out     BIGINT
)

-- 알림 기록
alerts (
  id          UUID PRIMARY KEY,
  server_id   UUID,
  level       VARCHAR,   -- warning, critical, down
  metric      VARCHAR,
  value       FLOAT,
  message     TEXT,
  created_at  TIMESTAMPTZ,
  resolved_at TIMESTAMPTZ
)

-- 서버별 커스텀 임계값
thresholds (
  server_id   UUID PRIMARY KEY,
  cpu_warning    FLOAT DEFAULT 75,
  cpu_critical   FLOAT DEFAULT 90,
  mem_warning    FLOAT DEFAULT 80,
  mem_critical   FLOAT DEFAULT 95,
  disk_warning   FLOAT DEFAULT 80,
  disk_critical  FLOAT DEFAULT 90
)
```

---

## 장애 감지 로직

### 임계값 (기본값, 서버별 오버라이드 가능)

| 메트릭 | Warning | Critical |
|--------|---------|----------|
| CPU | 75% | 90% |
| 메모리 | 80% | 95% |
| 디스크 | 80% | 90% |
| 서버 미응답 | - | - | Down (last_seen > 30초) |

### 알림 규칙
- **중복 방지**: 동일 서버/동일 레벨은 10분 쿨다운 후 재알림
- **에스컬레이션**: Warning → Critical 전환 시 즉시 알림
- **복구 알림**: 상태 정상화 시 Slack에 복구 메시지 전송

---

## 대시보드 화면 구성

1. **서버 목록 (홈)**: 서버 카드 그리드, 상태 색상 (초록/노랑/주황/빨강), CPU/메모리 요약
2. **서버 상세**: 실시간 메트릭 차트, 조회 범위 선택 (1h/6h/24h), 알림 히스토리
3. **알림 히스토리**: 전체 알림 목록, 레벨별 필터, 해결 시각 표시

---

## 프로젝트 구조

```
AlwaysOn/
├── agent/                  # Go 에이전트 (각 서버에 배포)
│   ├── main.go
│   ├── collector.go        # gopsutil 메트릭 수집
│   ├── sender.go           # HTTP POST 전송
│   └── agent.yaml.example
├── server/                 # Go 백엔드
│   ├── main.go
│   ├── api/                # Gin 라우터 및 핸들러
│   ├── service/            # 비즈니스 로직
│   ├── repository/         # DB 쿼리
│   ├── worker/             # Alert Worker goroutine
│   ├── model/              # 데이터 모델
│   └── config/
├── frontend/               # React 대시보드
│   ├── src/
│   │   ├── pages/
│   │   ├── components/
│   │   └── api/
│   └── package.json
├── migrations/             # DB 마이그레이션 SQL
├── docker-compose.yml      # 로컬 개발용 PostgreSQL+TimescaleDB
└── README.md
```
