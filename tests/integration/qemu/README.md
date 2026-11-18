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

## Fixtures

- `fake-state.sh` — kills the real client, starts a process whose command name
  contains `pangolin-agent` to satisfy the PID check, writes a simulated
  connected status and a DNS snapshot, then runs `firewall-sync` and `dns-sync`.
  Expected: NAT/filter anchors are populated; `0.0.0.0/0` and
  `128.0.0.0/1` are ignored; the Unbound include is created.
- `dns-change.sh` — adds `new.internal.example` and re-syncs. Expected: source
  and chroot include both contain the new record and `drill` returns it.
- `cleanup.sh` — removes the dummy PID and writes a disconnected status. Expected:
  anchors are empty, the rule files and Unbound include are gone, and `drill`
  returns `NXDOMAIN`.

Run them from the VM console:

```sh
fetch -q -o /tmp/fake.sh http://10.0.2.2:8000/tests/integration/qemu/fake-state.sh
sh /tmp/fake.sh

fetch -q -o /tmp/dnschange.sh http://10.0.2.2:8000/tests/integration/qemu/dns-change.sh
sh /tmp/dnschange.sh

fetch -q -o /tmp/cleanup.sh http://10.0.2.2:8000/tests/integration/qemu/cleanup.sh
sh /tmp/cleanup.sh
```

## Expected outputs

See `docs/TESTING.md` for the exact rule set, HTTP results and DNS queries.
Any deviation is a bug or an environment difference; record it before changing
the fixtures.

## Cleanup

Power off the VM. Do not reuse this disk for production. The fixtures write
fake state under `/var/db/pangolin` and modify `/usr/local/etc/unbound.opnsense.d`.
