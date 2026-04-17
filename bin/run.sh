#!/usr/bin/env bash
set -euo pipefail

echo "[run.sh] Starting service"

export APP_PORT="${APP_PORT:-8080}"

echo "[run.sh] Running DB migrations"
goose -dir ./db/migrations postgres "${DATABASE_URL}" up

echo "[run.sh] Starting Caddy"
caddy run --config /etc/caddy/Caddyfile &

echo "[run.sh] Starting Go app"
PORT="${APP_PORT}" exec /app/bin/app
