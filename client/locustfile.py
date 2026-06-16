"""Locust load client for the Envoy xDS degradation demo."""

from __future__ import annotations

import json
import os
import time

from locust import HttpUser, constant, events, task

REQUESTS_PER_SEC = float(os.getenv("REQUESTS_PER_SEC", "20"))
WAIT_SECONDS = 1.0 / REQUESTS_PER_SEC if REQUESTS_PER_SEC > 0 else 0.05

_by_category: dict[str, int] = {}
_by_status: dict[str, int] = {}
_total = 0
_recent_errors: list[dict] = []


def categorize(status: int) -> str:
    if 200 <= status < 300:
        return "2xx"
    if 400 <= status < 500:
        return "4xx"
    if 500 <= status < 600:
        return "5xx"
    if status == 0:
        return "conn_error"
    return "other"


def record(entry: dict) -> None:
    global _total
    _total += 1
    cat = entry["category"]
    _by_category[cat] = _by_category.get(cat, 0) + 1
    status = str(entry["status"])
    _by_status[status] = _by_status.get(status, 0) + 1
    if cat != "2xx":
        _recent_errors.append(entry)
        if len(_recent_errors) > 100:
            del _recent_errors[:-100]
    print(json.dumps(entry), flush=True)


@events.request.add_listener
def on_request(
    request_type,
    name,
    response_time,
    response_length,
    response,
    context,
    exception,
    **kwargs,
) -> None:
    ts_ms = int(time.time() * 1000)
    path = name or "/"
    latency_ms = int(response_time)

    if exception is not None:
        record(
            {
                "ts_ms": ts_ms,
                "path": path,
                "status": 0,
                "category": "conn_error",
                "error": str(exception),
                "latency_ms": latency_ms,
            }
        )
        return

    status = response.status_code if response is not None else 0
    category = categorize(status)
    entry = {
        "ts_ms": ts_ms,
        "path": path,
        "status": status,
        "category": category,
        "latency_ms": latency_ms,
    }
    if category != "2xx":
        entry["error"] = getattr(response, "reason", "") or "request failed"
    record(entry)


@events.quitting.add_listener
def on_quitting(environment, **kwargs) -> None:
    summary = {
        "by_category": dict(_by_category),
        "by_status": dict(_by_status),
        "total": _total,
        "recent_errors": _recent_errors[-20:],
    }
    print(f"traffic summary:\n{json.dumps(summary, indent=2)}", flush=True)


class SidecarUser(HttpUser):
    wait_time = constant(WAIT_SECONDS)

    @task
    def test_1(self) -> None:
        self.client.get(
            "/test-1",
            headers={"x-request-id": f"req-{int(time.time() * 1000)}"},
            name="/test-1",
        )

    @task
    def test_2(self) -> None:
        self.client.get(
            "/test-2",
            headers={"x-request-id": f"req-{int(time.time() * 1000)}"},
            name="/test-2",
        )
