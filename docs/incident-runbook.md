# Incident Runbook

Use this when the public site, game server, database, or observability stack is unhealthy.

## Quick Triage

```bash
docker compose ps
curl -I http://localhost:8080
curl http://localhost:8080/health
curl http://localhost:8080/metrics
```

Expected health response:

```json
{"ok":true,"db":true}
```

## Logs

Backend logs are structured JSON. Useful commands:

```bash
docker compose logs --tail=100 backend
docker compose logs -f backend
docker compose logs --tail=100 reverse-proxy
```

Look for:

- `event=server_start`
- `event=http_request`
- `event=panic_recovered`
- non-2xx/3xx HTTP status codes

## Common Problems

### Site Does Not Load

1. Check reverse proxy:
   ```bash
   docker compose ps reverse-proxy frontend
   docker compose logs --tail=100 reverse-proxy
   ```
2. Check frontend container:
   ```bash
   docker compose logs --tail=100 frontend
   ```
3. Verify local response:
   ```bash
   curl -I http://localhost:8080
   ```

### Login Or Game History Fails

1. Check backend health:
   ```bash
   curl http://localhost:8080/health
   ```
2. If `db:false`, check PostgreSQL:
   ```bash
   docker compose ps postgres
   docker compose logs --tail=100 postgres
   ```
3. Restart backend after database recovery:
   ```bash
   docker compose restart backend
   ```

### WebSocket Match Fails

1. Confirm `/ws` is proxied:
   ```bash
   docker compose logs --tail=100 reverse-proxy
   docker compose logs --tail=100 backend
   ```
2. Check active connection metrics:
   ```bash
   curl -s http://localhost:8080/metrics | grep chess_active_connections
   ```

### Cloudflare Tunnel Link Fails

Quick tunnels are temporary. If the link returns DNS errors or `Tunnel not found`, start a new tunnel:

```bash
cloudflared tunnel --protocol http2 --url http://localhost:8080
```

Use `--protocol http2` on networks that block QUIC/UDP.

## Security Checks

```bash
curl -I http://localhost:8080 | grep -E 'X-Content-Type-Options|X-Frame-Options|Referrer-Policy|Content-Security-Policy'
```

Authentication endpoints include backend rate limiting. A burst of failed login attempts should return `429`.

