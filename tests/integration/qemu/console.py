#!/usr/bin/env python3
"""Persistent serial-console bridge for the QEMU integration harness.

QEMU's TCP serial server handles one client at a time and drops output while
no client is connected. This helper keeps a single connection open, mirrors
output to a log file, and sends commands appended to a command file. Tests can
therefore drive the installer/console without reconnect races.
"""
import argparse
import os
import select
import socket
import sys
import time


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--host", default="127.0.0.1")
    ap.add_argument("--port", type=int, default=4444)
    ap.add_argument("--log", required=True)
    ap.add_argument("--commands", required=True)
    args = ap.parse_args()

    deadline = time.time() + 120
    sock = None
    while time.time() < deadline:
        try:
            sock = socket.create_connection((args.host, args.port), timeout=5)
            break
        except OSError:
            time.sleep(1)
    if sock is None:
        print("console: could not connect", file=sys.stderr)
        return 1
    sock.setblocking(False)

    log = open(args.log, "ab", buffering=0)
    last_cmd_mtime = 0.0
    if os.path.exists(args.commands):
        last_cmd_mtime = os.path.getmtime(args.commands)

    while True:
        try:
            r, _, _ = select.select([sock], [], [], 0.2)
        except OSError:
            break
        if r:
            try:
                data = sock.recv(65536)
            except BlockingIOError:
                data = b""
            except OSError:
                break
            if not data:
                break
            log.write(data)
        try:
            mtime = os.path.getmtime(args.commands)
        except OSError:
            mtime = last_cmd_mtime
        if mtime != last_cmd_mtime:
            last_cmd_mtime = mtime
            try:
                with open(args.commands, "rb") as fh:
                    payload = fh.read()
            except OSError:
                payload = b""
            if payload:
                sock.sendall(payload)
                open(args.commands, "wb").close()
    log.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
