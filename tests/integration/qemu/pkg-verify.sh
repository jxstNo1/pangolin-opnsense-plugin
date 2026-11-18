#!/bin/sh
# Package install/uninstall verification for the isolated OPNsense test VM.
#
# Usage: pkg-verify.sh pre|post-install|post-delete
#
# The output is evidence for docs/PHASE2-VALIDATION.md. It never writes or
# prints secrets.
set -u
mode=${1:-}
PKG=/tmp/os-pangolin-0.1.0.pkg

show_files() {
    for f in \
        /usr/local/sbin/pangolin-agent \
        /usr/local/etc/rc.d/pangolin_client \
        /usr/local/etc/rc.d/pangolin_site \
        /usr/local/etc/pangolin/hook.sh \
        /usr/local/opnsense/service/conf/actions.d/actions_pangolin.conf \
        /usr/local/opnsense/service/conf/actions.d/actions_pangolin_client.conf \
        /usr/local/opnsense/service/conf/actions.d/actions_pangolin_site.conf \
        /usr/local/opnsense/mvc/app/models/OPNsense/Pangolin/Pangolin.xml \
        /usr/local/opnsense/mvc/app/controllers/OPNsense/Pangolin/Api/ServiceController.php \
        /usr/local/opnsense/mvc/app/views/OPNsense/Pangolin/overview.volt \
        /usr/local/etc/inc/plugins.inc.d/pangolin.inc
    do
        if [ -e "$f" ]; then
            stat -f 'present %Sp %Su:%Sg %N' "$f"
        else
            echo "absent  $f"
        fi
    done
}

show_state() {
    echo "--- pf anchors ---"
    /sbin/pfctl -a os-pangolin/fw -sr 2>&1 | sed 's/^/fw: /'
    /sbin/pfctl -a os-pangolin/nat -sn 2>&1 | sed 's/^/nat: /'
    echo "--- unbound include ---"
    if [ -e /usr/local/etc/unbound.opnsense.d/os-pangolin.conf ]; then
        echo "present /usr/local/etc/unbound.opnsense.d/os-pangolin.conf"
    else
        echo "absent /usr/local/etc/unbound.opnsense.d/os-pangolin.conf"
    fi
    echo "--- runtime state ---"
    ls -la /var/db/pangolin 2>&1 | sed 's/^/db: /'
    pgrep -lf pangolin-agent 2>/dev/null | sed 's/^/pid: /' || echo "no pangolin-agent process"
}

case "$mode" in
pre)
    echo "=== pre-install baseline ==="
    show_files
    show_state
    if pkg info -e os-pangolin; then
        echo "os-pangolin already installed"
    else
        echo "os-pangolin not installed"
    fi
    ;;
post-install)
    echo "=== post-install verification ==="
    pkg info os-pangolin
    show_files
    echo "--- agent binary ---"
    file /usr/local/sbin/pangolin-agent
    /usr/local/sbin/pangolin-agent version
    echo "--- configd action ---"
    /usr/local/sbin/configctl pangolin_client status
    echo "--- web UI route (loopback TLS, unauthenticated redirect expected) ---"
    curl -k -s -o /dev/null -w 'http_code=%{http_code} redirect=%{redirect_url}\n' \
        --max-time 10 https://127.0.0.1/ui/pangolin/overview || true
    show_state
    ;;
post-delete)
    echo "=== post-delete verification ==="
    if pkg info -e os-pangolin; then
        echo "FAIL: os-pangolin still registered"
    else
        echo "os-pangolin package registration removed"
    fi
    show_files
    show_state
    ;;
*)
    echo "usage: $0 pre|post-install|post-delete" >&2
    exit 2
    ;;
esac
