#!/bin/sh
# Regression fixture: if manual Unbound entries cannot be inspected, the DNS
# sync must remove the existing alias include (source and chroot) instead of
# leaving stale aliases active.
set -eu

sh /tmp/fake.sh >/dev/null 2>&1 || true
python3 - <<'PY'
import datetime, json
path = "/var/db/pangolin/client-status.json"
with open(path) as fh:
    data = json.load(fh)
data["updatedAt"] = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
with open(path, "w") as fh:
    json.dump(data, fh)
PY

include=/usr/local/etc/unbound.opnsense.d/os-pangolin.conf
chroot=/var/unbound/etc/os-pangolin.conf
mkdir -p "$(dirname "$include")" "$(dirname "$chroot")"
cat > "$include" <<'EOF'
# stale include from a previous run
local-data: "stale.internal.example. 60 IN A 100.64.0.9"
EOF
cp "$include" "$chroot"

model=/usr/local/opnsense/mvc/app/models/OPNsense/Unbound/Unbound.php
backup=/tmp/Unbound.php.inspection-backup
mv "$model" "$backup"
trap 'mv "$backup" "$model" 2>/dev/null || true' EXIT INT TERM

/usr/local/sbin/configctl pangolin dns-sync >/dev/null 2>&1 || true

if [ -f "$include" ]; then
    echo "FAIL: source alias include survived inspection failure"
    cat "$include"
    exit 1
fi
if [ -f "$chroot" ]; then
    echo "FAIL: chroot alias include survived inspection failure"
    cat "$chroot"
    exit 1
fi
echo "inspection failure removed source and chroot include: PASS"
