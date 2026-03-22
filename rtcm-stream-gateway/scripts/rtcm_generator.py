#!/usr/bin/env python3
import socket
import time
import struct
import random

STATION_IDS = [1, 2, 3, 1001, 1002, 2001]


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
    num_sats = random.randint(8, 15)
    payload = bytes([0x00, 0x00, 0x00])
    payload += bytes([num_sats])
    for _ in range(num_sats):
        payload += bytes([random.randint(1, 37), 0x00])
        payload += bytes([random.randint(0, 127)])
        payload += bytes([random.randint(0, 63)])
        payload += bytes([random.randint(0, 15)])
    return build_rtcm3(1074, payload)


def build_msg1124(station_id):
    num_sats = random.randint(5, 10)
    payload = bytes([0x00, 0x00])
    payload += bytes([num_sats])
    for _ in range(num_sats):
        payload += bytes([random.randint(1, 5)])
        payload += bytes([random.randint(0, 127)])
        payload += bytes([random.randint(0, 15)])
    return build_rtcm3(1124, payload)


class RTCMGenerator:
    def __init__(self, host="127.0.0.1", port=12101, stations=None, fps=10):
        self.host = host
        self.port = port
        self.stations = stations or STATION_IDS
        self.fps = fps
        self.running = False
        self.sock = None

    def start(self):
        self.running = True
        print(f"[RTCMGen] Connecting to {self.host}:{self.port}")
        while self.running:
            try:
                self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                self.sock.connect((self.host, self.port))
                print(f"[RTCMGen] Connected! Generating for stations: {self.stations}")
                print(
                    "[RTCMGen] Sending rate: {0} msg/sec per station".format(self.fps)
                )
                print()

                self.send_loop()
            except ConnectionRefusedError:
                print(f"[RTCMGen] Connection refused, retrying in 2s...")
                time.sleep(2)
            except Exception as e:
                print(f"[RTCMGen] Error: {e}")
                time.sleep(2)

    def send_loop(self):
        interval = 1.0 / self.fps
        while self.running:
            for station_id in self.stations:
                frames = [
                    build_msg1005(station_id),
                    build_msg1006(station_id),
                    build_msg1074(station_id),
                    build_msg1124(station_id),
                ]
                for frame in frames:
                    try:
                        if self.sock:
                            self.sock.sendall(frame)
                    except:
                        return
            time.sleep(interval)

    def stop(self):
        self.running = False
        if self.sock:
            self.sock.close()


if __name__ == "__main__":
    import sys

    print("=" * 50)
    print("RTCM Test Data Generator")
    print("=" * 50)

    port = int(sys.argv[1]) if len(sys.argv) > 1 else 12101
    host = sys.argv[2] if len(sys.argv) > 2 else "127.0.0.1"

    gen = RTCMGenerator(host=host, port=port, stations=STATION_IDS, fps=5)
    try:
        gen.start()
    except KeyboardInterrupt:
        gen.stop()
        print("[RTCMGen] Stopped")
