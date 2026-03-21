#!/usr/bin/env bash
set -euo pipefail

REPO_URL=""
BRANCH="main"
INSTALL_DIR="/opt/rtcm-stream-gateway"
SERVICE_NAME="rtcm-stream-gateway"
APP_USER="rtcmgw"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO_URL="$2"; shift 2 ;;
    --branch)
      BRANCH="$2"; shift 2 ;;
    --dir)
      INSTALL_DIR="$2"; shift 2 ;;
    --service)
      SERVICE_NAME="$2"; shift 2 ;;
    --user)
      APP_USER="$2"; shift 2 ;;
    *)
      echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$REPO_URL" ]]; then
  echo "Usage: sudo bash bootstrap.sh --repo https://github.com/<owner>/<repo>.git [--branch main]" >&2
  exit 1
fi

echo "[1/8] Installing dependencies..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y git golang-go build-essential libpcap-dev ca-certificates

echo "[2/8] Preparing app user..."
id -u "$APP_USER" >/dev/null 2>&1 || useradd -r -s /usr/sbin/nologin "$APP_USER"

echo "[3/8] Cloning/updating repository..."
if [[ -d "$INSTALL_DIR/.git" ]]; then
  git -C "$INSTALL_DIR" fetch --all --prune
  git -C "$INSTALL_DIR" checkout "$BRANCH"
  git -C "$INSTALL_DIR" pull --ff-only origin "$BRANCH"
else
  rm -rf "$INSTALL_DIR"
  git clone --branch "$BRANCH" "$REPO_URL" "$INSTALL_DIR"
fi

echo "[4/8] Building binary..."
cd "$INSTALL_DIR"
go mod tidy
go build -o bin/gateway ./cmd/gateway

echo "[5/8] Preparing env file..."
if [[ ! -f "$INSTALL_DIR/gateway.env" ]]; then
  cp "$INSTALL_DIR/configs/gateway.env.example" "$INSTALL_DIR/gateway.env"
  echo "Created $INSTALL_DIR/gateway.env (please edit values)."
fi

chown -R "$APP_USER:$APP_USER" "$INSTALL_DIR"
chmod 600 "$INSTALL_DIR/gateway.env" || true

echo "[6/8] Installing systemd service..."
cat >/etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=RTCM stream gateway
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_USER}
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=${INSTALL_DIR}/gateway.env
ExecStart=${INSTALL_DIR}/bin/gateway
Restart=always
RestartSec=2
AmbientCapabilities=CAP_NET_RAW CAP_NET_ADMIN
CapabilityBoundingSet=CAP_NET_RAW CAP_NET_ADMIN
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=${INSTALL_DIR}
CPUQuota=35%
MemoryMax=700M

[Install]
WantedBy=multi-user.target
EOF

echo "[7/8] Enabling service..."
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"

echo "[8/8] Status"
systemctl --no-pager --full status "${SERVICE_NAME}" || true

echo
echo "Done. Next steps:"
echo "1) Edit ${INSTALL_DIR}/gateway.env with real CASTER_HOST/PORT/PASS and MOUNT_PREFIX"
echo "2) Restart service: sudo systemctl restart ${SERVICE_NAME}"
echo "3) Follow logs: sudo journalctl -u ${SERVICE_NAME} -f"
