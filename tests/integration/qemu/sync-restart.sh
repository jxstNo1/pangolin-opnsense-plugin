#!/bin/sh
# Regression fixture: pangolin sync must restart a running agent only when the
# generated JSON actually changed. This closes the DNS-page gap where enabling
# DNS sync left the old agent without the dns hook.
set -eu

/usr/local/bin/php /tmp/configure-model.php >/dev/null
/usr/local/etc/rc.d/pangolin_client stop >/dev/null 2>&1 || true
pkill -f '/usr/local/sbin/pangolin-agent run' >/dev/null 2>&1 || true
sleep 1

wait_for_agent() {
    i=0
    while [ "$i" -lt 25 ]; do
        pid=$(pgrep -f '[p]angolin-agent run --config /usr/local/etc/pangolin/client.json' | head -1 || true)
        if [ -n "$pid" ]; then
            echo "$pid"
            return 0
        fi
        sleep 1
        i=$((i + 1))
    done
    return 1
}

/usr/local/sbin/configctl pangolin sync >/dev/null
pid1=$(wait_for_agent) || {
    echo "FAIL: agent did not start"
    cat /var/log/pangolin/client.log 2>/dev/null || true
    exit 1
}

sleep 2
/usr/local/sbin/configctl pangolin sync >/dev/null
sleep 2
pid2=$(pgrep -f '[p]angolin-agent run --config /usr/local/etc/pangolin/client.json' | head -1 || true)
if [ "$pid1" != "$pid2" ]; then
    echo "FAIL: unchanged sync restarted agent ($pid1 -> $pid2)"
    exit 1
fi
echo "unchanged sync kept agent pid $pid1"

/usr/local/bin/php /tmp/change-dns-ttl.php >/dev/null
/usr/local/sbin/configctl pangolin sync >/dev/null

i=0
pid3=""
while [ "$i" -lt 25 ]; do
    pid3=$(pgrep -f '[p]angolin-agent run --config /usr/local/etc/pangolin/client.json' | head -1 || true)
    if [ -n "$pid3" ] && [ "$pid3" != "$pid1" ]; then
        break
    fi
    sleep 1
    i=$((i + 1))
done
if [ -z "$pid3" ] || [ "$pid3" = "$pid1" ]; then
    echo "FAIL: changed configuration did not restart the agent ($pid1 -> $pid3)"
    cat /var/log/pangolin/client.log 2>/dev/null | tail -20 || true
    exit 1
fi
echo "changed config restarted agent pid $pid1 -> $pid3"

/usr/local/sbin/configctl pangolin_client stop >/dev/null
echo "sync restart behaviour: PASS"
