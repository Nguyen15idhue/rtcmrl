# RTCM Gateway UI - Windows Standalone

## Cài đặt nhanh cho Windows

### Yêu cầu

1. **Go 1.21+** - Để build gateway
2. **Npcap** - Để dùng chế độ PCAP (tải từ https://npcap.com/)

### Build Gateway

```bash
cd rtcm-stream-gateway
go build -o gateway.exe ./cmd/gateway/
```

### Chạy nhanh

```bash
# Chạy gateway và UI cùng lúc
run.bat
```

Hoặc:

```bash
# 1. Chạy gateway
cd rtcm-stream-gateway
gateway.exe

# 2. Mở trình duyệt
# http://localhost:8080
```

### Các chế độ hoạt động

| Chế độ | Mô tả | Cần gì |
|--------|-------|---------|
| TCP | Listen trên port | Không cần gì |
| PCAP | Sniff traffic | Npcap |
| Auto | Tự động chọn | Npcap |

### Cấu hình mặc định

- **Port nghe:** 12101
- **NTRIP Caster:** rtktk.online:1509
- **Web UI:** http://localhost:8080

### Thay đổi cấu hình

Edit file `config.json` trong thư mục `rtcm-stream-gateway`:

```json
{
  "capture": {
    "device": "any",
    "listen_port": 12101
  },
  "caster": {
    "host": "YOUR_CASTER_HOST",
    "port": 2101,
    "pass": "YOUR_PASSWORD",
    "mount_prefix": "STN"
  },
  "mode": "auto"
}
```

### Chạy với các tùy chọn

```bash
# TCP mode
MODE=tcp gateway.exe

# PCAP mode với device cụ thể
MODE=pcap DEVICE="\Device\NPF_Loopback" gateway.exe

# Port khác
LISTEN_PORT=12102 gateway.exe
```

### Các thiết bị mạng trên Windows

Để xem danh sách thiết bị:

```bash
ipconfig /all
```

Device paths cho PCAP:
- Loopback: `\Device\NPF_Loopback`
- Ethernet: `\Device\NPF_{GUID}` (xem trong Npcap installer)
