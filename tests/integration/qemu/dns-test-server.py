#!/usr/bin/env python3
"""Minimal UDP DNS responder for the conditional-forwarding fixture."""
import socket
import struct

BIND = ("127.0.0.2", 53)
ANSWER = socket.inet_aton("10.99.0.9")


def response(query: bytes) -> bytes:
    if len(query) < 12:
        return b""
    txid = query[:2]
    pos = 12
    while pos < len(query) and query[pos] != 0:
        pos += query[pos] + 1
    pos += 1
    if pos + 4 > len(query):
        return b""
    qtype, _qclass = struct.unpack("!HH", query[pos:pos + 4])
    question = query[12:pos + 4]
    if qtype == 1:
        answer = b"\xc0\x0c" + struct.pack("!HHIH", 1, 1, 60, 4) + ANSWER
        ancount = 1
    else:
        answer = b""
        ancount = 0
    header = txid + b"\x81\x80" + struct.pack("!HHHH", 1, ancount, 0, 0)
    return header + question + answer


def main() -> int:
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(BIND)
    print(f"test DNS server on {BIND[0]}:{BIND[1]}", flush=True)
    while True:
        try:
            query, peer = sock.recvfrom(2048)
        except KeyboardInterrupt:
            break
        reply = response(query)
        if reply:
            sock.sendto(reply, peer)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
