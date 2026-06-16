#!/usr/bin/env python3
"""Orchestration CLI for the Envoy xDS degradation demo."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent
CONTROLPLANE_URL = "http://localhost:8080"


def compose_cmd() -> list[str]:
    """Return docker compose argv prefix (supports plugin and standalone binary)."""
    for candidate in (["docker", "compose"], ["docker-compose"]):
        try:
            subprocess.run(
                candidate + ["version"],
                cwd=ROOT,
                check=True,
                capture_output=True,
                text=True,
            )
            return candidate
        except (subprocess.CalledProcessError, FileNotFoundError):
            continue
    print("error: need 'docker compose' or 'docker-compose' installed", file=sys.stderr)
    sys.exit(1)


def run(cmd: list[str], *, check: bool = True, capture: bool = False) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        cwd=ROOT,
        check=check,
        text=True,
        capture_output=capture,
    )


def post_config(path: str) -> None:
    url = f"{CONTROLPLANE_URL}{path}"
    req = urllib.request.Request(url, method="POST", data=b"")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            body = resp.read().decode()
            print(f"controlplane: {body}")
    except urllib.error.URLError as exc:
        print(f"failed to reach controlplane at {url}: {exc}", file=sys.stderr)
        sys.exit(1)


def ensure_stack_running() -> None:
    result = run(compose_cmd() + ["ps", "-q", "traffic-client"], check=False, capture=True)
    if not (result.stdout or "").strip():
        print("starting demo stack (docker-compose up -d)...")
        run(compose_cmd() + ["up", "-d"])
        time.sleep(3)
        return
    status = run(compose_cmd() + ["ps", "traffic-client"], check=False, capture=True)
    if "Up" not in (status.stdout or ""):
        print("traffic-client not running; restarting stack...")
        run(compose_cmd() + ["up", "-d", "traffic-client"])
        time.sleep(2)


def send_traffic() -> None:
    ensure_stack_running()
    run(
        compose_cmd()
        + ["exec", "-T", "traffic-client", "/app/start-locust.sh"]
    )
    print("locust traffic started (background in traffic-client container)")


def stop_traffic() -> None:
    ensure_stack_running()
    run(
        compose_cmd() + ["exec", "-T", "traffic-client", "/app/stop-locust.sh"],
        check=False,
    )
    time.sleep(2)
    print("--- last 30 client log lines ---")
    result = run(
        compose_cmd() + ["logs", "--tail", "30", "traffic-client"],
        check=False,
        capture=True,
    )
    print(result.stdout or result.stderr)


def status() -> None:
    run(compose_cmd() + ["ps"], check=False)
    print("--- recent traffic-client logs ---")
    result = run(
        compose_cmd() + ["logs", "--tail", "20", "traffic-client"],
        check=False,
        capture=True,
    )
    print(result.stdout or result.stderr)


def summarize_errors(log_text: str) -> dict:
    summary: dict = {"by_category": {}, "by_status": {}, "total": 0}
    for line in log_text.splitlines():
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            entry = json.loads(line)
        except json.JSONDecodeError:
            continue
        if "category" not in entry:
            continue
        summary["total"] += 1
        cat = entry.get("category", "unknown")
        summary["by_category"][cat] = summary["by_category"].get(cat, 0) + 1
        status = str(entry.get("status", 0))
        summary["by_status"][status] = summary["by_status"].get(status, 0) + 1
    return summary


def run_scenario() -> None:
    ensure_stack_running()
    print("=== run-scenario: push config-1, send traffic, swap to config-2 ===")
    post_config("/config/1")
    time.sleep(3)
    send_traffic()
    print("warming up 10s...")
    time.sleep(10)
    print("pushing config-2 (expect brief degradation)...")
    post_config("/config/2")
    time.sleep(5)
    stop_traffic()
    result = run(
        compose_cmd() + ["logs", "traffic-client"],
        check=False,
        capture=True,
    )
    summary = summarize_errors(result.stdout or "")
    print("=== error summary ===")
    print(json.dumps(summary, indent=2))


def main() -> None:
    parser = argparse.ArgumentParser(description="Envoy xDS degradation demo orchestrator")
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("send-traffic", help="Start traffic from client to sidecar")
    sub.add_parser("stop-traffic", help="Stop traffic and show recent logs")
    sub.add_parser("update-config1", help="Push config-1 via xDS")
    sub.add_parser("update-config2", help="Push config-2 via xDS")
    sub.add_parser("status", help="Show compose status and recent logs")
    sub.add_parser("run-scenario", help="Run full degradation demo scenario")

    args = parser.parse_args()
    commands = {
        "send-traffic": send_traffic,
        "stop-traffic": stop_traffic,
        "update-config1": lambda: post_config("/config/1"),
        "update-config2": lambda: post_config("/config/2"),
        "status": status,
        "run-scenario": run_scenario,
    }
    commands[args.command]()


if __name__ == "__main__":
    main()
