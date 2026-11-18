#!/bin/sh
# Regression fixture: a failed rule-file write must not leave the previous
# anchors loaded. It makes /var/etc/pangolin a file, forces the write to fail,
# and expects the plugin to flush both anchors.
set -eu

# fake.sh loads anchors from a valid simulated connected state.
sh /tmp/fake.sh >/dev/null 2>&1 || true

rm -rf /var/etc/pangolin
: > /var/etc/pangolin

/usr/local/sbin/configctl pangolin firewall-sync >/dev/null 2>&1 || true

fw=$(/sbin/pfctl -a os-pangolin/fw -sr 2>/dev/null || true)
nat=$(/sbin/pfctl -a os-pangolin/nat -sn 2>/dev/null || true)
if [ -n "$fw" ] || [ -n "$nat" ]; then
    echo "FAIL: anchors survived a rule-write error"
    printf 'fw=%s\nnat=%s\n' "$fw" "$nat"
    rm -f /var/etc/pangolin
    mkdir -p /var/etc/pangolin
    exit 1
fi

rm -f /var/etc/pangolin
mkdir -p /var/etc/pangolin
echo "write error flushed anchors: PASS"
