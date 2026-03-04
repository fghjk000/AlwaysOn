# AlwaysOn - 모니터링 범위 확장 설계 문서

**작성일**: 2026-03-04
**상태**: 승인됨

---

## 개요

기존 시스템 메트릭(CPU/메모리/디스크/네트워크) 모니터링에 더해 두 가지 기능을 추가한다.

1. **HTTP/포트 헬스체크** — 서버 사이드 폴링 방식
2. **프로세스 감시** — 에이전트 확장 방식

---

## 아키텍처

```
[에이전트] ─ 프로세스 상태 포함하여 POST /api/metrics ─▶ [서버]
                                                              │
[서버 HealthCheckWorker] ─ HTTP/TCP 직접 폴링 ─▶ [모니터링 대상]
                                                              │
                                                    [DB 저장 + 알림]
```

---

## 기능 1: HTTP/포트 헬스체크 (서버 사이드 폴링)

### 동작 방식

- 사용자가 대시보드 또는 API로 헬스체크 대상(URL 또는 TCP 포트) 등록
- `HealthCheckWorker` goroutine이 30초마다 DB에서 enabled 설정 로드 후 병렬 체크
- 실패 시 기존 `alerts` 테이블에 기록 + Slack 알림
- 기존 `alert_cooldowns` 재활용해 중복 알림 방지

### DB 스키마

```sql
CREATE TABLE health_check_configs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  server_id       UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  name            VARCHAR(255) NOT NULL,
  type            VARCHAR(10) NOT NULL CHECK (type IN ('http', 'tcp')),
  target          VARCHAR(512) NOT NULL,   -- HTTP URL 또는 host:port
  expected_status INT DEFAULT 200,         -- HTTP 타입만 사용
  interval_sec    INT NOT NULL DEFAULT 30,
  enabled         BOOLEAN NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### API 엔드포인트

```
GET    /api/servers/:id/health-checks       ← 등록된 헬스체크 목록 조회
POST   /api/servers/:id/health-checks       ← 헬스체크 추가
DELETE /api/servers/:id/health-checks/:hid  ← 헬스체크 삭제
```

### 요청/응답 예시

```json
// POST /api/servers/:id/health-checks
{
  "name": "Nginx",
  "type": "http",
  "target": "http://10.0.0.1/health",
  "expected_status": 200,
  "interval_sec": 30
}

// POST /api/servers/:id/health-checks (TCP)
{
  "name": "PostgreSQL",
  "type": "tcp",
  "target": "10.0.0.1:5432",
  "interval_sec": 60
}
```

### 알림 규칙

- metric 필드: `"health_check:<name>"` (예: `"health_check:Nginx"`)
- HTTP: 응답 상태코드 불일치 또는 타임아웃(5초) 시 실패
- TCP: 연결 실패 또는 타임아웃(5초) 시 실패
- 실패는 `critical` 레벨로 처리
- 복구 시 resolved_at 업데이트 + Slack 복구 알림

---

## 기능 2: 프로세스 감시 (에이전트 확장)

### 동작 방식

- `agent.yaml`에 감시할 프로세스 이름 목록 설정
- 에이전트가 기존 메트릭 수집 주기(5초)에 프로세스 실행 여부 함께 확인
- `/api/metrics` 페이로드에 프로세스 상태 포함해 전송
- 서버의 `MetricService`에서 파싱 후 미실행 프로세스 알림

### 에이전트 설정

```yaml
# agent.yaml
processes:
  - nginx
  - mysql
  - redis-server
```

### 메트릭 페이로드 확장

```json
{
  "host": "web-01",
  "name": "Web Server 01",
  "cpu": 45.2,
  "memory": 60.1,
  "disk": 55.0,
  "net_in": 1024,
  "net_out": 512,
  "processes": [
    {"name": "nginx",        "running": true},
    {"name": "mysql",        "running": false},
    {"name": "redis-server", "running": true}
  ]
}
```

### 서버 처리

- `model.Metric`에 `Processes []ProcessStatus` 필드 추가
- `MetricService.Collect()`에서 프로세스 상태 파싱
- `running: false` 프로세스 감지 시 `alerts` 테이블에 기록 + Slack 알림
- metric 필드: `"process:<name>"` (예: `"process:mysql"`)
- 기존 `alert_cooldowns` 재활용해 중복 알림 방지

### 알림 규칙

- 레벨: `critical`
- 복구: 프로세스가 다시 `running: true`로 오면 resolved_at 업데이트 + Slack 복구 알림

---

## 대시보드 변경

### 서버 상세 페이지

- **헬스체크 패널**: 등록된 헬스체크 목록, 현재 상태 (초록/빨강), 추가/삭제 버튼
- **프로세스 패널**: 감시 중인 프로세스 뱃지 (초록: 실행 중 / 빨강: 미실행)

---

## 변경 파일 요약

| 파일/디렉토리 | 변경 내용 |
|---|---|
| `migrations/` | `health_check_configs` 테이블 추가 |
| `server/model/` | `HealthCheckConfig`, `ProcessStatus` 구조체 추가, `Metric`에 `Processes` 필드 추가 |
| `server/repository/` | `HealthCheckRepo` 추가 |
| `server/service/` | `MetricService`에 프로세스 알림 로직 추가 |
| `server/worker/` | `HealthCheckWorker` 추가 |
| `server/api/` | 헬스체크 CRUD 핸들러 추가, router 등록 |
| `server/main.go` | `HealthCheckWorker` 시작 등록 |
| `agent/` | `agent.yaml` processes 파싱, 프로세스 수집 로직 추가 |
| `frontend/` | 헬스체크 패널, 프로세스 뱃지 컴포넌트 추가 |
