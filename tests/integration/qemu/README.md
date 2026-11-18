# QEMU/OPNsense integration fixtures

These scripts reproduce the 0.1.0 integration checks on an OPNsense 26.7 VM.
They are development fixtures, not production tools. Never point them at a
real firewall: they simulate a connected client and replace state files.

## Prepare the VM

1. Download `OPNsense-26.7-nano-amd64.img.bz2` from an official mirror.
2. Decompress it and convert/resize to a virtual disk:

   ```sh
   bzip2 -d OPNsense-26.7-nano-amd64.img.bz2
   qemu-img convert -f raw -O qcow2 OPNsense-26.7-nano-amd64.img opnsense.qcow2
   qemu-img resize opnsense.qcow2 16G
   ```

3. Boot with two user networks and a TCP serial console. Example:

   ```sh
   qemu-system-x86_64 -m 2048 -smp 2 -machine q35 \
     -drive file=opnsense.qcow2,if=virtio,format=qcow2 \
     -netdev user,id=wan,net=10.0.2.0/24 -device virtio-net-pci,netdev=wan \
     -netdev user,id=lan,net=192.168.1.0/24,hostfwd=tcp:127.0.0.1:8443-192.168.1.1:443,hostfwd=tcp:127.0.0.1:2222-192.168.1.1:22 \
     -device virtio-net-pci,netdev=lan \
     -display none -serial tcp:127.0.0.1:4444,server=on,wait=off
   ```

4. Assign `vtnet1` to LAN and `vtnet0` to WAN via the console, set LAN to
   `192.168.1.1/24`, and set the root password if needed.
5. Start the persistent serial bridge in another terminal:

   ```sh
   python tests/integration/qemu/console.py --log /tmp/opn-console.log \
     --commands /tmp/opn-console.cmd
   ```

   Send commands by writing them (with `\r`) to `/tmp/opn-console.cmd`.

6. Copy the plugin to the VM. From the repository root, serve it and install:

   ```sh
   python -m http.server 8000 --bind 0.0.0.0
   # in the VM console:
   fetch -q -o /tmp/os-pangolin.tgz http://10.0.2.2:8000/dist/os-pangolin-0.1.0-amd64.tgz
   tar -xzf /tmp/os-pangolin.tgz -C /tmp
   cp -R /tmp/os-pangolin-0.1.0-amd64/src/* /usr/local/
   chmod 755 /usr/local/sbin/pangolin-agent /usr/local/etc/rc.d/pangolin_*
   chmod 700 /usr/local/etc/pangolin/hook.sh
   service configd restart
   ```

   For a source-only test, tar `plugin/src` manually.

## Package lifecycle test

`build-pkg.sh` builds a real FreeBSD `.pkg` from the release tarball **inside
the OPNsense VM** with `pkg create`. `pkg-verify.sh` records the before/after
state. The package scripts are part of the test: without a metadata directory
(`-m`), `pkg create` silently omits `+POST_INSTALL`/`+POST_DEINSTALL` and the
configd actions are never registered.

```sh
# in the VM, with the repository served at http://10.0.2.2:8000
fetch -q -o /tmp/os-pangolin-0.1.0-amd64.tgz \
  http://10.0.2.2:8000/dist/os-pangolin-0.1.0-amd64.tgz
fetch -q -o /tmp/build-pkg.sh \
  http://10.0.2.2:8000/tests/integration/qemu/build-pkg.sh
sh /tmp/build-pkg.sh

sh /tmp/pkg-verify.sh pre
pkg install -y /tmp/os-pangolin-0.1.0.pkg
sh /tmp/pkg-verify.sh post-install

fetch -q -o /tmp/configure-model.php \
  http://10.0.2.2:8000/tests/integration/qemu/configure-model.php
/usr/local/bin/php /tmp/configure-model.php

fetch -q -o /tmp/service-start.sh \
  http://10.0.2.2:8000/tests/integration/qemu/service-start.sh
sh /tmp/service-start.sh

pkg delete -y os-pangolin
sh /tmp/pkg-verify.sh post-delete
```

`service-start.sh` proves that `rc.d` starts the agent binary (not
`/usr/sbin/daemon` again), that `client.json` is 0600, that an unreachable
endpoint is reported as `connected: false`, and that `stop` terminates the
agent. It does **not** prove authentication; that needs the E2E credentials in
`tests/e2e/README.md`.

`configure-model.php` uses the real OPNsense model API and
`Config::getInstance()->save()`. Do not replace the save call with
`write_config()`: that rebuilds the config singleton from the legacy array and
discards model changes made by `serializeToConfig()`.

## Fixtures

- `fake-state.sh` — kills the real client, starts a live stand-in process whose
  command and PID satisfy the fresh-status check, writes a current heartbeat
  and a simulated connected status, then runs `firewall-sync` and `dns-sync`.
  Expected: NAT/filter anchors are populated; only allowed non-local remote
  routes remain; the inbound rule carries `on <interface>`; `0.0.0.0/0` and
  `128.0.0.0/1` are ignored; the Unbound include is created.
- `dns-change.sh` — refreshes the simulated heartbeat, adds
  `new.internal.example` and re-syncs. Expected: source and chroot include both
  contain the new record and `drill` returns it.
- `cleanup.sh` — removes the dummy PID and writes a disconnected status.
  Expected: anchors are empty, rule files and the Unbound include are gone, and
  `drill` returns `NXDOMAIN`.
- `empty-routes.sh` — connected state with only default/local routes. Expected:
  both anchors empty; no `to any` rule.
- `stale-status.sh` — a live fake PID with an old heartbeat, then a fresh
  heartbeat with a dead PID. Expected: anchors empty and include absent in both
  cases.
- `firewall-error.sh` — makes `/var/etc/pangolin` a file so rule writing fails.
  Expected: the previously loaded anchors are flushed.
- `manual-zone.php add|remove` plus `manual-zone-test.sh` — creates a manual
  Unbound domain override `corp.example.com` and checks that a learned
  `db.corp.example.com` alias is not published while an unrelated alias is.
- `dns-conflict.php` — pure parent-zone/wildcard conflict cases.
- `dns-inspect-failure.sh` — temporarily removes the Unbound model class so the
  manual-entry inspection fails. Expected: source and chroot alias include are
  removed and Unbound is restarted.
- `sync-restart.sh` plus `change-dns-ttl.php` — unchanged `pangolin sync` keeps
  the agent PID; a DNS TTL change restarts it. This covers the DNS page path.

Run them from the VM console:

```sh
fetch -q -o /tmp/fake.sh http://10.0.2.2:8000/tests/integration/qemu/fake-state.sh
sh /tmp/fake.sh

fetch -q -o /tmp/dnschange.sh http://10.0.2.2:8000/tests/integration/qemu/dns-change.sh
sh /tmp/dnschange.sh

fetch -q -o /tmp/cleanup.sh http://10.0.2.2:8000/tests/integration/qemu/cleanup.sh
sh /tmp/cleanup.sh
```

Review regression fixtures:

```sh
fetch -q -o /tmp/empty-routes.sh http://10.0.2.2:8000/tests/integration/qemu/empty-routes.sh
sh /tmp/empty-routes.sh

fetch -q -o /tmp/stale.sh http://10.0.2.2:8000/tests/integration/qemu/stale-status.sh
sh /tmp/stale.sh

fetch -q -o /tmp/fwerror.sh http://10.0.2.2:8000/tests/integration/qemu/firewall-error.sh
sh /tmp/fwerror.sh

fetch -q -o /tmp/dns-conflict.php http://10.0.2.2:8000/tests/integration/qemu/dns-conflict.php
/usr/local/bin/php /tmp/dns-conflict.php

fetch -q -o /tmp/dns-inspect-failure.sh http://10.0.2.2:8000/tests/integration/qemu/dns-inspect-failure.sh
sh /tmp/dns-inspect-failure.sh

fetch -q -o /tmp/manual-zone.php http://10.0.2.2:8000/tests/integration/qemu/manual-zone.php
fetch -q -o /tmp/manual-zone-test.sh http://10.0.2.2:8000/tests/integration/qemu/manual-zone-test.sh
/usr/local/bin/php /tmp/manual-zone.php add
sh /tmp/manual-zone-test.sh
/usr/local/bin/php /tmp/manual-zone.php remove

fetch -q -o /tmp/change-dns-ttl.php http://10.0.2.2:8000/tests/integration/qemu/change-dns-ttl.php
fetch -q -o /tmp/syncrestart.sh http://10.0.2.2:8000/tests/integration/qemu/sync-restart.sh
sh /tmp/syncrestart.sh
```

## Expected outputs

See `docs/TESTING.md` for the exact rule set, HTTP results and DNS queries.
Any deviation is a bug or an environment difference; record it before changing
the fixtures.

## Cleanup

Power off the VM. Do not reuse this disk for production. The fixtures write
fake state under `/var/db/pangolin` and modify `/usr/local/etc/unbound.opnsense.d`.
