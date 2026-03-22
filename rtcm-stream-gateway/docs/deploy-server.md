# RTCM Stream Gateway - Server Deployment Guide

## Overview

Hướng dẫn deploy RTCM Stream Gateway lên production server với systemd và reverse proxy.

## Server Requirements

- **OS**: Ubuntu 20.04+ / Debian 11+
- **RAM**: 512MB minimum (1GB recommended)
- **CPU**: 1 vCPU minimum
- **Disk**: 5GB
- **Network**: Public IP hoặc port forwarding

## Deployment Steps

### 1. Server Preparation

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install dependencies
sudo apt install -y curl wget git nginx certbot python3-certbot-nginx ufw

# Create application user
sudo useradd -m -s /bin/bash rtcm
sudo mkdir -p /opt/rtcm-gateway
sudo chown rtcm:rtcm /opt/rtcm-gateway
```

### 2. Build Binary

**Option A: Build trên server**
```bash
# Install Go
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Clone/build
git clone https://github.com/Nguyen15idhue/rtcmrl.git
cd rtcmrl/rtcm-stream-gateway
go build -o /opt/rtcm-gateway/gateway ./cmd/gateway
```

**Option B: Copy binary từ local**
```bash
# Trên local machine
cd rtcm-stream-gateway
GOOS=linux GOARCH=amd64 go build -o bin/gateway ./cmd/gateway

# Copy to server
scp bin/gateway user@server:/tmp/gateway
ssh user@server "sudo mv /tmp/gateway /opt/rtcm-gateway/gateway && sudo chown rtcm:rtcm /opt/rtcm-gateway/gateway"
```

### 3. Create Config File

```bash
sudo -u rtcm nano /opt/rtcm-gateway/config.yaml
```

```yaml
capture:
  device: "any"
  listen_port: 12101
  snap_len: 1024
  buffer_mb: 8

worker:
  queue_size: 8192
  min: 4
  max: 16
  auto_scale: true

caster:
  host: "your-caster.example.com"
  port: 2101
  mount_prefix: "STN"
  user: "your-username"
  password: "your-password"

web:
  port: 8080
  metrics_port: 6060

logging:
  level: "info"
```

### 4. Create Systemd Service

```bash
sudo nano /etc/systemd/system/rtcm-gateway.service
```

```ini
[Unit]
Description=RTCM Stream Gateway
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=rtcm
Group=rtcm
WorkingDirectory=/opt/rtcm-gateway
ExecStart=/opt/rtcm-gateway/gateway
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Environment variables
EnvironmentFile=/opt/rtcm-gateway/env

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/rtcm-gateway/logs
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Tạo environment file:

```bash
sudo -u rtcm nano /opt/rtcm-gateway/env
```

```
MODE=tcp
CASTER_HOST=your-caster.example.com
CASTER_PORT=2101
CASTER_USER=your-username
CASTER_PASS=your-password
LISTEN_PORT=12101
```

### 5. Setup Logging Directory

```bash
sudo -u rtcm mkdir -p /opt/rtcm-gateway/logs
sudo chown -R rtcm:rtcm /opt/rtcm-gateway
```

### 6. Start Service

```bash
sudo systemctl daemon-reload
sudo systemctl enable rtcm-gateway
sudo systemctl start rtcm-gateway

# Check status
sudo systemctl status rtcm-gateway
sudo journalctl -u rtcm-gateway -f
```

### 7. Firewall Setup

```bash
# RTCM TCP input (từ GPS receivers)
sudo ufw allow 12101/tcp

# HTTP API (cho admin)
sudo ufw allow 8080/tcp

# Prometheus metrics
sudo ufw allow 6060/tcp

# Enable firewall
sudo ufw enable
```

### 8. Nginx Reverse Proxy (Optional)

```bash
sudo nano /etc/nginx/sites-available/rtcm-gateway
```

```nginx
server {
    listen 80;
    server_name rtcm-api.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 60s;
    }

    location /metrics {
        proxy_pass http://127.0.0.1:6060;
        proxy_set_header Host $host;
    }
}
```

Enable site:

```bash
sudo ln -s /etc/nginx/sites-available/rtcm-gateway /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### 9. SSL Certificate (Let's Encrypt)

```bash
sudo certbot --nginx -d rtcm-api.example.com
sudo systemctl reload nginx
```

## Docker Deployment (Alternative)

### 1. Install Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

### 2. Create docker-compose.yml

```bash
sudo mkdir -p /opt/rtcm-gateway
sudo nano /opt/rtcm-gateway/docker-compose.yml
```

```yaml
version: '3.8'

services:
  rtcm-gateway:
    image: rtcm-gateway:latest
    container_name: rtcm-gateway
    restart: always
    ports:
      - "12101:12101/tcp"
      - "8080:8080"
      - "6060:6060"
    environment:
      - MODE=tcp
      - CASTER_HOST=your-caster.example.com
      - CASTER_PORT=2101
      - CASTER_USER=your-username
      - CASTER_PASS=your-password
      - LISTEN_PORT=12101
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./logs:/app/logs
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

networks:
  default:
    name: rtcm-network
```

### 3. Deploy

```bash
cd /opt/rtcm-gateway

# Pull/build image
docker build -t rtcm-gateway:latest .

# Start
docker-compose up -d

# Check logs
docker-compose logs -f
```

## Monitoring

### Check Service Status

```bash
# Systemd
sudo systemctl status rtcm-gateway

# Docker
docker ps
docker-compose ps
```

### View Logs

```bash
# Systemd
sudo journalctl -u rtcm-gateway -n 100 --no-pager

# Docker
docker-compose logs --tail=100
```

### Metrics

```bash
# Prometheus metrics
curl http://localhost:6060/metrics

# Stats API
curl http://localhost:8080/stats | jq
```

### Health Check

```bash
curl http://localhost:8080/health
```

## Update Process

### Systemd

```bash
# Stop service
sudo systemctl stop rtcm-gateway

# Backup old binary
sudo cp /opt/rtcm-gateway/gateway /opt/rtcm-gateway/gateway.bak

# Update binary
# (copy new binary or git pull & rebuild)

# Start service
sudo systemctl start rtcm-gateway

# Verify
sudo systemctl status rtcm-gateway
```

### Docker

```bash
cd /opt/rtcm-gateway

# Pull latest
docker-compose pull

# Restart
docker-compose up -d

# Verify
docker-compose ps
```

## Troubleshooting

### Service won't start

```bash
# Check logs
sudo journalctl -u rtcm-gateway -n 50

# Check config syntax
/opt/rtcm-gateway/gateway --validate 2>&1

# Check port availability
sudo ss -tlnp | grep 12101
```

### Caster connection fails

```bash
# Test caster connectivity
nc -zv caster.example.com 2101

# Check credentials
sudo nano /opt/rtcm-gateway/env
sudo systemctl restart rtcm-gateway
```

### GPS receivers can't connect

```bash
# Check firewall
sudo ufw status

# Check if port is listening
sudo ss -tlnp | grep 12101

# Test from another machine
nc -zv server-ip 12101
```

## Backup

```bash
# Backup config
sudo cp -r /opt/rtcm-gateway/config.yaml /backup/

# Backup logs
sudo tar -czf /backup/rtcm-logs-$(date +%Y%m%d).tar.gz /opt/rtcm-gateway/logs
```

## Security Checklist

- [ ] Change default caster password
- [ ] Enable firewall (ufw)
- [ ] Setup SSL/TLS via Let's Encrypt
- [ ] Restrict port 8080/6060 to admin IPs
- [ ] Use non-root user (rtcm)
- [ ] Enable fail2ban
- [ ] Regular security updates: `sudo apt update && sudo apt upgrade`
