# Envoy xDS Config Update Degradation Demo

Demonstrates brief request degradation when an Envoy sidecar receives updated configuration from an xDS control plane.

## Architecture

- **controlplane** — Go xDS server (gRPC `:18000`) + HTTP API (`:8080`) + web GUI (`:8022`)
- **app-sidecar** — Go backend on `:8080` + Envoy sidecar on `:8081` (host-exposed)
- **traffic-client** — Locust load generator hitting `app-sidecar:8081`

Routes: `/test-1`, `/test-2`  
Security: 10 pass-through HTTP filters (ext_authz, jwt_authn, rbac, cors, local_ratelimit, buffer, gzip, csrf, header_to_metadata, fault)

## Quick start

```bash
docker-compose up --build -d    # starts all 3 services including Locust
```

| UI | URL |
|----|-----|
| **Locust** (load test) | http://localhost:8089 |
| **Control plane GUI** | http://localhost:8022 |
| **Envoy sidecar** | http://localhost:8081/test-1 |

Push config from the GUI or CLI, then start a swarm in Locust (e.g. 1 user, spawn rate 1).

```bash
curl -X POST http://localhost:8080/config/1   # or use GUI :8022
```

Manual scenario with `demo.py`:

```bash
python3 demo.py update-config1
python3 demo.py send-traffic
sleep 10
python3 demo.py update-config2   # observe 503/504/conn_error spike
python3 demo.py stop-traffic
```

Or run the full scenario:

```bash
docker-compose up --build -d
python3 demo.py run-scenario
```

## Control plane GUI

Open http://localhost:8022 for the web UI (HTML + pure JS):

- **Config 1/2: push** — push snapshot via xDS
- **Config 1/2: tree** — push snapshot, fetch live config dump, render parsed tree (`envoyviz`)
- **Current: tree** — tree view of the sidecar's current config (no push)
- **Show Envoy config dump** — raw JSON admin dump

## demo.py commands

| Command | Description |
|---------|-------------|
| `send-traffic` | Start background traffic to the sidecar |
| `stop-traffic` | Stop traffic and print recent logs + summary |
| `update-config1` | Push config-1 snapshot via xDS |
| `update-config2` | Push config-2 snapshot via xDS |
| `status` | Show service status and recent client logs |
| `run-scenario` | Automated demo: config1 → traffic → config2 → stop |

## Traffic client (Locust)

Runs headless Locust inside the `traffic-client` container (~20 req/s by default). Each request is logged as JSONL:

```json
{"ts_ms":1717689600123,"path":"/test-1","status":503,"category":"5xx","error":"Service Unavailable","latency_ms":12}
```

Categories: `2xx`, `4xx`, `5xx`, `conn_error`, `other`

View the Locust web UI (start a swarm from the browser):

```bash
open http://localhost:8089
```

Live JSONL request logs:

```bash
docker-compose logs -f traffic-client
```

Optional: run Locust locally against the sidecar (two separate commands):

```bash
python -m pip install locust
python -m locust -f client/locustfile.py --headless -u 1 -r 1 --host http://localhost:8081 --run-time 1m
```

Ensure config is pushed first: `curl -X POST http://localhost:8080/config/1`

## Config differences

| | Config-1 | Config-2 |
|---|----------|----------|
| Cluster | `backend_v1` | `backend_v2` |
| Header | `x-demo-config: config-1` | `x-demo-config: config-2` |
| Filter order | baseline | reordered (triggers listener rebuild) |
| Timeouts | 15s route / 5s connect | 5s route / 2s connect |

## Manual checks


```bash
curl -s http://localhost:8081/test-1
curl -X POST http://localhost:8080/config/2
docker-compose logs -f traffic-client
```

```bash
docker-compose exec -T app-sidecar curl -s http://127.0.0.1:9901/config_dump 2>&1
```

Write envoy config dump
```bash
docker-compose exec -T app-sidecar curl -s 'http://127.0.0.1:9901/config_dump?include_eds=1' \
  | python3 -m json.tool > sidecar/config-dump.json
```