"""
RTCM3 PCAP Analyzer
Parses PCAP captured on TCP port 12101, reassembles TCP stream, then
decodes and reports RTCM3 message statistics.
"""

import struct
import sys
from collections import defaultdict, Counter

PCAP_FILE = r"f:\3.Laptrinh\1. Project\1. GNSS\rtcmrl\test.pcap"
TARGET_PORT = 12101

# ---------------------------------------------------------------------------
# CRC-24Q (RTCM3)
# ---------------------------------------------------------------------------
def _make_crc24q_table():
    table = []
    for i in range(256):
        crc = i << 16
        for _ in range(8):
            crc <<= 1
            if crc & 0x1000000:
                crc ^= 0x1864CFB
        table.append(crc & 0xFFFFFF)
    return table

_CRC_TABLE = _make_crc24q_table()

def crc24q(data: bytes) -> int:
    crc = 0
    for byte in data:
        crc = ((crc << 8) ^ _CRC_TABLE[((crc >> 16) ^ byte) & 0xFF]) & 0xFFFFFF
    return crc

# ---------------------------------------------------------------------------
# RTCM3 message names
# ---------------------------------------------------------------------------
MSG_NAMES = {
    1001: "GPS L1 Obs (basic)",         1002: "GPS L1 Obs (ext)",
    1003: "GPS L1/L2 Obs (basic)",      1004: "GPS L1/L2 Obs (ext)",
    1005: "Station ARP (no height)",    1006: "Station ARP (with height)",
    1007: "Antenna Descriptor",         1008: "Antenna S/N",
    1009: "GLO L1 Obs (basic)",         1010: "GLO L1 Obs (ext)",
    1011: "GLO L1/L2 (basic)",          1012: "GLO L1/L2 (ext)",
    1013: "System Parameters",          1014: "Network Aux Frame",
    1015: "GPS Ionospheric Corr",       1016: "GPS Geo Corr",
    1017: "GPS Combo Corr",             1019: "GPS Ephemeris",
    1020: "GLONASS Ephemeris",          1029: "Unicode Text String",
    1033: "Rcvr & Ant Desc",            1041: "NavIC/IRNSS Eph",
    1042: "BeiDou Ephemeris",           1044: "QZSS Ephemeris",
    1045: "Galileo F/Nav Eph",          1046: "Galileo I/Nav Eph",
    1057: "GPS SSR Orbit",              1058: "GPS SSR Clock",
    1059: "GPS SSR Code Bias",          1060: "GPS SSR Combined",
    1071: "GPS MSM1",  1072: "GPS MSM2",  1073: "GPS MSM3",  1074: "GPS MSM4",
    1075: "GPS MSM5",  1076: "GPS MSM6",  1077: "GPS MSM7",
    1081: "GLO MSM1",  1082: "GLO MSM2",  1083: "GLO MSM3",  1084: "GLO MSM4",
    1085: "GLO MSM5",  1086: "GLO MSM6",  1087: "GLO MSM7",
    1091: "GAL MSM1",  1092: "GAL MSM2",  1093: "GAL MSM3",  1094: "GAL MSM4",
    1095: "GAL MSM5",  1096: "GAL MSM6",  1097: "GAL MSM7",
    1101: "SBAS MSM1", 1104: "SBAS MSM4", 1107: "SBAS MSM7",
    1111: "QZSS MSM1", 1114: "QZSS MSM4", 1117: "QZSS MSM7",
    1121: "BDS MSM1",  1124: "BDS MSM4",  1127: "BDS MSM7",
    1230: "GLO L1/L2 Code Bias",
    4072: "Proprietary (u-blox)",
}

# ---------------------------------------------------------------------------
# RTCM3 parser
# ---------------------------------------------------------------------------
def parse_rtcm3(data: bytes):
    """Return (messages, crc_errors, sync_skips)
    messages = list of dicts: {offset, msg_type, length, raw}
    """
    messages = []
    crc_errors = 0
    sync_skips = 0
    i = 0
    n = len(data)
    while i < n:
        if data[i] != 0xD3:
            i += 1
            sync_skips += 1
            continue
        if i + 3 > n:
            break
        b1, b2 = data[i+1], data[i+2]
        if b1 & 0xFC:          # upper 6 bits of byte1 must be 0
            i += 1
            sync_skips += 1
            continue
        length = ((b1 & 0x03) << 8) | b2
        total = 3 + length + 3
        if i + total > n:
            break
        frame = data[i: i + total]
        calc_crc = crc24q(frame[:-3])
        stored_crc = (frame[-3] << 16) | (frame[-2] << 8) | frame[-1]
        if calc_crc != stored_crc:
            i += 1
            crc_errors += 1
            continue
        msg_type = -1
        if length >= 2:
            msg_type = ((frame[3] << 4) | (frame[4] >> 4)) & 0xFFF
        messages.append({"offset": i, "msg_type": msg_type, "length": length, "raw": frame})
        i += total
    return messages, crc_errors, sync_skips

# ---------------------------------------------------------------------------
# PCAP reader (no external dependencies)
# ---------------------------------------------------------------------------
def read_pcap(path):
    with open(path, "rb") as f:
        raw = f.read()

    magic, ver_major, ver_minor, _tz, _sig, snaplen, link_type = struct.unpack_from("<IHHiIII", raw, 0)
    if magic not in (0xA1B2C3D4, 0xD4C3B2A1):
        raise ValueError(f"Not a PCAP file (magic=0x{magic:08X})")

    swap = (magic == 0xD4C3B2A1)  # big-endian PCAP is rare; A1B2... is LE normal
    fmt = "<IIII"

    print(f"PCAP v{ver_major}.{ver_minor}  snaplen={snaplen}  link_type={link_type}")

    offset = 24
    packets = []          # (ts_float, ip_proto, src_ip, src_port, dst_ip, dst_port, seq, flags, payload)
    raw_len = len(raw)

    while offset + 16 <= raw_len:
        ts_sec, ts_usec, incl_len, orig_len = struct.unpack_from(fmt, raw, offset)
        offset += 16
        if offset + incl_len > raw_len:
            break
        pkt = raw[offset: offset + incl_len]
        offset += incl_len
        ts = ts_sec + ts_usec / 1_000_000

        # ----- strip link layer -----
        if link_type == 1:       # Ethernet
            if len(pkt) < 14: continue
            proto = struct.unpack_from(">H", pkt, 12)[0]
            ip = pkt[14:]
        elif link_type == 113:   # Linux SLL v1 (16 bytes)
            if len(pkt) < 16: continue
            proto = struct.unpack_from(">H", pkt, 14)[0]
            ip = pkt[16:]
        elif link_type == 276:   # Linux SLL2 v2 (20 bytes)
            if len(pkt) < 20: continue
            proto = struct.unpack_from(">H", pkt, 0)[0]
            ip = pkt[20:]
        elif link_type == 101:   # Raw IPv4
            proto = 0x0800
            ip = pkt
        else:
            continue

        # ----- IPv4 only -----
        if proto != 0x0800:
            continue
        if len(ip) < 20:
            continue
        ihl = (ip[0] & 0x0F) * 4
        ip_proto = ip[9]
        src_ip = "%d.%d.%d.%d" % (ip[12], ip[13], ip[14], ip[15])
        dst_ip = "%d.%d.%d.%d" % (ip[16], ip[17], ip[18], ip[19])

        # ----- TCP only -----
        if ip_proto != 6:
            continue
        tcp = ip[ihl:]
        if len(tcp) < 20:
            continue
        src_port = struct.unpack_from(">H", tcp, 0)[0]
        dst_port = struct.unpack_from(">H", tcp, 2)[0]
        seq      = struct.unpack_from(">I", tcp, 4)[0]
        tcp_hlen = ((tcp[12] >> 4) & 0xF) * 4
        flags    = tcp[13]
        payload  = tcp[tcp_hlen:]

        packets.append((ts, src_ip, src_port, dst_ip, dst_port, seq, flags, payload))

    return packets

# ---------------------------------------------------------------------------
# TCP stream reassembler (simple, ISN-unaware, offset by first seq)
# ---------------------------------------------------------------------------
def reassemble_streams(packets):
    """Returns dict: stream_key -> (list_of_ts, assembled_bytes)"""
    # Group by stream
    segs = defaultdict(list)   # key -> [(seq, ts, payload)]
    for ts, si, sp, di, dp, seq, flags, payload in packets:
        if sp == TARGET_PORT or dp == TARGET_PORT:
            key = (si, sp, di, dp)
            segs[key].append((seq, ts, payload))

    streams = {}
    for key, items in segs.items():
        items_sorted = sorted(items, key=lambda x: x[0])
        assembled = bytearray()
        ts_list   = []
        next_seq  = None
        for seq, ts, payload in items_sorted:
            if not payload:
                ts_list.append(ts)
                continue
            if next_seq is None:
                assembled.extend(payload)
                next_seq = (seq + len(payload)) & 0xFFFFFFFF
                ts_list.append(ts)
            else:
                # Wrap-aware comparison
                diff = (seq - next_seq) & 0xFFFFFFFF
                if diff == 0:
                    assembled.extend(payload)
                    next_seq = (seq + len(payload)) & 0xFFFFFFFF
                    ts_list.append(ts)
                elif diff < 0x80000000:  # gap (out-of-order or dropped)
                    assembled.extend(bytes(diff))   # fill gap with 0x00
                    assembled.extend(payload)
                    next_seq = (seq + len(payload)) & 0xFFFFFFFF
                    ts_list.append(ts)
                else:
                    # overlap / retransmit
                    overlap = (next_seq - seq) & 0xFFFFFFFF
                    if overlap < len(payload):
                        assembled.extend(payload[overlap:])
                        next_seq = (seq + len(payload)) & 0xFFFFFFFF
                    ts_list.append(ts)
        streams[key] = (ts_list, bytes(assembled))
    return streams

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
def main():
    packets = read_pcap(PCAP_FILE)
    print(f"Total TCP packets parsed: {len(packets)}")

    # Overall capture window
    all_ts = [p[0] for p in packets]
    if all_ts:
        t0, t1 = min(all_ts), max(all_ts)
        import datetime
        dt0 = datetime.datetime.utcfromtimestamp(t0)
        dt1 = datetime.datetime.utcfromtimestamp(t1)
        duration = t1 - t0
        print(f"Capture start : {dt0.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]} UTC")
        print(f"Capture end   : {dt1.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]} UTC")
        print(f"Duration      : {duration:.3f} s  ({duration/60:.2f} min)")
    print()

    streams = reassemble_streams(packets)

    if not streams:
        print(f"No TCP streams found on port {TARGET_PORT}.")
        # Show which ports ARE present
        port_counter = Counter()
        for _, si, sp, di, dp, *_ in packets:
            port_counter[sp] += 1
            port_counter[dp] += 1
        print("Top 10 ports seen:")
        for port, cnt in port_counter.most_common(10):
            print(f"  {port}: {cnt} packets")
        return

    print(f"TCP streams on port {TARGET_PORT}: {len(streams)}")
    print()

    # Sort by assembled data size (largest = likely RTCM data stream)
    sorted_streams = sorted(streams.items(), key=lambda x: len(x[1][1]), reverse=True)

    for key, (ts_list, data) in sorted_streams:
        si, sp, di, dp = key
        direction = f"{si}:{sp} -> {di}:{dp}"
        if not data:
            print(f"[Stream {direction}] 0 bytes, skipping.")
            continue

        print("=" * 72)
        print(f"Stream : {direction}")
        print(f"Pkts   : {len(ts_list)}   Assembled: {len(data):,} bytes ({len(data)/1024:.1f} KB)")

        # --- Try RTCM parse ---
        messages, crc_err, sync_skip = parse_rtcm3(data)

        if not messages:
            # Show first 128 bytes as hex to see what's in there
            print(f"  ⚠  No valid RTCM3 frames found.")
            print(f"     CRC errors: {crc_err}  Sync skips: {sync_skip}")
            hex_preview = " ".join(f"{b:02X}" for b in data[:128])
            print(f"     First 128 bytes: {hex_preview}")
            # Check for NTRIP/HTTP header
            try:
                text_head = data[:512].decode("utf-8", errors="replace")
                if any(k in text_head for k in ("HTTP", "NTRIP", "ICY", "SOURCE")):
                    print(f"     Detected NTRIP/HTTP handshake at start.")
                    # Find first 0xD3
                    first_d3 = data.find(0xD3)
                    if first_d3 != -1:
                        print(f"     First 0xD3 at offset {first_d3}, retrying RTCM parse from there...")
                        messages, crc_err, sync_skip = parse_rtcm3(data[first_d3:])
                        for m in messages:
                            m["offset"] += first_d3
            except Exception:
                pass

        if not messages:
            print()
            continue

        # --- Statistics ---
        print(f"  RTCM frames   : {len(messages):,}")
        print(f"  CRC errors    : {crc_err}")
        print(f"  Sync skips    : {sync_skip}")

        # Capture timing
        if ts_list:
            stream_t0 = min(ts_list)
            stream_t1 = max(ts_list)
            stream_dur = stream_t1 - stream_t0
            if stream_dur > 0:
                avg_rate = len(messages) / stream_dur
                print(f"  Stream window : {stream_dur:.1f} s")
                print(f"  Avg msg rate  : {avg_rate:.2f} msg/s")

        # Per-message-type breakdown
        type_counter  = Counter(m["msg_type"] for m in messages)
        total_bytes_by_type = defaultdict(int)
        for m in messages:
            total_bytes_by_type[m["msg_type"]] += m["length"] + 6  # +6 for header+CRC

        print()
        print(f"  {'MsgID':>5}  {'Description':<32}  {'Count':>6}  {'Bytes':>8}  {'%Data':>6}")
        print(f"  {'-'*5}  {'-'*32}  {'-'*6}  {'-'*8}  {'-'*6}")
        total_msg_bytes = sum(total_bytes_by_type.values())
        for mt, cnt in sorted(type_counter.items()):
            name  = MSG_NAMES.get(mt, "Unknown / Proprietary")
            mb    = total_bytes_by_type[mt]
            pct   = 100 * mb / total_msg_bytes if total_msg_bytes else 0
            print(f"  {mt:>5}  {name:<32}  {cnt:>6}  {mb:>8}  {pct:>5.1f}%")

        # Per-second message rate table (first 70 seconds)
        print()
        print("  Per-second RTCM message counts (by packet timestamp approximation):")
        # Map messages to approximate second using offset into assembled stream
        # We'll use a simpler approach: spread messages evenly over stream window
        if ts_list and len(ts_list) > 1:
            stream_t0 = min(ts_list)
            stream_t1 = max(ts_list)
            stream_dur = max(stream_t1 - stream_t0, 0.001)
            total_assembled_bytes = len(data)
            per_second = defaultdict(list)   # second_bucket -> [msg_type]
            for m in messages:
                # Approximate timestamp via byte offset fraction
                frac = m["offset"] / total_assembled_bytes
                approx_ts = stream_t0 + frac * stream_dur
                bucket = int(approx_ts - stream_t0)
                per_second[bucket].append(m["msg_type"])

            max_bucket = max(per_second.keys()) if per_second else 0
            print(f"  {'Sec':>4}  {'Msgs':>5}  Types")
            print(f"  {'-'*4}  {'-'*5}  {'-'*40}")
            for s in range(min(max_bucket + 1, 75)):
                msgs = per_second.get(s, [])
                type_str = ", ".join(str(t) for t in sorted(Counter(msgs).keys()))
                bar = "█" * min(len(msgs), 40)
                print(f"  {s:>4}  {len(msgs):>5}  {bar}")
        else:
            print("  (not enough timing data for per-second breakdown)")

        print()

    # Summary of all message types across all streams
    print("=" * 72)
    print("OVERALL SUMMARY — all streams combined")
    all_msgs = []
    for key, (ts_list, data) in streams.items():
        if data:
            msgs, _, _ = parse_rtcm3(data)
            if not msgs:
                first_d3 = data.find(0xD3)
                if first_d3 != -1:
                    msgs, _, _ = parse_rtcm3(data[first_d3:])
            all_msgs.extend(msgs)

    if all_msgs:
        total_frames = len(all_msgs)
        overall_counter = Counter(m["msg_type"] for m in all_msgs)
        print(f"Total RTCM3 frames decoded : {total_frames:,}")
        print()
        print(f"  {'MsgID':>5}  {'Description':<32}  {'Count':>6}")
        print(f"  {'-'*5}  {'-'*32}  {'-'*6}")
        for mt, cnt in sorted(overall_counter.items()):
            name = MSG_NAMES.get(mt, "Unknown / Proprietary")
            print(f"  {mt:>5}  {name:<32}  {cnt:>6}")
    else:
        print("No RTCM3 frames found across all streams.")

if __name__ == "__main__":
    main()
