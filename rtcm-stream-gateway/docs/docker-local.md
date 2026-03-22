# RTCM Stream Gateway - Docker Local Guide

## Prerequisites

- Docker Desktop (Windows) hoặc Docker Engine (Linux)
- Docker Compose

## Quick Start

### 1. Build Docker Image

```bash
cd rtcm-stream-gateway
docker build -t rtcm-gateway:latest .
```

### 2. Run Gateway (TCP Mode)

```bash
docker run -d \
  --name rtcm-gateway \
  -p 8080:8080 \
  -p 6060:6060 \
  -p 12101:12101/tcp \
  -e MODE=tcp \
  -e CASTER_HOST=test \
  -e CASTER_PORT=2101 \
  -e CASTER_USER=gateway \
  -e CASTER_PASS=password \
  -e LISTEN_PORT=12101 \
  rtcm-gateway:latest
```

### 3. Run Test Sender

```bash
docker run -d \
  --name rtcm-sender \
  --network host \
  rtcm-gateway:latest \
  /sender
```

### 4. Verify

```bash
# Check gateway logs
docker logs -f rtcm-gateway

# Check stats
curl http://localhost:8080/stats
```

## Docker Compose Setup

Tạo file `docker-compose.yml`:

```yaml
version: '3.8'

services:
  rtcm-gateway:
    build: .
    image: rtcm-gateway:latest
    container_name: rtcm-gateway
    ports:
      - "8080:8080"      # HTTP API
      - "6060:6060"      # Prometheus metrics
      - "12101:12101/tcp" # RTCM TCP input
    environment:
      - MODE=tcp
      - CASTER_HOST=test
      - CASTER_PORT=2101
      - CASTER_USER=gateway
      - CASTER_PASS=password
      - LISTEN_PORT=12101
      - QUEUE_SIZE=4096
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

Run:

```bash
docker-compose up -d
docker-compose logs -f
```

## Connect GPS Receiver

Kết nối GPS receiver đến gateway qua TCP:

```bash
# Netro+ or similar
nc <container-ip> 12101

# Or via Docker host network (Windows/Mac)
nc host.docker.internal 12101
```

## Testing with Sender Container

Build và chạy sender:

```bash
# Build sender image
docker build -t rtcm-sender:latest . --target sender

# Run sender
docker run --rm \
  --network host \
  rtcm-sender:latest
```

## Environment Variables Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `MODE` | `pcap` | `tcp` hoặc `pcap` |
| `CASTER_HOST` | `127.0.0.1` | NTRIP caster host |
| `CASTER_PORT` | `2101` | NTRIP caster port |
| `CASTER_USER` | `gateway` | NTRIP username |
| `CASTER_PASS` | `password` | NTRIP password |
| `LISTEN_PORT` | `12101` | TCP/pcap listen port |
| `QUEUE_SIZE` | `4096` | Frame queue size |
| `WEB_PORT` | `8080` | HTTP API port |
| `METRICS_PORT` | `6060` | Prometheus metrics port |

## Volume Mount (Config File)

Mount config file từ host:

```bash
docker run -d \
  --name rtcm-gateway \
  -p 8080:8080 \
  -p 6060:6060 \
  -p 12101:12101/tcp \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  rtcm-gateway:latest
```

## Health Check

```bash
# HTTP API health
curl http://localhost:8080/health

# Prometheus metrics
curl http://localhost:6060/metrics
```

## Cleanup

```bash
# Stop containers
docker-compose down

# Remove images
docker rmi rtcm-gateway:latest rtcm-sender:latest

# Remove volumes
docker-compose down -v
```

## Troubleshooting

### Container can't bind port

Port 12101 có thể đã được sử dụng. Kiểm tra:

```bash
# Windows
netstat -an | findstr 12101

# Linux
ss -tlnp | grep 12101
```

### GPS receiver not connecting

Đảm bảo container port được expose đúng:

```bash
docker port rtcm-gateway
```

### Logs not showing

```bash
docker logs rtcm-gateway
docker-compose logs rtcm-gateway
```

## Multi-stage Build Notes

Dockerfile sử dụng multi-stage build để tạo 2 binaries:
- `gateway` - Main gateway
- `sender` - Test sender

Build riêng lẻ:

```bash
# Gateway only
docker build --target gateway -t rtcm-gateway:latest .

# Sender only
docker build --target sender -t rtcm-sender:latest .
```
