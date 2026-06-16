#!/bin/sh
if [ -f /tmp/locust.pid ]; then
  kill "$(cat /tmp/locust.pid)" 2>/dev/null || true
fi
pkill -f "locust -f /app/locustfile.py" 2>/dev/null || true
echo "locust stopped"
