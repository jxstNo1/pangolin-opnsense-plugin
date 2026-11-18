#!/bin/sh
# Integration fixture for manual Unbound zone conflicts: a learned alias below
# a manually managed domain override must not be published.
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

cat > /var/db/pangolin/dns-records.json <<'EOF'
[
  {"domain": "db.corp.example.com", "ip": "100.64.0.2", "type": "A", "siteId": 1},
  {"domain": "allowed.internal.example", "ip": "100.64.0.3", "type": "A", "siteId": 2}
]
EOF

/usr/local/sbin/configctl pangolin dns-sync >/dev/null
include=/usr/local/etc/unbound.opnsense.d/os-pangolin.conf
if grep -q 'db.corp.example.com' "$include" 2>/dev/null; then
    echo "FAIL: alias under manual zone was published"
    cat "$include"
    exit 1
fi
if ! grep -q 'allowed.internal.example' "$include" 2>/dev/null; then
    echo "FAIL: non-conflicting alias was not published"
    cat "$include" 2>/dev/null || true
    exit 1
fi
echo "manual parent zone blocked subdomain; allowed alias published: PASS"
