#!/bin/sh
# Regression fixture: a live daemon supervisor must not keep a dead or stale
# agent "connected". The plugin must flush anchors and remove DNS aliases when
# the status PID is dead or the heartbeat is old.
set -eu

/usr/local/etc/rc.d/pangolin_client stop >/dev/null 2>&1 || true
pkill -f 'pangolin-agent' >/dev/null 2>&1 || true
sleep 1

cat > /tmp/pangolin-agent <<'EOF'
#!/bin/sh
sleep 600
EOF
chmod 755 /tmp/pangolin-agent
/tmp/pangolin-agent &
fake_pid=$!
echo "$fake_pid" > /var/run/pangolin_client.pid

mkdir -p /var/db/pangolin
cat > /var/db/pangolin/dns-records.json <<'EOF'
[
  {"domain": "stale.internal.example", "ip": "100.64.0.2", "type": "A", "siteId": 1}
]
EOF

check_flushed() {
    if [ -n "$(/sbin/pfctl -a os-pangolin/fw -sr 2>/dev/null || true)" ]; then
        echo "FAIL: filter anchor still populated"
        exit 1
    fi
    if [ -n "$(/sbin/pfctl -a os-pangolin/nat -sn 2>/dev/null || true)" ]; then
        echo "FAIL: NAT anchor still populated"
        exit 1
    fi
    if [ -f /usr/local/etc/unbound.opnsense.d/os-pangolin.conf ]; then
        echo "FAIL: Unbound include still present"
        exit 1
    fi
}

# Case 1: live PID, stale heartbeat.
cat > /var/db/pangolin/client-status.json <<EOF
{
  "version": "0.1.0",
  "mode": "client",
  "pid": $fake_pid,
  "startedAt": "2020-01-01T00:00:00Z",
  "updatedAt": "2020-01-01T00:00:00Z",
  "connected": true,
  "registered": true,
  "tunnelIp": "100.64.0.2",
  "interface": "pangolin0",
  "routes": ["192.168.50.0/24"],
  "subnetRouter": {
    "enabled": true,
    "backend": "opnsense",
    "tunnelIp": "100.64.0.2",
    "localNetworks": ["192.168.1.0/24"],
    "applied": true
  }
}
EOF
/usr/local/sbin/configctl pangolin firewall-sync >/dev/null 2>&1 || true
/usr/local/sbin/configctl pangolin dns-sync >/dev/null 2>&1 || true
check_flushed
echo "stale heartbeat rejected: anchors empty, include absent"

# Case 2: fresh heartbeat, dead PID.
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
cat > /var/db/pangolin/client-status.json <<EOF
{
  "version": "0.1.0",
  "mode": "client",
  "pid": 999999,
  "startedAt": "$now",
  "updatedAt": "$now",
  "connected": true,
  "registered": true,
  "tunnelIp": "100.64.0.2",
  "interface": "pangolin0",
  "routes": ["192.168.50.0/24"],
  "subnetRouter": {
    "enabled": true,
    "backend": "opnsense",
    "tunnelIp": "100.64.0.2",
    "localNetworks": ["192.168.1.0/24"],
    "applied": true
  }
}
EOF
/usr/local/sbin/configctl pangolin firewall-sync >/dev/null 2>&1 || true
/usr/local/sbin/configctl pangolin dns-sync >/dev/null 2>&1 || true
check_flushed
echo "dead agent PID rejected: anchors empty, include absent"

kill "$fake_pid" 2>/dev/null || true
rm -f /tmp/pangolin-agent /var/run/pangolin_client.pid
