# Linux Chess Portfolio

Realtime multiplayer chess service built as a backend and Linux operations portfolio.

This project is designed to show how I build, package, monitor, and document a service that can run on a Linux server. The chess product gives reviewers an interactive workflow, while the surrounding infrastructure demonstrates backend engineering, deployment, observability, and operations practice.

![Linux Chess demo](docs/assets/linux-chess-demo.png)

Observability screenshots:

- [Prometheus targets](docs/assets/observability/prometheus-targets.png)
- [Grafana dashboard](docs/assets/observability/grafana-dashboard.png)
- [Alertmanager alerts](docs/assets/observability/alertmanager-alerts.png)

## Portfolio Summary

**Target roles:** backend developer, junior DevOps engineer, Linux server operator, full-stack developer with infrastructure responsibilities.

**Core idea:** not just a browser chess game. It is a small production-style service with authentication, realtime WebSocket traffic, persistence, cache, reverse proxying, health checks, metrics, alert rules, CI, smoke tests, and operations documentation.

**Repository:** `github.com/jiyoon99/linux-chess-portfolio`

## Highlights

- Realtime WebSocket gameplay
- Server-side chess validation
- Cookie-based account registration and login
- Authenticated recent game history
- Authenticated game detail and PGN review
- In-memory matchmaking with room codes and AI games
- PostgreSQL-ready game persistence layer
- Redis-backed room/game runtime cache
- Prometheus metrics, health checks, readiness checks, and alert rules
- Readiness checks for DB and Redis
- Authenticated operations status panel
- Prometheus and Grafana observability stack in Docker Compose
- Nginx reverse proxy for HTTP and WebSocket traffic
- Docker Compose local production topology
- systemd service examples for Linux deployment
- Backup, logging, firewall, and monitoring notes
- GitHub Actions CI and Playwright smoke test

## Skills Demonstrated

| Area | Evidence in this repo |
| --- | --- |
| Backend | Go REST handlers, WebSocket game loop, auth/session logic, server-side move validation |
| Frontend | React chess UI, account panel, game history, analysis panel, operations status panel |
| Database | PostgreSQL schema, user/game persistence, stats and game detail queries |
| Cache | Redis room and active game runtime records |
| Linux operations | systemd unit, Nginx/Caddy configs, backup script, deployment checklist |
| Observability | `/metrics`, Prometheus config, Grafana dashboard, alert rules, JSON logs |
| Quality | Go tests, TypeScript check, production build, Playwright browser smoke test, CI |

## What I Would Explain In An Interview

I built this as an end-to-end Linux service rather than a simple UI demo. The backend owns chess validation so the browser cannot submit illegal moves. WebSocket traffic is routed through a reverse proxy, completed games can be persisted to PostgreSQL, and Redis is used for room/game runtime state. The service exposes health, readiness, and Prometheus metrics, with Grafana dashboards and alert rules for operational review. The repository also includes systemd, backup/restore, incident response, and production deployment notes to show how the app would be run and maintained.

## Repository Layout

```text
backend/       Go WebSocket and REST API server
frontend/      React chess client
infra/         Docker, Nginx, systemd, scripts
docs/          Linux operations documentation
.github/       CI workflow
```

Operational docs:

- `docs/linux-ops.md`
- `docs/incident-runbook.md`
- `docs/db-gui.md`
- `docs/backup-restore.md`
- `docs/load-test.md`
- `docs/operations-checklist.md`
- `docs/production-deploy.md`
- `docs/portfolio-audit.md`
- `docs/roadmap.md`

## Architecture

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

## Local Development

Install dependencies:

```bash
npm install
```

Run backend:

```bash
npm run dev:backend
```

Run backend with PostgreSQL persistence:

```bash
npm run dev:postgres
npm run dev:backend:db
```

Run frontend in another terminal:

```bash
npm run dev:frontend
```

Open `http://localhost:5173`.

Useful backend endpoints:

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

`/admin/status` requires a signed-in admin user. Configure admin usernames with `ADMIN_USERS`, for example:

```bash
export ADMIN_USERS=admin,gi990422
```

For production WebSocket origin checks, configure the public site origin:

```bash
export ALLOWED_ORIGINS=https://chess.example.com
```

## Stockfish AI

The backend can use Stockfish through the UCI protocol. Resolution order:

1. `STOCKFISH_PATH`
2. `tools/stockfish/stockfish/stockfish-ubuntu-x86-64-avx2`
3. `stockfish` from `PATH`

If Stockfish is unavailable, the server falls back to the built-in legal-move heuristic AI.

On Debian/Ubuntu, a system install can be as simple as:

```bash
sudo apt install stockfish
```

## Game Storage and Cache

Set `DATABASE_URL` to enable PostgreSQL persistence for users and completed games. For local development, `npm run dev:postgres` publishes PostgreSQL on `127.0.0.1:5432`.

```bash
export DATABASE_URL=postgres://chess:chess@localhost:5432/chess
```

The backend auto-creates the `games` table on startup. The matching SQL is in `infra/sql/001_games.sql`.

Set `REDIS_URL` to enable runtime cache records for open room codes and active game state.

```bash
export REDIS_URL=redis://localhost:6379
```

## Docker Topology

```bash
docker compose up --build
```

Services:

- `frontend`: static React build served by Nginx
- `backend`: Go WebSocket and REST API
- `postgres`: durable users/games/moves storage
- `redis`: room-code and active-game runtime cache
- `reverse-proxy`: public entrypoint on ports 8080 and 8443
- `prometheus`: metrics scraper on port 9090
- `grafana`: provisioned observability dashboards on port 3001

After `docker compose up --build`, open:

- App: `http://localhost:8080`
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3001` (`admin` / `chess`)

Prometheus loads alert rules from `infra/prometheus/alerts.yml`, including backend scrape failures and elevated WebSocket disconnect rates.

## Verification

Run the unit/type/build checks:

```bash
npm run lint
npm run build
```

Apply the PostgreSQL schema explicitly when using a database:

```bash
DATABASE_URL=postgres://chess:chess@localhost:5432/chess npm run migrate
```

Run the browser smoke test after installing the Chromium browser once:

```bash
npx playwright install chromium
npm run smoke
```

The smoke tests start the Go backend and Vite frontend, then cover registration, AI analysis, private room join, resignation, and saved game detail.

## Production HTTPS

For a normal public website with a domain and HTTPS, use the production compose file:

```bash
cp .env.prod.example .env.prod
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

See `docs/production-deploy.md` for the full VPS and DNS checklist.

## Demo Checklist

See `docs/portfolio-review.md` for a short reviewer-focused demo path. The core flow is:

1. Register or sign in.
2. Start an AI game or create a private room.
3. Make a legal move and request analysis.
4. Inspect `/health` and `/metrics`.
5. Inspect `/ready` to confirm PostgreSQL and Redis are available.
6. Sign in as a username listed in `ADMIN_USERS` and inspect the System panel for live operations status.
7. Run `npm run loadtest` against the Docker stack.
8. Run `npm run smoke` to show the browser-level login, AI game, and analysis check.
9. Review Nginx, systemd, backup, alerting, and Docker Compose assets.

## Portfolio Narrative

This project is designed to be shown as an end-to-end Linux service, not just a browser game. The chess domain gives a clear interactive product, while the deployment layer demonstrates process supervision, reverse proxying, socket traffic, persistent storage, backups, observability, and host hardening.
