#!/bin/sh
# Integration fixture: simulate a connected machine client, then verify that
# OPNsense-owned pf anchors and the Unbound include are generated safely.
set -e

/usr/local/etc/rc.d/pangolin_client stop >/dev/null 2>&1 || true
pkill -f 'pangolin-agent' >/dev/null 2>&1 || true
sleep 2
rm -f /var/run/pangolin_client.pid
sleep 1

# Satisfy Pangolin::serviceRunning() without running the real tunnel. The
# plugin verifies the pid's command name, so use a wrapper that contains the
# expected string.
cat > /tmp/pangolin-agent <<'EOF'
#!/bin/sh
sleep 600
EOF
chmod 755 /tmp/pangolin-agent
/usr/sbin/daemon -f -p /var/run/pangolin_client.pid /tmp/pangolin-agent
sleep 1
cat /var/run/pangolin_client.pid
ps -p "$(cat /var/run/pangolin_client.pid)" -o pid,command

cat > /var/db/pangolin/client-status.json <<'EOF'
{
  "version": "0.1.0",
  "mode": "client",
  "pid": 0,
  "startedAt": "2026-11-18T00:00:00Z",
  "updatedAt": "2026-11-18T00:00:00Z",
  "connected": true,
  "registered": true,
  "tunnelIp": "100.64.0.2",
  "interface": "pangolin0",
  "dnsProxyIp": "100.64.0.3",
  "routes": ["10.20.0.0/16", "192.168.50.0/24", "0.0.0.0/0", "128.0.0.0/1"],
  "haActive": true,
  "subnetRouter": {
    "enabled": true,
    "backend": "opnsense",
    "tunnelIp": "100.64.0.2",
    "localNetworks": ["10.10.0.0/16"],
    "applied": true
  }
}
EOF

cat > /var/db/pangolin/dns-records.json <<'EOF'
[
  {"domain": "db.internal.example", "ip": "100.64.0.2", "type": "A", "siteId": 1},
  {"domain": "*.apps.internal.example", "ip": "100.64.0.3", "type": "A", "siteId": 2}
]
EOF

/usr/local/sbin/configctl pangolin firewall-sync
/usr/local/sbin/configctl pangolin dns-sync

echo "--- filter anchor ---"
/sbin/pfctl -a os-pangolin/fw -sr
echo "--- nat anchor ---"
/sbin/pfctl -a os-pangolin/nat -sn
echo "--- unbound include ---"
cat /usr/local/etc/unbound.opnsense.d/os-pangolin.conf
echo "--- unbound check ---"
/usr/local/sbin/unbound-checkconf /var/unbound/unbound.conf
