# Load Test

This project includes a lightweight WebSocket load test that exercises the real game loop:

1. Connect to `/ws`
2. Start an AI game
3. Play `e2e4`
4. Wait for the AI response
5. Resign cleanly

Run against the local Docker stack:

```bash
docker compose up -d --build
npm run loadtest
```

Run a larger test:

```bash
go run ./backend/cmd/loadtest -url ws://localhost:8080/ws -clients 100 -timeout 45s
```

Expected output:

```text
clients=20 ok=20 failed=0 duration=...
```

If failures appear, check:

```bash
docker compose logs --tail=100 backend
curl -s http://localhost:8080/health
curl -s http://localhost:8080/metrics
```
