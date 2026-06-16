#!/bin/sh
set -e

echo "Starting backend on ${BACKEND_ADDR:-127.0.0.1:8080}..."
/backend &
BACKEND_PID=$!

echo "Waiting for backend to be ready..."
for i in $(seq 1 30); do
  if curl -sf "http://${BACKEND_ADDR:-127.0.0.1:8080}/health" >/dev/null 2>&1; then
    echo "Backend is ready."
    break
  fi
  sleep 0.2
done

echo "Starting Envoy sidecar on :8081..."
exec /usr/local/bin/envoy -c /etc/envoy/bootstrap.yaml --log-level info
