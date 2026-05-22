# Portfolio Audit

This audit tracks review findings for the employment portfolio version of the project.

## Current Verdict

The repository is strong enough to present as a backend and Linux operations portfolio. It shows an interactive product plus deployment, monitoring, CI, smoke tests, and operations documentation. The remaining work is mostly production hardening, broader automated coverage, and live deployment proof.

## Strengths

- Clear product surface: realtime chess, AI game, private rooms, history, and analysis.
- Backend ownership of game rules instead of trusting the browser.
- Practical service dependencies: PostgreSQL for durable records and Redis for runtime state.
- Operations surface: health, readiness, metrics, Grafana dashboard, Prometheus alert rules, backup script, systemd unit, Nginx/Caddy configs.
- Quality gates: Go tests, TypeScript checks, production build, Playwright smoke test, GitHub Actions workflow.
- Reviewer-friendly documentation: README screenshot, demo checklist, incident runbook, production deployment notes, roadmap.

## Findings Fixed During Audit

- WebSocket origin policy was previously permissive. It now allows same-host requests, loopback development requests, and configured production origins through `ALLOWED_ORIGINS`.
- Production Caddy routing documented `/metrics`, but the Caddyfile did not proxy it. `/metrics` is now routed to the backend.
- Cookie-authenticated logout now requires a double-submit CSRF token.
- Sessions are cached in Redis when Redis is configured, while retaining in-memory fallback for local development.
- PostgreSQL schema setup is available as an explicit `npm run migrate` command.
- Prometheus alert rules are wired to an Alertmanager example service.
- Playwright smoke tests now cover registration, AI analysis, private room join, resignation, and saved game detail.
- Prometheus, Grafana, and Alertmanager screenshots are captured under `docs/assets/observability/`.

## Remaining Risks

- Session persistence falls back to backend memory when Redis is unavailable.
- Database schema is still auto-created on startup for convenience, even though explicit migration tooling now exists.
- CSRF coverage currently protects the cookie-authenticated POST route; future authenticated POST routes should use the same guard.
- The repository does not yet prove a live VPS deployment with a public URL.

## Recommended Next Improvements

1. Add external uptime checks and document the check URL in README.
2. Add Alertmanager config and notification routing for existing Prometheus rules.
3. Add a screenshot for the authenticated admin operations panel.
4. Add integration tests with real PostgreSQL and Redis containers.
5. Add load-test thresholds to CI or the release checklist.
6. Add CSRF guard reuse tests when new authenticated POST routes are introduced.
