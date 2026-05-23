# Temporary Public Preview

Use this when you want someone on another computer to try the chess app without a VPS or domain.

This is a temporary preview, not production hosting. Your computer must stay on, Docker must keep running, and the terminal running the tunnel must remain open.

## Requirements

- Docker Engine
- Docker Compose plugin
- cloudflared

## Start Preview

```bash
npm run preview:tunnel
```

The command starts the local Docker stack at:

```text
http://localhost:8080
```

Then it opens a Cloudflare quick tunnel and prints a public URL like:

```text
https://example-name.trycloudflare.com
```

Share that HTTPS URL with another person. They can open it from another computer or phone and use the multiplayer chess flow.

## Stop Preview

Stop the tunnel with `Ctrl+C`.

Stop the Docker stack:

```bash
docker compose down
```

## Notes

- The public URL changes when you restart the tunnel.
- If WebSocket connections fail on a restricted network, the script already uses `--protocol http2` to avoid QUIC/UDP issues.
- For a permanent public site, use `docs/production-deploy.md` instead.
