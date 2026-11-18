#!/usr/bin/env python3
"""Guard the vendored logging redactions.

Re-vendoring newt/olm must not reintroduce the debug statements that leaked
the client secret, bearer token, WireGuard private key or the full OLM config.
"""
import pathlib

root = pathlib.Path(__file__).resolve().parent.parent
forbidden = {
    "third_party/olm/olm/olm.go": [
        'logger.Debug("Starting tunnel with config: %+v", config)',
    ],
    "third_party/olm/websocket/client.go": [
        "with body: %s",
        'logger.Debug("websocket: Received token: %s"',
    ],
    "third_party/olm/peers/peer.go": [
        'logger.Debug("Configuring peer with config: %s"',
    ],
    "third_party/newt/websocket/client.go": [
        'logger.Debug("Token response body: %s"',
        'logger.Debug("Received token: %s"',
    ],
}

failed = 0
for relative, patterns in forbidden.items():
    text = (root / relative).read_text(encoding="utf-8")
    for pattern in patterns:
        if pattern in text:
            failed += 1
            print(f"FAIL {relative}: forbidden log statement: {pattern}")

print(f"redaction failures: {failed}")
raise SystemExit(1 if failed else 0)
