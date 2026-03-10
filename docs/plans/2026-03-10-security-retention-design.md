# Security & Retention Design

**Date:** 2026-03-10
**Goal:** API 인증과 TimescaleDB 데이터 보존 정책 추가

---

## 1. API 인증 (Agent API Key)

### 문제
`POST /api/metrics`가 인증 없이 열려있어 누구나 가짜 메트릭을 밀어넣을 수 있음.

### 설계 결정
- **방식:** `X-Agent-Key` 헤더 기반 공유 API Key
- **적용 범위:** `POST /api/metrics` 만 (대시보드 GET은 내부망 전용이므로 제외)
- **키 관리:** 환경변수 `AGENT_API_KEY`로 설정. 비어있으면 인증 비활성화 (개발 편의)
- **구현:** Gin 미들웨어 `AgentAuthMiddleware`로 분리, 해당 라우트에만 적용
- **에이전트:** `agent.yaml`에 `api_key` 필드 추가, HTTP 요청 헤더에 자동 포함

### 변경 파일
- `server/config/config.go` — `AgentAPIKey` 필드 추가
- `server/api/middleware.go` — `AgentAuthMiddleware` 신규 생성
- `server/api/router.go` — `/api/metrics` 라우트에 미들웨어 적용
- `server/.env.example` — `AGENT_API_KEY` 항목 추가
- `agent/agent.go` — `api_key` 설정 읽어 헤더 포함
- `agent/agent.yaml.example` — `api_key` 필드 추가

---

## 2. TimescaleDB 데이터 보존 정책

### 문제
메트릭 데이터가 무한정 누적됨. 5초 간격 수집 시 서버 10대 기준 월 500만건 이상 → 디스크 고갈 위험.

### 설계 결정
- **방식:** TimescaleDB `add_retention_policy()` 네이티브 기능 사용
- **기본값:** 30일 보존
- **설정:** 환경변수 `METRICS_RETENTION_DAYS`로 조정 가능 (기본 30)
- **적용:** DB 마이그레이션 파일 `000004_retention_policy`로 관리
- **대상:** `metrics` hypertable 만 해당 (servers, alerts 등 일반 테이블 제외)

### 변경 파일
- `migrations/000004_retention_policy.up.sql` — retention policy 적용
- `migrations/000004_retention_policy.down.sql` — retention policy 제거
- `server/config/config.go` — `MetricsRetentionDays` 필드 추가 (문서용)

---

## 우선순위
1. TimescaleDB 보존 정책 (마이그레이션만으로 완료, 독립적)
2. API 인증 (서버 + 에이전트 양쪽 변경 필요)
