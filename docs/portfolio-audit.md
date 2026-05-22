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

## Remaining Risks

- Sessions are stored in backend memory, so active logins are lost on process restart and would not work across multiple backend replicas.
- Database schema creation currently happens on startup; a real deployment should use explicit migration tooling.
- Cookie-authenticated POST routes rely on SameSite behavior and do not yet include explicit CSRF tokens.
- Browser coverage currently exercises registration, AI game start, and analysis only.
- The repository does not yet prove a live VPS deployment with a public URL.

## Recommended Next Improvements

1. Add external uptime checks and document the check URL in README.
2. Add Alertmanager config and notification routing for existing Prometheus rules.
3. Add migration tooling for PostgreSQL schema changes.
4. Expand Playwright tests for private room join, resignation, and game history detail.
5. Add screenshots for Grafana, Prometheus alerts, and the admin operations panel.
6. Move session persistence to Redis if multi-instance deployment becomes a goal.
7. Add explicit CSRF protection for authenticated POST routes.
