# Linux Chess Portfolio

실시간 멀티플레이 체스 서비스를 기반으로 만든 백엔드 및 Linux 운영 포트폴리오입니다.

단순한 브라우저 체스 게임이 아니라, Linux 서버에서 실제 서비스처럼 실행할 수 있도록 인증, WebSocket, 서버 측 체스 검증, 데이터 저장, Redis 캐시, 리버스 프록시, 모니터링, 알림, 백업, 배포 문서까지 함께 구성했습니다.

![Linux Chess demo](docs/assets/linux-chess-demo.png)

관측성 화면:

- [Prometheus targets](docs/assets/observability/prometheus-targets.png)
- [Grafana dashboard](docs/assets/observability/grafana-dashboard.png)
- [Alertmanager alerts](docs/assets/observability/alertmanager-alerts.png)

## What I Built / 만든 것

브라우저끼리 상태를 맞추는 데모가 아니라 Go 서버가 체스 규칙과 게임 상태를 책임지는 실시간 서비스를 만들었습니다. 사용자는 계정을 만들고 WebSocket 대국이나 Stockfish AI 대국을 진행할 수 있으며, 완료된 게임은 PostgreSQL에 저장하고 진행 중인 방과 게임 상태는 Redis로 관리합니다.

## Project Summary / 프로젝트 구성

| 항목 | 내용 |
| --- | --- |
| 프로젝트명 | Linux Chess Portfolio |
| 목적 | 실시간 게임 서비스와 Linux 운영 구성을 하나의 실행 가능한 프로젝트로 구현 |
| 주요 기능 | 실시간 체스 대국, AI 대국, 비공개 방 코드, 회원가입/로그인, 게임 기록, PGN 리뷰, 리플레이 뷰어, 재대전, 운영 상태 확인 |
| 백엔드 | Go, REST API, WebSocket, 서버 측 체스 규칙 검증 |
| 프론트엔드 | React, TypeScript, Vite, chess.js |
| 데이터 저장 | PostgreSQL |
| 캐시 | Redis |
| 운영/배포 | Docker Compose, Nginx, Caddy, systemd, Prometheus, Grafana, Alertmanager |
| 품질 관리 | Go 테스트, TypeScript 검사, 프로덕션 빌드, Playwright smoke test |

## Key Implementations / 주요 구현 내용

- WebSocket 기반 실시간 체스 대국
- 브라우저가 아닌 서버에서 체스 수 검증
- Stockfish UCI 연동 및 내장 휴리스틱 AI fallback
- 쿠키 기반 회원가입/로그인 세션
- 최근 게임 기록, 게임 상세 조회, PGN 리뷰, 리플레이 뷰어, 재대전
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

## Development / 개발 방식

### Authoritative game server

- 클라이언트는 이동 요청만 보내고 Go 서버가 현재 turn, legal move, check·mate 상태를 검증합니다.
- 검증된 상태만 WebSocket room에 broadcast해 참가자가 동일한 game state를 받도록 구성했습니다.
- 연결 종료, 기권, 게임 종료 이벤트도 서버 상태 변경을 기준으로 처리합니다.

### Persistence and runtime state

- PostgreSQL에는 사용자, 완료된 게임, move history, PGN 조회에 필요한 영속 데이터를 저장합니다.
- Redis에는 비공개 room code와 진행 중인 game state처럼 빠르게 만료·조회할 런타임 데이터를 둡니다.
- 데이터베이스가 설정되지 않은 개발 환경에서도 실행할 수 있도록 저장소 경계를 분리했습니다.

### Operations

- Nginx 또는 Caddy가 정적 프론트엔드와 Go API·WebSocket 요청을 분기합니다.
- `/health`는 프로세스 상태, `/ready`는 의존 서비스 준비 상태, `/metrics`는 Prometheus 지표를 제공합니다.
- Docker Compose에 PostgreSQL, Redis, API, frontend, Prometheus, Grafana, Alertmanager를 함께 정의했습니다.
- systemd unit, backup/restore script, incident runbook, production deployment 문서를 저장소에 포함했습니다.

### AI opponent

- Stockfish는 UCI 프로토콜로 실행하고 환경변수와 기본 경로에서 바이너리를 탐색합니다.
- Stockfish를 사용할 수 없으면 legal move 기반 내장 heuristic AI로 전환해 게임 흐름을 유지합니다.

## Architecture / 아키텍처

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

## Directory Structure / 디렉터리 구조

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

## Local Run / 로컬 실행 방법

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

## Main APIs / 주요 API

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

## Stockfish AI / Stockfish 인공지능

백엔드는 UCI 프로토콜을 통해 Stockfish를 사용할 수 있습니다. 탐색 순서는 다음과 같습니다.

1. `STOCKFISH_PATH`
2. `tools/stockfish/stockfish/stockfish-ubuntu-x86-64-avx2`
3. `PATH`에 등록된 `stockfish`

Stockfish를 찾을 수 없는 경우에는 내장된 legal-move 기반 휴리스틱 AI로 fallback합니다.

Debian/Ubuntu에서는 다음 명령으로 설치할 수 있습니다.

```bash
sudo apt install stockfish
```

## Data Storage and Cache / 데이터 저장 및 캐시

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

## Docker Operations / Docker 운영 구성

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

## Temporary Public Preview / 임시 공개 미리보기

서버나 도메인 없이 다른 컴퓨터에서 잠깐 접속해보게 하려면 Cloudflare Tunnel 미리보기를 사용할 수 있습니다.

```bash
npm run preview:tunnel
```

명령이 Docker 스택을 `http://localhost:8080`으로 실행한 뒤 `https://*.trycloudflare.com` 형태의 임시 공개 주소를 출력합니다. 그 주소를 다른 사람에게 공유하면 다른 컴퓨터나 휴대폰에서 멀티플레이 체스 흐름을 테스트할 수 있습니다.

자세한 내용은 `docs/preview-tunnel.md`를 참고하세요.

## Validation / 검증 방법

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

## Deployment Documentation / 배포 문서

운영용 Docker Compose 구성:

```bash
cp .env.prod.example .env.prod
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

VPS, DNS, HTTPS, 방화벽, 백업, 운영 점검 절차는 `docs/production-deploy.md`와 `docs/operations-checklist.md`에 정리되어 있습니다.

## Demo Flow / 실행 확인 흐름

다음 순서로 주요 기능과 운영 구성을 확인할 수 있습니다.

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

## License / 라이선스

MIT License. 자세한 내용은 [LICENSE](LICENSE)를 참고하세요.
