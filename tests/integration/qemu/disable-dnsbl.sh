#!/bin/sh
# Test-VM workaround: the nano image used for these fixtures references a
# python dnsbl module that is not installed. Remove that module from the
# generated Unbound configuration so the forwarding/DNS fixtures can run.
# This patches the disposable test VM only, not the plugin or production code.
set -eu

UNBOUND_INC=/usr/local/etc/inc/plugins.inc.d/unbound.inc
if grep -q 'module-config: "{\$module_config}"' "$UNBOUND_INC" || grep -q '^{\$bindints}$' "$UNBOUND_INC"; then
    sed -i '' \
        -e 's|module-config: "{\$module_config}"|module-config: "iterator"|' \
        -e '/^python:$/d' \
        -e '/^python-script: unbound-dnsbl\/dnsbl_module.py$/d' \
        -e 's|^{\$bindints}$|interface: 127.0.0.1|' \
        "$UNBOUND_INC"
    echo "patched unbound.inc to remove missing dnsbl python module and bind localhost only"
else
    echo "unbound.inc already patched"
fi
