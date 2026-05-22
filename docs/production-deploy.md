# Production Deployment

This deploys the chess portfolio as a normal HTTPS site with Caddy, Docker Compose, PostgreSQL, Redis, Prometheus, and Grafana.

## Server Requirements

- Ubuntu 24.04 or similar Linux server
- Public IPv4 address
- DNS A records pointing to the server:
  - `SITE_DOMAIN`
  - `GRAFANA_DOMAIN`
- Ports `80` and `443` open
- Docker Engine and Docker Compose plugin installed

## First Deploy

```bash
git clone <repo-url> chess-portfolio
cd chess-portfolio
cp .env.prod.example .env.prod
```

Edit `.env.prod`:

```bash
nano .env.prod
```

Start the stack:

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

Check status:

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml ps
curl -I https://$SITE_DOMAIN
curl https://$SITE_DOMAIN/health
```

## URLs

- App: `https://$SITE_DOMAIN`
- Grafana: `https://$GRAFANA_DOMAIN`
- Health: `https://$SITE_DOMAIN/health`
- Metrics: `https://$SITE_DOMAIN/metrics`

## Operations

View logs:

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f backend
docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f edge
```

Update after code changes:

```bash
git pull
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

Back up PostgreSQL:

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml exec postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > chess-backup.sql
```

## Notes

- Caddy automatically provisions and renews HTTPS certificates.
- PostgreSQL is not exposed to the public internet in production.
- Prometheus is internal-only; Grafana is exposed through Caddy.
