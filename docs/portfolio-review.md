# Portfolio Review Guide

Use this checklist when presenting the project or reviewing it before publishing.

## Demo Path

1. Open the React client and register a user.
2. Start an AI game and make a legal move.
3. Run position analysis and show the selected engine.
4. Resign or finish the game and show recent history plus profile stats.
5. Create a private room and show the room code flow.
6. Open `/health` to show service status.
7. Open `/metrics` to show operational counters.
8. Open Prometheus and query `chess_active_connections`.
9. Open Grafana and show the Chess Service Overview dashboard.
10. Show alert rules for backend scrape failures and WebSocket disconnect spikes.
11. Run `npm run smoke` to demonstrate the automated login, AI game, and analysis path.
12. Show Docker Compose, Nginx, systemd, backup, and SQL files.

## What To Emphasize

- The chess server validates moves server-side rather than trusting the browser.
- WebSocket traffic is reverse proxied through Nginx.
- Users authenticate with HttpOnly cookie sessions.
- Auth endpoints include a small server-side rate limit.
- Completed games are linked back to authenticated users.
- User profiles show total games, W-D-L, and win rate.
- Completed games can persist to PostgreSQL when `DATABASE_URL` is configured.
- The service exposes Prometheus-compatible operational metrics.
- Docker Compose includes Prometheus scraping and a provisioned Grafana dashboard.
- The repository includes Linux deployment assets, not just application code.
- Prometheus loads alert rules from `infra/prometheus/alerts.yml`.
- Playwright smoke tests cover registration, AI game start, and analysis.
- Backend application logs are structured JSON events.

## Remaining High-Impact Improvements

- Add external uptime checks against `/health`.
- Ship JSON logs to Loki or another log backend.
