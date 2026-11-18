#!/bin/sh
# Regression fixture: a stale Unbound include must be removed when the agent
# restarts with zero current aliases. The agent sends the DNS hook on startup.
set -eu

/usr/local/etc/rc.d/pangolin_client stop >/dev/null 2>&1 || true
mkdir -p /usr/local/etc/unbound.opnsense.d
cat > /usr/local/etc/unbound.opnsense.d/os-pangolin.conf <<'EOF'
# stale include from a previous run
local-data: "stale.example.com. 60 IN A 100.64.0.9"
EOF

/usr/local/etc/rc.d/pangolin_client start >/dev/null
i=0
while [ "$i" -lt 20 ]; do
    if [ ! -f /usr/local/etc/unbound.opnsense.d/os-pangolin.conf ]; then
        break
    fi
    sleep 1
    i=$((i + 1))
done
if [ -f /usr/local/etc/unbound.opnsense.d/os-pangolin.conf ]; then
    echo "FAIL: stale Unbound include survived agent startup"
    cat /usr/local/etc/unbound.opnsense.d/os-pangolin.conf
    /usr/local/etc/rc.d/pangolin_client stop >/dev/null 2>&1 || true
    exit 1
fi
/usr/local/etc/rc.d/pangolin_client stop >/dev/null
echo "stale include removed on agent startup: PASS"
