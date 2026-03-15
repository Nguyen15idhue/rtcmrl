# TCPDUMP RTCM Relay: Demo và Triển Khai Chi Tiết

Tài liệu này mô tả cách relay dữ liệu RTCM theo hướng:
- Chỉ capture ở mức hệ điều hành (server biết).
- Dịch vụ caster đang chạy trên port 12101 không biết có relay.
- Đẩy stream sang caster khác thành 1 mountpoint mới.

## 1. Mục tiêu và giới hạn

### Mục tiêu
- Không sửa cấu hình caster gốc.
- Không tạo client kết nối trực tiếp vào caster gốc.
- Chỉ đọc packet realtime trên host.
- Relay sang caster B theo thời gian thực.

### Giới hạn
- Có footprint CPU/RAM trên host vì phải capture và relay.
- Không phải lựa chọn tối ưu nhất cho hệ thống 24/7 nếu bạn có thể dùng relay chuẩn.
- Nếu luồng gốc dùng TLS thì payload có thể bị mã hóa, không đọc được RTCM.

## 2. Kiến trúc tổng quan

Pipeline hoạt động:
1. `tcpdump` capture packet TCP port 12101.
2. Process relay reassemble TCP stream theo từng flow.
3. Bỏ NTRIP/HTTP header đầu stream, lấy raw RTCM bắt đầu từ byte `D3`.
4. Đẩy bytes sang caster B ở mountpoint đích.

## 3. Demo nhanh (MVP)

Mục đích demo:
- Chứng minh có thể lấy realtime stream và đẩy sang caster B.
- Chưa cần decode MsgID.
- Chưa cần CRC check đầy đủ.

### 3.1 Điều kiện
- Ubuntu 22.04 hoặc 24.04.
- Có quyền `sudo`.
- Có thông tin caster B:
  - `CASTER_B_HOST`
  - `CASTER_B_PORT`
  - `CASTER_B_SOURCE_PASS`
  - `MOUNTPOINT_DICH`

### 3.2 Cài gói cần thiết

```bash
sudo apt-get update
sudo apt-get install -y python3 python3-pip tcpdump
```

### 3.3 Tạo script relay demo

Tạo file `/opt/rtcm-relay/relay_from_tcpdump.py` với nội dung sau:

```python
#!/usr/bin/env python3
import argparse
import socket
import struct
import subprocess
import sys
import time
from collections import defaultdict


def parse_pcap_stream(stream, target_port):
    gh = stream.read(24)
    if len(gh) < 24:
        return
    magic = struct.unpack('<I', gh[:4])[0]
    if magic not in (0xA1B2C3D4, 0xD4C3B2A1):
        raise RuntimeError('PCAP header không hợp lệ')

    while True:
        ph = stream.read(16)
        if len(ph) < 16:
            break
        ts_sec, ts_usec, incl_len, orig_len = struct.unpack('<IIII', ph)
        pkt = stream.read(incl_len)
        if len(pkt) < incl_len:
            break

        # Linux SLL2 link header: 20 bytes
        if len(pkt) < 20:
            continue
        proto = struct.unpack('>H', pkt[0:2])[0]
        if proto != 0x0800:
            continue
        ip = pkt[20:]
        if len(ip) < 20:
            continue

        ihl = (ip[0] & 0x0F) * 4
        if len(ip) < ihl + 20:
            continue
        if ip[9] != 6:
            continue

        src_ip = '.'.join(str(x) for x in ip[12:16])
        dst_ip = '.'.join(str(x) for x in ip[16:20])
        tcp = ip[ihl:]
        src_port = struct.unpack('>H', tcp[0:2])[0]
        dst_port = struct.unpack('>H', tcp[2:4])[0]
        seq = struct.unpack('>I', tcp[4:8])[0]
        off = ((tcp[12] >> 4) & 0xF) * 4
        payload = tcp[off:]

        if src_port != target_port and dst_port != target_port:
            continue
        if not payload:
            continue

        yield (src_ip, src_port, dst_ip, dst_port, seq, payload)


def source_line(password, mountpoint):
    if not mountpoint.startswith('/'):
        mountpoint = '/' + mountpoint
    return f'SOURCE {password} {mountpoint}\r\nSource-Agent: tcpdump-relay\r\n\r\n'.encode()


def connect_caster(host, port, password, mountpoint):
    s = socket.create_connection((host, port), timeout=10)
    s.sendall(source_line(password, mountpoint))
    s.settimeout(10)
    return s


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--iface', default='any')
    ap.add_argument('--port', type=int, default=12101)
    ap.add_argument('--b-host', required=True)
    ap.add_argument('--b-port', type=int, default=2101)
    ap.add_argument('--b-pass', required=True)
    ap.add_argument('--b-mount', required=True)
    ap.add_argument('--source-ip', default='')
    args = ap.parse_args()

    tcpdump_cmd = [
        'tcpdump', '-i', args.iface, '-s', '0', '-U', '-w', '-',
        f'tcp port {args.port}'
    ]

    print('Start tcpdump:', ' '.join(tcpdump_cmd), file=sys.stderr)
    proc = subprocess.Popen(tcpdump_cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    sock = None
    buffers = defaultdict(bytes)
    synced = set()
    last_reconnect = 0

    try:
        for src_ip, src_port, dst_ip, dst_port, seq, payload in parse_pcap_stream(proc.stdout, args.port):
            if args.source_ip and src_ip != args.source_ip and dst_ip != args.source_ip:
                continue

            flow = (src_ip, src_port, dst_ip, dst_port)
            b = buffers[flow] + payload

            if flow not in synced:
                d3 = b.find(b'\xD3')
                if d3 >= 0:
                    b = b[d3:]
                    synced.add(flow)
                else:
                    if len(b) > 8192:
                        b = b[-4096:]
                    buffers[flow] = b
                    continue

            buffers[flow] = b

            if sock is None:
                now = time.time()
                if now - last_reconnect < 1:
                    continue
                last_reconnect = now
                try:
                    sock = connect_caster(args.b_host, args.b_port, args.b_pass, args.b_mount)
                    print('Đã kết nối caster B', file=sys.stderr)
                except Exception as ex:
                    print('Kết nối caster B lỗi:', ex, file=sys.stderr)
                    sock = None
                    continue

            try:
                sock.sendall(b)
                buffers[flow] = b''
            except Exception as ex:
                print('Gửi lỗi, sẽ reconnect:', ex, file=sys.stderr)
                try:
                    sock.close()
                except Exception:
                    pass
                sock = None

    finally:
        try:
            if sock:
                sock.close()
        except Exception:
            pass
        proc.kill()


if __name__ == '__main__':
    main()
```

### 3.4 Chạy demo

```bash
sudo mkdir -p /opt/rtcm-relay
sudo nano /opt/rtcm-relay/relay_from_tcpdump.py
sudo chmod +x /opt/rtcm-relay/relay_from_tcpdump.py

sudo /opt/rtcm-relay/relay_from_tcpdump.py \
  --iface any \
  --port 12101 \
  --b-host CASTER_B_HOST \
  --b-port 2101 \
  --b-pass CASTER_B_SOURCE_PASS \
  --b-mount DEMO_MOUNT \
  --source-ip IP_NGUON_UU_TIEN
```

Nếu chưa biết source-ip, có thể bỏ `--source-ip` để test nhanh.

### 3.5 Kiểm tra demo thành công
- Trên caster B: mountpoint `DEMO_MOUNT` lên online.
- Có data RTCM đổ liên tục, không bị ngắt quãng lớn.
- Log relay không bị reconnect liên tục.

## 4. Triển khai production chi tiết

### 4.1 Tạo user riêng và thư mục

```bash
sudo useradd -r -s /usr/sbin/nologin rtcmrelay || true
sudo mkdir -p /opt/rtcm-relay /var/log/rtcm-relay
sudo chown -R rtcmrelay:rtcmrelay /opt/rtcm-relay /var/log/rtcm-relay
```

### 4.2 Tạo file cấu hình môi trường

Tạo `/opt/rtcm-relay/relay.env`:

```bash
IFACE=any
IN_PORT=12101
SOURCE_IP=27.67.120.242
B_HOST=your-caster-b.example.com
B_PORT=2101
B_PASS=your_source_password
B_MOUNT=DEMO_MOUNT
```

Set quyền:

```bash
sudo chown rtcmrelay:rtcmrelay /opt/rtcm-relay/relay.env
sudo chmod 600 /opt/rtcm-relay/relay.env
```

### 4.3 Tạo wrapper script

Tạo `/opt/rtcm-relay/run_relay.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
source /opt/rtcm-relay/relay.env
exec /usr/bin/python3 /opt/rtcm-relay/relay_from_tcpdump.py \
  --iface "$IFACE" \
  --port "$IN_PORT" \
  --b-host "$B_HOST" \
  --b-port "$B_PORT" \
  --b-pass "$B_PASS" \
  --b-mount "$B_MOUNT" \
  --source-ip "$SOURCE_IP"
```

Set quyền:

```bash
sudo chown rtcmrelay:rtcmrelay /opt/rtcm-relay/run_relay.sh
sudo chmod 750 /opt/rtcm-relay/run_relay.sh
```

### 4.4 Tạo systemd service

Tạo `/etc/systemd/system/rtcm-relay.service`:

```ini
[Unit]
Description=RTCM relay from tcpdump to caster B
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=rtcmrelay
Group=rtcmrelay
ExecStart=/opt/rtcm-relay/run_relay.sh
Restart=always
RestartSec=2
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/opt/rtcm-relay /var/log/rtcm-relay
AmbientCapabilities=CAP_NET_RAW CAP_NET_ADMIN
CapabilityBoundingSet=CAP_NET_RAW CAP_NET_ADMIN
StandardOutput=append:/var/log/rtcm-relay/relay.log
StandardError=append:/var/log/rtcm-relay/relay.err
CPUQuota=20%
MemoryMax=300M
```

Áp dụng service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable rtcm-relay
sudo systemctl start rtcm-relay
sudo systemctl status rtcm-relay --no-pager
```

### 4.5 Healthcheck và giám sát

Kiểm tra process:

```bash
systemctl is-active rtcm-relay
```

Theo dõi lỗi reconnect:

```bash
tail -f /var/log/rtcm-relay/relay.err
```

Xác nhận mountpoint B đang có data:
- Quan sát trực tiếp ở caster B hoặc dùng NTRIP client test.

### 4.6 Rollback an toàn

Dừng relay:

```bash
sudo systemctl stop rtcm-relay
sudo systemctl disable rtcm-relay
```

Xóa service:

```bash
sudo rm -f /etc/systemd/system/rtcm-relay.service
sudo systemctl daemon-reload
```

Rollback này không ảnh hưởng caster gốc vì relay chạy out-of-band.

## 5. Mẹo giảm ảnh hưởng lên host

1. Lọc hẹp traffic capture, ví dụ chỉ 1 nguồn chính:
   - `tcp port 12101 and host 27.67.120.242`
2. Giữ CPU quota thấp (20-30%) trong systemd.
3. Giữ MemoryMax rõ ràng để tránh ăn RAM bất thường.
4. Chạy nice thấp ưu tiên (ví dụ `nice -n 10`).
5. Chỉ chọn 1 source-ip ổn định để relay, tránh trộn nhiều phiên.

## 6. Tiêu chí nghiệm thu

Demo đạt khi:
1. Mountpoint trên caster B online ổn định.
2. Luồng RTCM liên tục tối thiểu 15 phút.
3. Caster gốc không có dấu hiệu bị ảnh hưởng bất thường (CPU, RAM, socket count).
4. Dừng relay thì caster gốc vẫn hoạt động bình thường.

## 7. Khuyến nghị sau demo

Nếu chạy lâu dài, nên nâng cấp relay:
1. Thêm CRC24Q check.
2. Thêm dedup frame để tránh trùng do retransmit.
3. Thêm metrics và cảnh báo (throughput, reconnect, độ trễ).
4. Sau cùng cân nhắc chuyển sang relay chuẩn nếu điều kiện cho phép.
