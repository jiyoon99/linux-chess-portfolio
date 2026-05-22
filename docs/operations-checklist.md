# Operations Checklist

Use this before showing or deploying the portfolio.

## Preflight

```bash
npm run lint
npm run build
docker compose config
docker compose up -d --build
```

## Runtime Checks

```bash
curl -s http://localhost:8080/health
curl -s http://localhost:8080/ready
curl -s http://localhost:8080/metrics
npm run loadtest
```

Expected:

- `/health` returns `ok: true`
- `/ready` returns `ok: true`, `db: true`, and `redis: true`
- `/metrics` includes `chess_redis_enabled 1`
- load test returns `failed=0`

## Admin Access

Set admin usernames with `ADMIN_USERS`.

```bash
ADMIN_USERS=admin,gi990422
```

Only those users can call:

```bash
curl -s http://localhost:8080/admin/status
```

## Incident Drill

1. Stop Redis: `docker compose stop redis`
2. Confirm `/ready` returns HTTP 503
3. Start Redis: `docker compose start redis`
4. Confirm `/ready` returns HTTP 200
5. Check backend logs: `docker compose logs --tail=100 backend`
