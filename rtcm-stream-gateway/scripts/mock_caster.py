#!/usr/bin/env python3
import socket
import threading
import time
import struct
import random


class MockNTRIPCaster:
    def __init__(self, host="0.0.0.0", port=2101):
        self.host = host
        self.port = port
        self.mounts = {}
        self.running = False
        self.sock = None

    def start(self):
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self.sock.bind((self.host, self.port))
        self.sock.listen(50)
        self.running = True
        print(f"[MockCaster] Listening on {self.host}:{self.port}")
        print("[MockCaster] Mountpoints available:")
        print("  - /STN_0001")
        print("  - /STN_0002")
        print("  - /STN_0003")
        print("  - /TEST")
        print()

        while self.running:
            try:
                self.sock.settimeout(1.0)
                conn, addr = self.sock.accept()
                threading.Thread(
                    target=self.handle_client, args=(conn, addr), daemon=True
                ).start()
            except socket.timeout:
                continue
            except Exception as e:
                if self.running:
                    print(f"[MockCaster] Error: {e}")

    def handle_client(self, conn, addr):
        try:
            conn.settimeout(10)
            data = b""
            while b"\r\n\r\n" not in data:
                chunk = conn.recv(4096)
                if not chunk:
                    return
                data += chunk

            lines = data.decode("utf-8", errors="ignore").split("\r\n")
            if not lines:
                return

            first_line = lines[0]
            parts = first_line.split(" ")
            if len(parts) < 3:
                conn.close()
                return

            cmd = parts[0]
            if cmd == "SOURCE":
                passwd = parts[1]
                mount = parts[2].lstrip("/")
                mount_key = f"/{mount}"
                self.mounts[mount_key] = {
                    "pass": passwd,
                    "addr": addr,
                    "frames": 0,
                    "bytes": 0,
                    "connected": time.time(),
                }
                print(
                    f"[MockCaster] Client connected: {addr} -> {mount_key} (total: {len(self.mounts)} mounts)"
                )
                conn.sendall(b"HTTP/1.0 200 OK\r\n\r\n")

                while True:
                    try:
                        frame = conn.recv(4096)
                        if not frame:
                            break
                        self.mounts[mount_key]["frames"] += 1
                        self.mounts[mount_key]["bytes"] += len(frame)

                        if self.mounts[mount_key]["frames"] % 100 == 0:
                            print(
                                f"[MockCaster] {mount_key}: {self.mounts[mount_key]['frames']} frames, "
                                f"{self.mounts[mount_key]['bytes'] / 1024:.1f} KB"
                            )
                    except socket.timeout:
                        break
                    except:
                        break
            elif cmd == "GET":
                mount = parts[1].lstrip("/")
                print(f"[MockCaster] NTRIP client GET: {mount}")
                conn.sendall(b"HTTP/1.0 200 OK\r\n\r\n")
                while True:
                    try:
                        conn.recv(1024)
                    except:
                        break

        except Exception as e:
            print(f"[MockCaster] Client error: {e}")
        finally:
            for k, v in list(self.mounts.items()):
                if v["addr"] == addr:
                    print(
                        f"[MockCaster] Client disconnected: {addr} from {k} "
                        f"({v['frames']} frames, {v['bytes'] / 1024:.1f} KB)"
                    )
                    del self.mounts[k]
                    break
            conn.close()

    def stop(self):
        self.running = False
        if self.sock:
            self.sock.close()

    def print_stats(self):
        print(f"\n[MockCaster] Active mounts: {len(self.mounts)}")
        for mount, info in self.mounts.items():
            print(
                f"  {mount}: {info['frames']} frames, {info['bytes'] / 1024:.1f} KB, "
                f"uptime: {time.time() - info['connected']:.0f}s"
            )
        print()


if __name__ == "__main__":
    caster = MockNTRIPCaster(host="0.0.0.0", port=2101)
    try:
        caster.start()
    except KeyboardInterrupt:
        caster.print_stats()
        caster.stop()
