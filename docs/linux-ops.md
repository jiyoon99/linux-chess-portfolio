# Linux Operations Notes

## Host Baseline

- Create a dedicated `chess` Unix user for the service.
- Run application code from `/opt/linux-chess`.
- Keep mutable state under `/var/lib/linux-chess`.
- Send process logs to journald through systemd.
- Expose only SSH, HTTP, and HTTPS on the host firewall.

## Nginx

Nginx terminates public traffic and forwards:

- `/` to the frontend container
- `/ws` to the backend WebSocket server
- `/auth/` to backend account endpoints
- `/health` to the backend health endpoint
- `/metrics` to the backend Prometheus text endpoint

Important WebSocket headers:

```nginx
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection $connection_upgrade;
proxy_http_version 1.1;
```

## systemd Hardening

The sample unit uses:

- `Restart=on-failure` for process recovery
- `NoNewPrivileges=true` to block privilege escalation
- `PrivateTmp=true` for isolated temporary storage
- `ProtectSystem=strict` to make system paths read-only
- `ReadWritePaths=/var/lib/linux-chess` for explicit writable state

## Backup

`infra/scripts/backup-postgres.sh` creates compressed PostgreSQL dumps and deletes backups older than 14 days.

Recommended cron:

```cron
15 3 * * * /opt/linux-chess/infra/scripts/backup-postgres.sh
```

## Observability

The backend exposes a Prometheus-compatible `/metrics` endpoint with:

- `chess_active_connections`
- `chess_total_connections`
- `chess_waiting_players`
- `chess_open_rooms`
- `chess_active_games`
- `chess_total_moves`
- `chess_completed_games`
- `chess_disconnects`

Docker Compose includes:

- Prometheus configured by `infra/prometheus/prometheus.yml`
- Alert rules configured by `infra/prometheus/alerts.yml`
- Grafana datasource provisioning in `infra/grafana/provisioning/datasources/prometheus.yml`
- Grafana dashboard provisioning in `infra/grafana/dashboards/chess-overview.json`

Local URLs:

- Application through Nginx: `http://localhost:8080`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3001` (`admin` / `chess`)

Useful Prometheus queries:

- `chess_active_connections`
- `rate(chess_total_moves[5m])`
- `chess_completed_games`
- `chess_disconnects`

Recommended next steps:

- Use journald or Loki for structured logs.

Alert rules:

- `ChessBackendScrapeDown`: backend scrape failure for 2 minutes.
- `ChessHighDisconnectRate`: WebSocket disconnect rate above 0.2 per second for 5 minutes.

## Logging

The backend emits structured JSON logs for operational events and service failures:

- `server_start`
- `http_request`
- `panic_recovered`
- `register_failed`
- `persist_game_failed`
- `redis_game_cache_failed`

Example:

```bash
docker compose logs -f backend
```

Use `docs/incident-runbook.md` for triage commands and common failure modes.

## Security Controls

- HttpOnly cookie sessions
- PBKDF2-SHA256 password hashing
- Auth endpoint rate limiting
- Panic recovery middleware
- Security headers:
  - `X-Content-Type-Options`
  - `X-Frame-Options`
  - `Referrer-Policy`
  - `Permissions-Policy`
  - `Content-Security-Policy`

## Security Roadmap

- Add TLS with Certbot or Caddy.
- Add Nginx edge rate limits for `/auth/`.
- Use fail2ban for SSH and Nginx abuse patterns.
- Move secrets to environment files owned by root with `0600` permissions.
