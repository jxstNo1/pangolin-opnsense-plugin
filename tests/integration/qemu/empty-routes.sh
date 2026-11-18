#!/bin/sh
# Regression fixture: a connected tunnel with only default-route-equivalent or
# local-interface "remote" routes must leave both anchors empty.
set -eu

sh /tmp/fake.sh >/dev/null 2>&1 || true

python3 - <<'PY'
import datetime, json
path = "/var/db/pangolin/client-status.json"
with open(path) as fh:
    data = json.load(fh)
data["updatedAt"] = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
data["routes"] = ["0.0.0.0/0", "0.0.0.0/1", "128.0.0.0/1", "192.168.1.0/24", "10.0.2.0/24"]
with open(path, "w") as fh:
    json.dump(data, fh)
PY

/usr/local/sbin/configctl pangolin firewall-sync >/dev/null 2>&1 || true

fw=$(/sbin/pfctl -a os-pangolin/fw -sr 2>/dev/null || true)
nat=$(/sbin/pfctl -a os-pangolin/nat -sn 2>/dev/null || true)
if [ -n "$fw" ] || [ -n "$nat" ]; then
    echo "FAIL: empty/blocked route set produced rules"
    printf 'fw=%s\nnat=%s\n' "$fw" "$nat"
    exit 1
fi
echo "empty/blocked route set: anchors empty PASS"
