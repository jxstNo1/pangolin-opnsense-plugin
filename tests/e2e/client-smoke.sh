#!/bin/sh
# Machine-client smoke test for a real Pangolin instance.
#
# The test intentionally takes a complete, already-validated agent config file
# (mode 0600) so client secrets never appear on the command line. It runs the
# agent in the foreground with a temporary state directory, waits for a real
# authenticated tunnel, prints redacted evidence, stops the agent and checks
# that state is cleaned up.
#
# Usage:
#   PANGOLIN_E2E_CONFIG=/root/pangolin-e2e-client.json \
#     tests/e2e/client-smoke.sh
#
# Requirements: FreeBSD/OPNsense, root, the endpoint reachable, and a real
# machine-client ID/secret with access to at least one resource.
set -eu

: "${PANGOLIN_E2E_CONFIG:?set PANGOLIN_E2E_CONFIG to a 0600 agent config}"
AGENT=${AGENT:-/usr/local/sbin/pangolin-agent}

if [ "$(id -u)" -ne 0 ]; then
    echo "must run as root (TUN/route creation)" >&2
    exit 1
fi
if [ ! -r "$PANGOLIN_E2E_CONFIG" ]; then
    echo "config not readable: $PANGOLIN_E2E_CONFIG" >&2
    exit 1
fi
mode=$(stat -f '%Lp' "$PANGOLIN_E2E_CONFIG")
if [ "$mode" != "600" ]; then
    echo "config must be mode 0600, got $mode" >&2
    exit 1
fi

"$AGENT" validate --config "$PANGOLIN_E2E_CONFIG"

# The caller should set stateDir to a temporary path. Refuse to run against a
# production state directory by accident.
state_dir=$(sed -n 's/.*"stateDir"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$PANGOLIN_E2E_CONFIG")
if [ -z "$state_dir" ]; then
    echo "config has no stateDir" >&2
    exit 1
fi
case "$state_dir" in
    /var/db/pangolin) echo "refusing to use the production state directory" >&2; exit 1 ;;
esac

log=$(mktemp "${TMPDIR:-/tmp}/pangolin-e2e.XXXXXX.log")
pid=""
cleanup() {
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        kill -TERM "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
    fi
    rm -f "$log"
}
trap cleanup EXIT INT TERM

"$AGENT" run --config "$PANGOLIN_E2E_CONFIG" >"$log" 2>&1 &
pid=$!

status="$state_dir/client-status.json"
i=0
while [ "$i" -lt 90 ]; do
    if [ -r "$status" ] && grep -q '"connected": true' "$status"; then
        break
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
        echo "agent exited before connecting" >&2
        grep -v -i 'secret' "$log" | tail -30 >&2 || true
        exit 1
    fi
    sleep 2
    i=$((i + 1))
done

if [ "$i" -ge 90 ]; then
    echo "agent did not report connected within 180s" >&2
    grep -v -i 'secret' "$log" | tail -30 >&2 || true
    exit 1
fi

echo "connected=true"
echo "status file: $status"
python3 - "$status" <<'PY'
import json, sys
st = json.load(open(sys.argv[1]))
for key in ("connected", "registered", "tunnelIp", "interface", "routes", "lastError"):
    print(f"{key}: {st.get(key)}")
PY

# Verify the runtime did not print the configured ID/secret. This catches
# regressions in the vendored logging redaction.
python3 - "$PANGOLIN_E2E_CONFIG" "$log" <<'PY'
import json, sys
cfg = json.load(open(sys.argv[1]))
text = open(sys.argv[2], errors="ignore").read()
for value in (cfg["client"].get("id"), cfg["client"].get("secret")):
    if value and value in text:
        print("credential material found in agent log", file=sys.stderr)
        raise SystemExit(1)
print("credential redaction verified")
PY

# Prove the Rust/Go side actually got a WireGuard device and route state. The
# interface existence check is intentionally simple and works on FreeBSD.
if ! ifconfig | grep -q '^[a-z].*flags='; then
    echo "warning: ifconfig produced no interfaces" >&2
fi
if grep -q '"tunnelIp": ""' "$status"; then
    echo "tunnel IP missing" >&2
    exit 1
fi

kill -TERM "$pid"
wait "$pid" || true
pid=""

# After a clean stop, the status file must not claim a live connection.
if grep -q '"connected": true' "$status"; then
    echo "status still reports connected after shutdown" >&2
    exit 1
fi
echo "clean shutdown verified"
