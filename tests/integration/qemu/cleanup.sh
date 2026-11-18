#!/bin/sh
set -e
if [ -f /var/run/pangolin_client.pid ]; then
    kill "$(cat /var/run/pangolin_client.pid)" 2>/dev/null || true
fi
rm -f /var/run/pangolin_client.pid
cat > /var/db/pangolin/client-status.json <<'EOF'
{
  "version": "0.1.0",
  "mode": "client",
  "pid": 0,
  "startedAt": "2026-11-18T00:00:00Z",
  "updatedAt": "2026-11-18T00:00:00Z",
  "connected": false,
  "registered": false,
  "interface": "pangolin0",
  "haActive": true,
  "subnetRouter": {
    "enabled": true,
    "backend": "opnsense",
    "localNetworks": ["10.10.0.0/16"],
    "applied": false
  }
}
EOF
/usr/local/sbin/configctl pangolin firewall-sync
/usr/local/sbin/configctl pangolin dns-sync
sleep 2
echo "--- filter anchor ---"
/sbin/pfctl -a os-pangolin/fw -sr || true
echo "--- nat anchor ---"
/sbin/pfctl -a os-pangolin/nat -sn || true
echo "--- files ---"
ls -l /var/etc/pangolin /usr/local/etc/unbound.opnsense.d/os-pangolin.conf 2>&1 || true
echo "--- query (expected NXDOMAIN) ---"
/usr/local/bin/drill @127.0.0.1 new.internal.example | grep -E 'rcode|ANSWER SECTION' -A2 || true
