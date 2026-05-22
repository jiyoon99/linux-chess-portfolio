# Improvement Roadmap

This project is intentionally scoped as a portfolio service. The next improvements are prioritized by how much they would increase production realism and interview value.

## Priority 1: Production Readiness

- Add external uptime checks against `/health` from outside the host.
- Ship structured JSON logs to Loki, Elasticsearch, or another log backend.
- Add TLS and real-domain deployment notes from an actual VPS run.

## Priority 2: Test Coverage

- Add integration tests with PostgreSQL and Redis using Docker Compose.
- Add WebSocket protocol tests for invalid move rejection and reconnect behavior.
- Add load-test thresholds in CI or a release checklist.

## Priority 3: Security

- Add Nginx or Caddy rate limits for `/auth/` endpoints.
- Add account lockout or stronger auth throttling metrics.
- Review Content Security Policy for production asset hosting.

## Priority 4: Operations Experience

- Add Grafana panels for HTTP latency, auth failures, and Redis availability.
- Add a restore drill script that verifies backup integrity automatically.
- Add a `make demo` or `task demo` wrapper for reviewer setup.
- Add a screenshot for the authenticated admin operations panel.

## Priority 5: Product Features

- Add spectator mode for active games.
- Add rematch flow and time controls.
- Add stronger Stockfish configuration controls.
- Add user profile pages with historical PGN exports.
- Add mobile layout refinements for smaller screens.
