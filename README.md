# Linux Chess Portfolio

실시간 멀티플레이 체스 서비스를 기반으로 만든 백엔드 및 Linux 운영 포트폴리오입니다.

단순한 브라우저 체스 게임이 아니라, Linux 서버에서 실제 서비스처럼 실행할 수 있도록 인증, WebSocket, 서버 측 체스 검증, 데이터 저장, Redis 캐시, 리버스 프록시, 모니터링, 알림, 백업, 배포 문서까지 함께 구성했습니다.

![Linux Chess demo](docs/assets/linux-chess-demo.png)

관측성 화면:

- [Prometheus targets](docs/assets/observability/prometheus-targets.png)
- [Grafana dashboard](docs/assets/observability/grafana-dashboard.png)
- [Alertmanager alerts](docs/assets/observability/alertmanager-alerts.png)

## 프로젝트 요약

| 항목 | 내용 |
| --- | --- |
| 프로젝트명 | Linux Chess Portfolio |
| 목적 | 백엔드 개발, 실시간 통신, Linux 서버 운영 역량을 보여주기 위한 포트폴리오 |
| 주요 기능 | 실시간 체스 대국, AI 대국, 비공개 방 코드, 회원가입/로그인, 게임 기록, PGN 리뷰, 운영 상태 확인 |
| 백엔드 | Go, REST API, WebSocket, 서버 측 체스 규칙 검증 |
| 프론트엔드 | React, TypeScript, Vite, chess.js |
| 데이터 저장 | PostgreSQL |
| 캐시 | Redis |
| 운영/배포 | Docker Compose, Nginx, Caddy, systemd, Prometheus, Grafana, Alertmanager |
| 품질 관리 | Go 테스트, TypeScript 검사, 프로덕션 빌드, Playwright smoke test |

## 지원 직무와 연결되는 역량

이 프로젝트는 다음 직무에 맞춰 설명할 수 있도록 구성했습니다.

- 백엔드 개발자
- 주니어 DevOps 엔지니어
- Linux 서버 운영자
- 인프라 이해도가 있는 풀스택 개발자

핵심은 “체스 게임을 만들었다”가 아니라, 사용자가 접속하고 게임을 진행하는 실제 서비스를 Linux 환경에서 운영 가능한 형태로 만들었다는 점입니다.

## 주요 구현 내용

- WebSocket 기반 실시간 체스 대국
- 브라우저가 아닌 서버에서 체스 수 검증
- Stockfish UCI 연동 및 내장 휴리스틱 AI fallback
- 쿠키 기반 회원가입/로그인 세션
- 최근 게임 기록, 게임 상세 조회, PGN 리뷰
- 방 코드 기반 비공개 매칭
- PostgreSQL 기반 사용자/게임 기록 저장
- Redis 기반 방 코드 및 진행 중 게임 상태 캐시
- `/health`, `/ready`, `/metrics` 운영 엔드포인트
- Prometheus 메트릭 수집 및 Grafana 대시보드
- Alertmanager 알림 예시 구성
- Nginx/Caddy 리버스 프록시 설정
- Docker Compose 로컬 운영 토폴로지
- Linux systemd 서비스 예시
- 백업/복구, 장애 대응, 배포 체크리스트 문서화
- GitHub Actions CI 및 Playwright 브라우저 smoke test

## 기술별 포트폴리오 포인트

| 영역 | 확인할 수 있는 내용 |
| --- | --- |
| Backend | Go REST handler, WebSocket game loop, 인증/세션 처리, 서버 측 체스 수 검증 |
| Frontend | React 체스 UI, 계정 패널, 게임 기록, 분석 패널, 운영 상태 패널 |
| Database | PostgreSQL 스키마, 사용자/게임 저장, 통계 및 상세 조회 |
| Cache | Redis room/runtime state 저장 |
| Linux 운영 | systemd unit, Nginx/Caddy 설정, 백업 스크립트, 배포 체크리스트 |
| Observability | `/metrics`, Prometheus 설정, Grafana 대시보드, alert rule, JSON 로그 |
| Quality | Go test, TypeScript check, production build, Playwright smoke test |

## 면접에서 설명할 수 있는 내용

이 프로젝트는 프론트엔드에서만 동작하는 게임 데모가 아니라, 백엔드가 게임 규칙과 상태를 책임지는 서비스로 설계했습니다. 사용자는 WebSocket을 통해 실시간으로 대국을 진행하고, 서버는 모든 수를 검증한 뒤 권위 있는 게임 상태만 클라이언트에 전달합니다.

완료된 게임은 PostgreSQL에 저장할 수 있고, 진행 중인 방과 게임 상태는 Redis를 통해 런타임 캐시로 관리합니다. 운영 관점에서는 health check, readiness check, Prometheus metrics, Grafana dashboard, alert rule, backup script, systemd unit, reverse proxy 설정을 포함해 Linux 서버에서 어떻게 운영할지까지 문서화했습니다.

## 아키텍처

```mermaid
flowchart LR
  Browser[Browser] --> Edge[Nginx or Caddy]
  Edge --> Frontend[React static frontend]
  Edge --> Backend[Go API and WebSocket server]
  Backend --> Postgres[(PostgreSQL users and games)]
  Backend --> Redis[(Redis room and game cache)]
  Backend --> Stockfish[Stockfish engine]
  Prometheus[Prometheus] --> Backend
  Grafana[Grafana] --> Prometheus
```

## 디렉터리 구조

```text
backend/       Go WebSocket 및 REST API 서버
frontend/      React 체스 클라이언트
infra/         Docker, Nginx, Caddy, systemd, 운영 스크립트
docs/          Linux 운영 및 포트폴리오 문서
tests/         Playwright smoke test
```

주요 운영 문서:

- `docs/linux-ops.md`
- `docs/incident-runbook.md`
- `docs/db-gui.md`
- `docs/backup-restore.md`
- `docs/load-test.md`
- `docs/operations-checklist.md`
- `docs/production-deploy.md`
- `docs/portfolio-audit.md`
- `docs/roadmap.md`

## 로컬 실행 방법

의존성 설치:

```bash
npm install
```

백엔드 실행:

```bash
npm run dev:backend
```

프론트엔드 실행:

```bash
npm run dev:frontend
```

브라우저에서 접속:

```text
http://localhost:5173
```

PostgreSQL을 함께 사용하는 경우:

```bash
npm run dev:postgres
npm run dev:backend:db
```

## 주요 API

```bash
curl -X POST http://localhost:3000/auth/register \
  -H 'content-type: application/json' \
  -d '{"username":"player_one","password":"correct-password"}'

curl http://localhost:3000/games/recent
curl http://localhost:3000/games/stats
curl 'http://localhost:3000/games/detail?id=<game-id>'
curl http://localhost:3000/admin/status
curl http://localhost:3000/health
curl http://localhost:3000/ready
curl http://localhost:3000/metrics
```

`/admin/status`는 관리자 로그인 상태에서만 접근할 수 있습니다. 관리자 계정은 `ADMIN_USERS` 환경 변수로 설정합니다.

```bash
export ADMIN_USERS=admin,gi990422
```

운영 환경에서 WebSocket origin 검증을 사용하려면 공개 사이트 주소를 설정합니다.

```bash
export ALLOWED_ORIGINS=https://chess.example.com
```

## Stockfish AI

백엔드는 UCI 프로토콜을 통해 Stockfish를 사용할 수 있습니다. 탐색 순서는 다음과 같습니다.

1. `STOCKFISH_PATH`
2. `tools/stockfish/stockfish/stockfish-ubuntu-x86-64-avx2`
3. `PATH`에 등록된 `stockfish`

Stockfish를 찾을 수 없는 경우에는 내장된 legal-move 기반 휴리스틱 AI로 fallback합니다.

Debian/Ubuntu에서는 다음 명령으로 설치할 수 있습니다.

```bash
sudo apt install stockfish
```

## 데이터 저장 및 캐시

`DATABASE_URL`을 설정하면 PostgreSQL 저장소를 사용합니다.

```bash
export DATABASE_URL=postgres://chess:chess@localhost:5432/chess
```

스키마는 `infra/sql/001_games.sql`에 있으며, 명시적으로 migration을 실행할 수 있습니다.

```bash
DATABASE_URL=postgres://chess:chess@localhost:5432/chess npm run migrate
```

`REDIS_URL`을 설정하면 방 코드와 진행 중 게임 상태를 Redis에 기록합니다.

```bash
export REDIS_URL=redis://localhost:6379
```

## Docker 운영 구성

```bash
docker compose up --build
```

포함된 서비스:

- `frontend`: Nginx로 제공되는 React 정적 빌드
- `backend`: Go WebSocket 및 REST API 서버
- `postgres`: 사용자/게임/수 기록 저장소
- `redis`: 방 코드 및 진행 중 게임 상태 캐시
- `reverse-proxy`: 8080/8443 포트의 공개 진입점
- `prometheus`: 메트릭 수집
- `grafana`: 대시보드
- `alertmanager`: 알림 라우팅 예시

실행 후 접속 주소:

- App: `http://localhost:8080`
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3001` (`admin` / `chess`)

## 임시 공개 미리보기

서버나 도메인 없이 다른 컴퓨터에서 잠깐 접속해보게 하려면 Cloudflare Tunnel 미리보기를 사용할 수 있습니다.

```bash
npm run preview:tunnel
```

명령이 Docker 스택을 `http://localhost:8080`으로 실행한 뒤 `https://*.trycloudflare.com` 형태의 임시 공개 주소를 출력합니다. 그 주소를 다른 사람에게 공유하면 다른 컴퓨터나 휴대폰에서 멀티플레이 체스 흐름을 테스트할 수 있습니다.

자세한 내용은 `docs/preview-tunnel.md`를 참고하세요.

## 검증 방법

단위 테스트, 타입 검사, 프로덕션 빌드:

```bash
npm run lint
npm run build
```

브라우저 smoke test:

```bash
npx playwright install chromium
npm run smoke
```

Smoke test는 회원가입, AI 게임 시작, 분석 요청, 비공개 방 참가, 기권, 저장된 게임 상세 조회 흐름을 확인합니다.

## 배포 문서

운영용 Docker Compose 구성:

```bash
cp .env.prod.example .env.prod
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

VPS, DNS, HTTPS, 방화벽, 백업, 운영 점검 절차는 `docs/production-deploy.md`와 `docs/operations-checklist.md`에 정리되어 있습니다.

## 시연 순서

포트폴리오 리뷰 또는 면접 시 다음 순서로 보여줄 수 있습니다.

1. React 클라이언트에서 회원가입 또는 로그인
2. AI 게임 시작 또는 비공개 방 생성
3. 합법적인 수를 둔 뒤 서버가 상태를 갱신하는 흐름 확인
4. 분석 요청으로 AI 엔진 응답 확인
5. 게임 종료 후 최근 기록과 상세 PGN 확인
6. `/health`, `/ready`, `/metrics` 확인
7. Prometheus에서 `chess_active_connections` 쿼리
8. Grafana Chess Service Overview 대시보드 확인
9. Alertmanager 및 alert rule 확인
10. Docker Compose, Nginx/Caddy, systemd, backup script 설명
11. `npm run smoke`로 자동화된 브라우저 검증 실행

## 프로젝트를 통해 보여주고 싶은 점

이 저장소는 UI만 있는 미니 프로젝트가 아니라, 작은 기능이라도 실제 서비스처럼 설계하고 운영할 수 있다는 점을 보여주기 위해 만들었습니다.

체스라는 도메인을 통해 실시간 상태 동기화와 서버 검증을 명확하게 보여주고, Linux 운영 구성으로 배포, 모니터링, 백업, 장애 대응까지 함께 설명할 수 있도록 구성했습니다.
