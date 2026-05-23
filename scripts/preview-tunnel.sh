#!/usr/bin/env bash
set -euo pipefail

APP_URL="http://localhost:8080"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required to run the preview stack."
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose plugin is required."
  exit 1
fi

if ! command -v cloudflared >/dev/null 2>&1; then
  echo "cloudflared is required to create a temporary public URL."
  echo "Install it from: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"
  exit 1
fi

echo "Starting local chess stack on ${APP_URL}..."
docker compose up -d --build reverse-proxy backend frontend postgres redis

echo "Waiting for backend health..."
for attempt in {1..30}; do
  if curl -fsS "${APP_URL}/health" >/dev/null 2>&1; then
    echo "Local app is ready: ${APP_URL}"
    break
  fi

  if [ "${attempt}" -eq 30 ]; then
    echo "Backend did not become healthy. Check logs with: docker compose logs --tail=100 backend"
    exit 1
  fi

  sleep 2
done

echo "Opening temporary public tunnel. Share the https://*.trycloudflare.com URL."
echo "Keep this terminal open while other computers are connected."
cloudflared tunnel --protocol http2 --url "${APP_URL}"
