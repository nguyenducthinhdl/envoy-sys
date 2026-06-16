#!/bin/sh
set -e
HOST="${LOCUST_HOST:-http://app-sidecar:8081}"
RPS="${REQUESTS_PER_SEC:-20}"

pkill -f "locust -f /app/locustfile.py" 2>/dev/null || true
rm -f /tmp/locust.pid

locust -f /app/locustfile.py \
  --headless \
  -u 1 \
  -r 1 \
  --host "$HOST" \
  --run-time 24h \
  >> /proc/1/fd/1 2>&1 &

echo $! > /tmp/locust.pid
echo "locust started host=$HOST rps=$RPS pid=$(cat /tmp/locust.pid)"
