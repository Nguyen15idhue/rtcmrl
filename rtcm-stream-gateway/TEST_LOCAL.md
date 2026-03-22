# Local Testing Guide

No real caster or NTRIP source needed. Everything runs locally.

## Prerequisites

```bash
# Python 3 for test scripts
python3 --version

# Go 1.22+ for building
go version
```

## Architecture: What We're Testing

```
┌──────────────┐    TCP    ┌─────────────────────┐   NTRIP   ┌─────────────────┐
│ rtcm_generator │ ────────▶ │ rtcm-stream-gateway │ ────────▶ │ mock_caster.py │
│  (fake RTCM)   │  port    │   (TCP mode)         │  port    │ (fake caster B) │
└──────────────┘  12101    │                     │  2101     └─────────────────┘
                            │  - Web UI :8080     │
                            │  - Metrics :6060    │
                            │  - Worker Pool      │
                            └─────────────────────┘
```

## Step 1: Build the Gateway

```bash
cd rtcm-stream-gateway

# Build Go binary
go mod tidy
go build -o bin/gateway ./cmd/gateway

# Build React dashboard (optional, embedded fallback included)
cd frontend
npm install
npm run build
cd ..
```

## Step 2: Terminal 1 - Start Mock Caster B (port 2101)

```bash
python3 scripts/mock_caster.py
```

Expected output:
```
[MockCaster] Listening on 0.0.0.0:2101
[MockCaster] Mountpoints available:
  - /STN_0001
  - /STN_0002
  - /STN_0003
  - /TEST
```

## Step 3: Terminal 2 - Start the Gateway (TCP mode)

```bash
cd rtcm-stream-gateway

CASTER_HOST=127.0.0.1 \
CASTER_PORT=2101 \
CASTER_PASS=test123 \
MOUNT_PREFIX=STN \
LISTEN_PORT=12101 \
WEB_PORT=8080 \
METRICS_PORT=6060 \
WORKER_MIN=2 \
WORKER_MAX=8 \
AUTO_SCALE=true \
MODE=tcp \
./bin/gateway
```

Or using env file:
```bash
cp .env.example .env
# Edit .env, set CASTER_HOST=127.0.0.1, CASTER_PORT=2101, CASTER_PASS=test123
# Set MODE=tcp
./bin/gateway
```

Expected output:
```
[BOOT] rtcm-stream-gateway v2.0.0 starting
[BOOT] mode: tcp
[BOOT] caster: 127.0.0.1:2101 prefix=STN
[BOOT] web: :8080 metrics: :6060
[BOOT] workers: min=2 max=8 auto_scale=true
[BOOT] capture: TCP mode on port 12101 (no libpcap)
[WEB] HTTP server starting on :8080
[MET] Metrics server starting on :6060
```

## Step 4: Terminal 3 - Generate Fake RTCM Data (port 12101)

```bash
python3 scripts/test_local.py
```

Expected output:
```
Connecting to gateway at 127.0.0.1:12101...
Connected! Sending RTCM test data...
```

## Step 5: Watch the Gateway Logs

You should see station connections:
```
[NEW] station=1 variant=... mount=STN_0001
[NEW] station=2 variant=... mount=STN_0002
[NEW] station=3 variant=... mount=STN_0003
[STAT] sources=3 stations=3 forwarded=... unknown=0 ambiguous=0 drops=0
```

And on the Mock Caster side:
```
[MockCaster] Client connected: 127.0.0.1:xxxx -> /STN_0001
[MockCaster] Client connected: 127.0.0.1:xxxx -> /STN_0002
[MockCaster] Client connected: 127.0.0.1:xxxx -> /STN_0003
[MockCaster] /STN_0001: 100 frames, 8.5 KB
```

## Step 6: Test Web Dashboard

Open browser: **http://localhost:8080**

You should see:
- Dashboard with live stats (stations, workers, throughput)
- Stations page with all 5 test stations
- Config page to adjust workers and auto-scale

Test auto-scale: Change workers via API:
```bash
# Disable auto-scale
curl -X POST http://localhost:8080/api/v1/workers/auto-scale \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'

# Set workers to 6
curl -X POST http://localhost:8080/api/v1/workers \
  -H "Content-Type: application/json" \
  -d '{"count": 6}'

# Update runtime config
curl -X POST http://localhost:8080/api/v1/config \
  -H "Content-Type: application/json" \
  -d '{"runtime": {"source_idle_sec": 120}}'
```

## Step 7: Test Prometheus Metrics

Open: **http://localhost:6060/metrics**

Search for `rtcm_` metrics:
- `rtcm_stations_active` - should be ~5
- `rtcm_workers_active` - should be between min/max
- `rtcm_frames_forwarded_total` - increasing
- `rtcm_bytes_forwarded_total` - increasing
- `rtcm_queue_depth` - should stay low

## Docker Compose (Full Stack with Prometheus + Grafana)

```bash
# Copy and edit env
cp .env.example .env

# Edit .env:
# CASTER_HOST=127.0.0.1
# CASTER_PORT=2101
# CASTER_PASS=test123
# MODE=tcp

# Start all (gateway + prometheus + grafana)
docker compose up -d

# Gateway API: http://localhost:8080
# Grafana: http://localhost:3000 (admin/admin123)
# Prometheus: http://localhost:9090
```

For Docker, you need Npcap on Windows or libpcap on Linux. Since we're using TCP mode, Docker gateway also works without pcap.

## Quick Test Without Python Scripts

Just use netcat to send raw bytes to port 12101:

```bash
# Send fake RTCM preamble repeatedly
while true; do
  echo -ne '\xd3\x00\x00' | nc 127.0.0.1 12101
  sleep 0.1
done
```

## Troubleshooting

### Gateway won't bind port 12101
```bash
# Check port usage
netstat -an | grep 12101
# or
ss -tlnp | grep 12101
```

### Mock caster won't connect
Make sure mock_caster.py is running BEFORE starting the gateway.

### No stations showing in web UI
- Check gateway logs for `[NEW]` entries
- Make sure test_local.py is running and connected
- Check the Mock Caster terminal - did connections arrive?

### pcap not available on Windows
Always use `MODE=tcp` - this bypasses libpcap entirely.
