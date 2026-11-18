#!/bin/sh
# Verify that the packaged rc.d service really starts the agent binary.
#
# This test does not need a live Pangolin endpoint. It proves the daemon/agent
# command split, the 0600 config generation and a clean stop. It deliberately
# fails if the daemon log contains "daemon: run: No such file or directory",
# which was the regression caused by passing ${procname} as daemon's child.
set -eu

# Remove stale log content from earlier package versions so the daemon-child
# assertion below only covers this start.
rm -f /var/log/pangolin/client.log
configctl pangolin sync >/dev/null

i=0
while [ "$i" -lt 20 ]; do
    if pgrep -f '[p]angolin-agent run --config /usr/local/etc/pangolin/client.json' >/dev/null 2>&1; then
        break
    fi
    sleep 1
    i=$((i + 1))
done

if ! pgrep -f '[p]angolin-agent run --config /usr/local/etc/pangolin/client.json' >/dev/null 2>&1; then
    echo "FAIL: agent process did not start"
    cat /var/log/pangolin/client.log 2>/dev/null || true
    exit 1
fi

echo "agent process started:"
ps auxw | grep '[p]angolin-agent run --config' || true

mode=$(stat -f '%Lp' /usr/local/etc/pangolin/client.json)
if [ "$mode" != "600" ]; then
    echo "FAIL: client.json mode is $mode, expected 600"
    exit 1
fi
echo "client.json mode: $mode"

if grep -q 'daemon: run: No such file or directory' /var/log/pangolin/client.log 2>/dev/null; then
    echo "FAIL: daemon launched the wrong child command"
    exit 1
fi
echo "daemon child command: correct"

# The invalid test endpoint must not be reported as connected.
sleep 8
if [ -r /var/db/pangolin/client-status.json ]; then
    if grep -q '"connected": true' /var/db/pangolin/client-status.json; then
        echo "FAIL: unreachable test endpoint reported connected"
        exit 1
    fi
    echo "status: $(tr -d '\n' < /var/db/pangolin/client-status.json | cut -c1-200)"
else
    echo "status: not written yet (endpoint retry in progress)"
fi

configctl pangolin_client stop >/dev/null
sleep 2
if pgrep -f '[p]angolin-agent run --config /usr/local/etc/pangolin/client.json' >/dev/null 2>&1; then
    echo "FAIL: agent still running after stop"
    exit 1
fi
echo "clean stop verified"
