#!/bin/sh
# Conditional-forwarding regression fixture.
#
# Covers:
#  - existing plugin-owned dot is removed, a manual dot is never touched
#  - a new plugin dot is added and resolves through the test DNS server
#  - when the Unbound model cannot be loaded, the plugin-owned dot is removed
#    directly from config.xml, the manual dot remains, and Unbound restarts
set -eu

DNS_PID=""
MODEL=/usr/local/opnsense/mvc/app/models/OPNsense/Unbound/Unbound.php
MODEL_BACKUP=/tmp/Unbound.php.forwarding-backup

cleanup() {
    set +e
    trap - EXIT INT TERM
    if [ -n "$DNS_PID" ]; then
        kill "$DNS_PID" 2>/dev/null || true
    fi
    if [ -f "$MODEL_BACKUP" ]; then
        mv "$MODEL_BACKUP" "$MODEL"
    fi
    [ -f /tmp/conditional.php ] && /usr/local/bin/php /tmp/conditional.php off >/dev/null 2>&1
    [ -f /tmp/seed-forwarding.php ] && /usr/local/bin/php /tmp/seed-forwarding.php remove >/dev/null 2>&1
    [ -f /tmp/cleanup.sh ] && sh /tmp/cleanup.sh >/dev/null 2>&1
    /sbin/ifconfig lo0 inet 127.0.0.2/32 -alias >/dev/null 2>&1 || true
    /usr/local/sbin/configctl unbound restart >/dev/null 2>&1 || true
    pkill -f '/tmp/pangolin-agent' >/dev/null 2>&1 || true
    pkill -f 'dns-test-server.py' >/dev/null 2>&1 || true
    rm -f /tmp/dns-test-server.log /var/run/pangolin_client.pid
}
trap cleanup EXIT INT TERM

fail() {
    echo "FAIL: $*"
    exit 1
}

wait_for_dot() {
    domain=$1
    present=$2
    i=0
    while [ "$i" -lt 25 ]; do
        if [ "$present" = yes ]; then
            grep -q "name: \"$domain\"" /var/unbound/etc/dot.conf 2>/dev/null && return 0
        else
            ! grep -q "name: \"$domain\"" /var/unbound/etc/dot.conf 2>/dev/null && return 0
        fi
        sleep 1
        i=$((i + 1))
    done
    return 1
}

/usr/local/etc/rc.d/pangolin_client stop >/dev/null 2>&1 || true
pkill -f 'pangolin-agent' >/dev/null 2>&1 || true
pkill -f 'dns-test-server.py' >/dev/null 2>&1 || true
sleep 1
[ -f /tmp/disable-dnsbl.sh ] && sh /tmp/disable-dnsbl.sh >/dev/null
/usr/local/bin/php /tmp/seed-forwarding.php remove >/dev/null 2>&1 || true
/sbin/ifconfig lo0 inet 127.0.0.2/32 alias >/dev/null 2>&1 || true

python3 /tmp/dns-test-server.py >/tmp/dns-test-server.log 2>&1 &
DNS_PID=$!
sleep 1
if ! kill -0 "$DNS_PID" 2>/dev/null; then
    cat /tmp/dns-test-server.log 2>/dev/null || true
    fail "test DNS server did not start"
fi

sh /tmp/fake.sh >/dev/null 2>&1 || true
python3 - <<'PY'
import datetime, json
path = "/var/db/pangolin/client-status.json"
with open(path) as fh:
    data = json.load(fh)
data["dnsProxyIp"] = "127.0.0.2"
data["updatedAt"] = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
with open(path, "w") as fh:
    json.dump(data, fh)
PY

/usr/local/bin/php /tmp/seed-forwarding.php add
/usr/local/bin/php /tmp/conditional.php on

old_pid=$(cat /var/run/unbound.pid)
/usr/local/sbin/configctl pangolin dns-sync >/dev/null

wait_for_dot corp.internal yes || fail "corp.internal forward zone was not generated"
wait_for_dot manual.internal yes || fail "manual.internal forward zone disappeared"
if grep -q 'name: "old.internal"' /var/unbound/etc/dot.conf; then
    fail "old plugin forward zone was not removed"
fi

grep -q '<domain>corp.internal</domain>' /conf/config.xml || fail "corp.internal missing from config.xml"
grep -q '<domain>manual.internal</domain>' /conf/config.xml || fail "manual.internal missing from config.xml"
if grep -q '<domain>old.internal</domain>' /conf/config.xml; then
    fail "old.internal still present in config.xml"
fi
count=$(grep -c '<description>os-pangolin conditional forward</description>' /conf/config.xml || true)
[ "$count" -eq 1 ] || fail "expected exactly one plugin-owned dot, found $count"

grep -q 'corp.internal' /var/db/pangolin/unbound-dots.json || fail "state file missing corp.internal"
if grep -q 'old.internal' /var/db/pangolin/unbound-dots.json; then
    fail "state file still records old.internal"
fi

drill @127.0.0.1 corp.internal | grep -q '10.99.0.9' || fail "corp.internal did not resolve through forwarding"
drill @127.0.0.1 manual.internal | grep -q '10.99.0.9' || fail "manual.internal did not resolve through forwarding"
if drill @127.0.0.1 old.internal | grep -q '10.99.0.9'; then
    fail "old.internal still resolves through forwarding"
fi

new_pid=$(cat /var/run/unbound.pid)
[ "$new_pid" != "$old_pid" ] || fail "Unbound PID did not change after forwarding update"
echo "conditional forwarding add/remove: PASS"

# Simulate an unavailable Unbound model class.
mv "$MODEL" "$MODEL_BACKUP"
old_pid=$(cat /var/run/unbound.pid)
/usr/local/sbin/configctl pangolin dns-sync >/dev/null 2>&1 || true
wait_for_dot corp.internal no || fail "plugin forward zone survived model failure"
wait_for_dot manual.internal yes || fail "manual forward zone disappeared on model failure"

if grep -q '<domain>corp.internal</domain>' /conf/config.xml; then
    fail "corp.internal still present in config.xml after model failure"
fi
grep -q '<domain>manual.internal</domain>' /conf/config.xml || fail "manual.internal was removed during model failure"
if grep -q '<description>os-pangolin conditional forward</description>' /conf/config.xml; then
    fail "plugin-owned dot survived direct cleanup"
fi
grep -q '{}' /var/db/pangolin/unbound-dots.json || fail "state file was not cleared"

if drill @127.0.0.1 corp.internal | grep -q '10.99.0.9'; then
    fail "corp.internal still resolves after model failure cleanup"
fi
drill @127.0.0.1 manual.internal | grep -q '10.99.0.9' || fail "manual.internal stopped resolving after model failure"

new_pid=$(cat /var/run/unbound.pid)
[ "$new_pid" != "$old_pid" ] || fail "Unbound PID did not change after model-failure cleanup"
echo "model-unavailable forwarding cleanup: PASS"

mv "$MODEL_BACKUP" "$MODEL"
cleanup
echo "conditional forwarding fixture: ALL PASS"
