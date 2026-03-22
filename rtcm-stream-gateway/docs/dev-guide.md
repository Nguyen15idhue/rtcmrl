# RTCM Stream Gateway - Source Code Guide

## Overview

**Module**: `github.com/your-org/rtcm-stream-gateway`

RTCM Stream Gateway nhận RTCM3 frames từ các nguồn (TCP/pcap), trích xuất thông tin station, và forward frames đến NTRIP caster. Hỗ trợ chế độ test local không cần caster thật.

## Architecture

```
[GPS Receivers] --> [TCP/pcap Source]
                           |
                           v
                  +------------------+
                  |   TCP Listener   |  (capture_tcp.go)
                  |   / pcap Capture |
                  +------------------+
                           |
                           v
                  +------------------+
                  |   RTCM Scanner   |  (rtcm/scanner.go)
                  |  - Sync preamble |
                  |  - Extract frame |
                  |  - Validate CRC  |
                  +------------------+
                           |
                           v
                  +------------------+
                  |     Engine       |  (engine/engine.go)
                  |  - Station ID   |
                  |  - Route        |
                  |  - Fingerprint  |
                  +------------------+
                           |
                           v
                  +------------------+
                  |   NTRIP Caster   |
                  +------------------+
```

## Key Components

### 1. RTCM Scanner (`internal/rtcm/scanner.go`)

Trích xuất RTCM3 frames từ TCP stream.

**RTCM3 Frame Format:**
```
Byte 0:       Preamble (0xD3)
Byte 1-2:     Length (10 bits, payload length)
Bytes 3..N:   Payload
Bytes N+1..:  CRC24Q (3 bytes)
```

**Key functions:**
- `NewScanner()` - Tạo scanner instance
- `Push(data []byte) [][]byte` - Đẩy bytes vào, trả về các frame đã trích xuất
- `ParseFrame(data []byte) (Frame, bool)` - Parse một frame đơn lẻ
- `Encapsulate(payload []byte) []byte` - Tạo frame từ payload
- `CRC24Q(data []byte) uint32` - Tính CRC24Q checksum

**CRC24Q Polynomial:** `0x1864CFB`

### 2. Meta Functions (`internal/rtcm/meta.go`)

Trích xuất metadata từ RTCM3 frame:

- `MessageType(frame []byte) (int, bool)` - Lấy message type (1005, 1006, 1033...)
- `StationID(frame []byte) (int, bool)` - Lấy reference station ID
- `StationFingerprint(frame []byte) (string, bool)` - Tạo fingerprint từ payload hash

### 3. TCP Listener (`internal/capture/capture_tcp.go`)

TCP server nhận RTCM3 frames từ GPS receivers.

```go
type TCPConfig struct {
    ListenPort int
    QueueSize  int
}

handler := func(sourceKey, sourceIP string, frame []byte, at time.Time) {
    pool.Input(engine.InFrame{SourceKey: sourceKey, SourceIP: sourceIP, Frame: frame, At: at})
}
listener := capture.NewTCPListener(cfg, handler)
listener.Run(ctx)
```

### 4. Engine (`internal/engine/engine.go`)

Xử lý logic routing và forwarding:

- Route frame theo Station ID + Fingerprint
- Tạo/v quản lý NTRIP clients cho mỗi mount point
- Auto-create mount points với prefix `STN`
- Metrics collection

**Station variant:** Mỗi station có thể có nhiều "variant" (dựa trên fingerprint). Mount point format: `STN_{ID}` hoặc `STN_{ID}_{fingerprint}`.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MODE` | `pcap` | Capture mode: `tcp` hoặc `pcap` |
| `CASTER_HOST` | `127.0.0.1` | NTRIP caster host |
| `CASTER_PORT` | `2101` | NTRIP caster port |
| `CASTER_USER` | `gateway` | NTRIP username |
| `CASTER_PASS` | `password` | NTRIP password |
| `LISTEN_PORT` | `12101` | TCP/pcap listen port |
| `QUEUE_SIZE` | `4096` | Frame queue size |

**Test Mode:** Khi `CASTER_HOST=test`, engine bỏ qua kết nối caster (để test local).

## Configuration File (`config.yaml`)

```yaml
capture:
  device: "any"        # pcap device
  listen_port: 12101   # TCP/pcap port
  snap_len: 1024
  buffer_mb: 8

worker:
  queue_size: 4096
  min: 4
  max: 16
  auto_scale: true

caster:
  host: "127.0.0.1"
  port: 2101
  mount_prefix: "STN"
  user: "gateway"
  password: "password"

web:
  port: 8080
  metrics_port: 6060
```

## Dependencies

```
github.com/bamiaux/iobit          # Bit-level binary parsing
github.com/go-gnss/rtcm/rtcm3      # RTCM3 message definitions
github.com/go-chi/chi/v5           # HTTP router
github.com/google/gopacket          # pcap parsing
github.com/prometheus/client_golang  # Metrics
```

## Build

```bash
# Build gateway
go build -o bin/gateway ./cmd/gateway

# Build test sender
go build -o bin/sender ./cmd/sender
```

## Testing Locally

```bash
# Terminal 1: Start gateway
MODE=tcp CASTER_HOST=test ./bin/gateway

# Terminal 2: Start sender
./bin/sender

# Or use test script
bash test_local.sh
```

## Directory Structure

```
rtcm-stream-gateway/
├── cmd/
│   ├── gateway/          # Main gateway binary
│   ├── sender/           # Test RTCM sender
│   └── verify_parser/    # Debug station ID parsing
├── internal/
│   ├── rtcm/             # RTCM3 parsing
│   │   ├── scanner.go    # Frame extraction
│   │   ├── meta.go       # Metadata extraction
│   │   └── crc24q.go     # CRC24Q implementation
│   ├── capture/          # Source capture
│   │   ├── capture_tcp.go
│   │   └── capture.go    # pcap mode
│   ├── engine/           # Core routing engine
│   │   ├── engine.go
│   │   └── ntrip.go      # NTRIP client
│   ├── caster/           # NTRIP caster client
│   └── web/              # HTTP API + metrics
├── config.yaml
└── test_local.sh
```

## Common Issues

### Station ID returns 0

Kiểm tra `meta.go` - đảm bảo length check sử dụng `length+3` thay vì `length+6+3`.

### CRC validation fails

Đảm bảo CRC24Q polynomial là `0x1864CFB`. Kiểm tra `crc24q.go`.

### TCP frames not extracted

Scanner cần đồng bộ preamble (0xD3). Nếu TCP stream bắt đầu giữa frame, scanner sẽ tự động re-sync.
