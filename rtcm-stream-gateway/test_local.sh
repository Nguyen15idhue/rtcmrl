#!/bin/bash
# Test local: runs gateway + sender, verifies frame extraction works

GATEWAY_DIR="rtcm-stream-gateway"
PORT=12101

echo "[1/4] Killing any existing processes..."
taskkill //F //IM gateway 2>/dev/null
sleep 1

echo "[2/4] Starting gateway in test mode..."
cd "$GATEWAY_DIR"
export CASTER_HOST=test
export CASTER_PASS=test
export CAPTURE_MODE=tcp
export LISTEN_PORT=$PORT
./bin/gateway > ../gateway_out.log 2>&1 &
GATEWAY_PID=$!
echo "Gateway PID: $GATEWAY_PID"
cd ..

sleep 3

echo "[3/4] Checking gateway startup..."
tail -20 gateway_out.log
echo "---"

echo "[4/4] Starting sender..."
cd "$GATEWAY_DIR"
./bin/sender > ../sender_out.log 2>&1 &
SENDER_PID=$!
echo "Sender PID: $SENDER_PID"
cd ..

echo "Waiting 5 seconds..."
sleep 5

echo ""
echo "=== GATEWAY OUTPUT ==="
cat gateway_out.log

echo ""
echo "=== SENDER OUTPUT ==="
cat sender_out.log

echo ""
echo "=== KILLING PROCESSES ==="
kill $GATEWAY_PID 2>/dev/null
kill $SENDER_PID 2>/dev/null
echo "Done"
