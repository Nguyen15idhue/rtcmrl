# RTCM Stream Gateway (Go)

Dự án này capture traffic TCP vào port 12101 theo kiểu thụ động (tcpdump-style), tái lắp TCP stream, tách khung RTCM3 hợp lệ và fan-out đồng thời nhiều trạm sang caster B theo nhiều mountpoint.

Mục tiêu:
- Dịch vụ caster đang chạy không cần thay đổi cấu hình.
- Host biết có process capture, nhưng app caster không có client đăng nhập thêm.
- Tối ưu cho nhiều nguồn đồng thời (30-50 trạm).
- Tự nhận diện trạm ổn định theo RTCM Station ID (không phụ thuộc IP 4G).

## Tính năng hiện có

- Capture qua libpcap + BPF filter.
- Tái lắp TCP stream với `tcpassembly`.
- Đồng bộ theo byte `0xD3`, tách frame RTCM3 theo length field.
- Kiểm CRC24Q trước khi gửi.
- Trích `Station ID` từ RTCM header để định danh trạm ổn định.
- Mỗi Station ID được tự map thành mountpoint riêng: `PREFIX_XXXX`.
- Anti-collision Station ID bằng fingerprint tĩnh từ bản tin 1005/1006/1033.
- Khi trùng Station ID, tự tách thành mountpoint phụ: `PREFIX_XXXX_ABCDEF`.
- Tự tạo kết nối SOURCE riêng cho từng mountpoint.
- Tự reconnect caster đích khi lỗi mạng.
- Log thống kê định kỳ.

## Cấu trúc

- `cmd/gateway/main.go`: điểm vào chương trình.
- `internal/capture`: capture + reassembly.
- `internal/rtcm`: tách frame và CRC24Q.
- `internal/engine`: chọn nguồn active và điều phối gửi.
- `internal/caster`: kết nối SOURCE tới caster đích.
- `configs/gateway.env.example`: mẫu cấu hình.

## Chạy local

1. Cài Go 1.22+ và libpcap-dev (Linux):

```bash
sudo apt-get update
sudo apt-get install -y libpcap-dev
```

2. Build:

```bash
go mod tidy
go build -o bin/gateway ./cmd/gateway
```

3. Chạy:

```bash
sudo ./bin/gateway \
  -device any \
  -listen-port 12101 \
  -caster-host rtktk.online \
  -caster-port 1509 \
  -caster-pass 123456 \
  -mount-prefix STN
```

Mountpoint sẽ tự sinh theo Station ID, ví dụ:
- `STN_0023`
- `STN_0157`
- `STN_1021`

4. Cách detect ổn định khi IP SIM 4G thay đổi

- Không dùng IP để định danh trạm.
- Dùng `Station ID` trong RTCM (12-bit) để map mountpoint.
- Khi IP đổi, chỉ cần trạm vẫn phát Station ID cũ thì vẫn đi đúng mountpoint cũ.
- `SOURCE_IDLE_SEC` chỉ dùng để dọn mapping flow cũ đã im lặng.

5. Cơ chế anti-collision Station ID

- Nếu 2 thiết bị khác nhau vô tình cùng Station ID:
  - Engine sẽ dùng fingerprint tĩnh (từ 1005/1006/1033) để tách 2 nguồn.
  - Nguồn đầu tiên dùng mountpoint gốc `PREFIX_XXXX`.
  - Nguồn thứ hai trở đi dùng mountpoint có hậu tố fingerprint ngắn.
- Điều này giúp tránh dồn sai dữ liệu vào cùng một mountpoint.

## Deploy 1 lệnh từ GitHub

Sau khi bạn push repo lên GitHub, chạy trên VPS:

```bash
curl -fsSL https://raw.githubusercontent.com/<owner>/<repo>/<branch>/scripts/bootstrap.sh | sudo bash -s -- \
  --repo https://github.com/<owner>/<repo>.git \
  --branch <branch>
```

Script sẽ tự:
- cài dependency (git, golang, libpcap-dev),
- clone/pull repo,
- build binary,
- tạo user chạy service,
- tạo/cập nhật systemd service,
- enable + restart service.

Sau đó chỉnh file env:

```bash
sudo nano /opt/rtcm-stream-gateway/gateway.env
sudo systemctl restart rtcm-stream-gateway
sudo journalctl -u rtcm-stream-gateway -f
```

## Gợi ý triển khai VPS

- Chạy bằng systemd.
- Dùng `AmbientCapabilities=CAP_NET_RAW CAP_NET_ADMIN`.
- Giới hạn `CPUQuota`, `MemoryMax` để tránh ảnh hưởng host.
