#!/usr/bin/env python3
import socket
import time
import struct


def make_crc24q_table():
    table = []
    for i in range(256):
        crc = i << 16
        for _ in range(8):
            crc <<= 1
            if crc & 0x1000000:
                crc ^= 0x1864CFB
        table.append(crc & 0xFFFFFF)
    return table


CRC24Q_TABLE = make_crc24q_table()


def crc24q(data):
    crc = 0
    for b in data:
        crc = ((crc << 8) ^ CRC24Q_TABLE[(crc >> 16) ^ b]) & 0xFFFFFF
    return crc


def build_rtcm3(msg_type, payload):
    header = bytes([0xD3, (len(payload) >> 8) & 0xFF, len(payload) & 0xFF])
    msg_byte3 = (msg_type >> 4) & 0xFF
    msg_byte4 = (msg_type & 0x0F) << 4
    data = header + bytes([msg_byte3, msg_byte4]) + payload
    crc = crc24q(data)
    return data + bytes([(crc >> 16) & 0xFF, (crc >> 8) & 0xFF, crc & 0xFF])


def build_msg1005(station_id):
    # RTCM3 1005: 13-byte payload
    # DF002-003: reserved (2)
    # DF021-048: ECEF X (8)
    # DF049-052: antenna height (3)
    payload = bytes([0x00, 0x00])
    payload += struct.pack(">d", 1234567.89)
    payload += bytes([0x00, 0x00, 0x00])
    return build_rtcm3(1005, payload)


def build_msg1006(station_id):
    payload = bytes([0x00, 0x00])
    payload += struct.pack(">d", 1234567.89)
    payload += struct.pack(">H", 100)
    return build_rtcm3(1006, payload)


def build_msg1074(station_id):
    num_sats = 10
    payload = bytes([0x00, 0x00, 0x00, num_sats])
    for i in range(num_sats):
        payload += bytes([i + 1, 0x00, i * 10, i])
    return build_rtcm3(1074, payload)


def build_msg1124(station_id):
    num_sats = 8
    payload = bytes([0x00, 0x00, num_sats])
    for i in range(num_sats):
        payload += bytes([i + 1, i * 8, i])
    return build_rtcm3(1124, payload)


STATIONS = [1, 2, 3, 1001, 2001]
FPS = 5

if __name__ == "__main__":
    host = "127.0.0.1"
    port = 12101

    print(f"Connecting to gateway at {host}:{port}...")
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.connect((host, port))
    print("Connected! Sending RTCM test data...")

    interval = 1.0 / FPS
    while True:
        for sid in STATIONS:
            frames = [
                build_msg1005(sid),
                build_msg1006(sid),
                build_msg1074(sid),
                build_msg1124(sid),
            ]
            for fr in frames:
                s.sendall(fr)
        time.sleep(interval)
