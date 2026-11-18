#!/bin/sh
set -e
cat > /var/db/pangolin/dns-records.json <<'EOF'
[
  {"domain": "db.internal.example", "ip": "100.64.0.2", "type": "A", "siteId": 1},
  {"domain": "*.apps.internal.example", "ip": "100.64.0.3", "type": "A", "siteId": 2},
  {"domain": "new.internal.example", "ip": "100.64.0.9", "type": "A", "siteId": 3}
]
EOF
/usr/local/sbin/configctl pangolin dns-sync
sleep 2
echo "--- source ---"
cat /usr/local/etc/unbound.opnsense.d/os-pangolin.conf
echo "--- chroot ---"
cat /var/unbound/etc/os-pangolin.conf
echo "--- query ---"
/usr/local/bin/drill @127.0.0.1 new.internal.example | grep -A2 'ANSWER SECTION'
