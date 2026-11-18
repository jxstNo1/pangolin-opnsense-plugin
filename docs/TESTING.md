# Testing

This document separates what was actually executed from what still requires a
live lab. No result below is invented; commands and outputs were produced on the
development host and an OPNsense 26.7 QEMU VM.

## 1. Go unit tests

```sh
cd agent
go test ./...
go vet ./...
```

Result (2026-11-18): all packages pass; `go vet` clean. Covered:

- config defaults, absolute paths, endpoint scheme, secret/interface/duration
  validation, default-route rejection, broad-prefix rejection, overlap and
  insecure-permission handling;
- atomic status/DNS writes, stale/missing health file handling;
- pf NAT/filter rule text, source scoping, default-route and newline rejection;
- CARP output parsing (`MASTER`, `BACKUP`, `INIT`, missing CARP);
- route normalization and DNS snapshot sorting.

## 2. Cross-compilation

```sh
GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build ./...
GOOS=freebsd GOARCH=arm64 CGO_ENABLED=0 go build ./...
```

Result: both pass. The produced amd64 binary is a FreeBSD ELF
(`7F 45 4C 46 02 01 01 09`), 25,313,450 bytes with `-s -w`.

The binary was copied to an OPNsense 26.7/FreeBSD 15.1 VM and executed:

```
# /usr/local/sbin/pangolin-agent version
os-pangolin-agent 0.1.0 (commit dev, built 2026-11-18)
# /usr/local/sbin/pangolin-agent validate --config /usr/local/etc/pangolin/site.json
configuration OK: mode=site stateDir=/var/db/pangolin
```

## 3. PHP and XML checks

```sh
python3 scripts/check-php.py "$(command -v php)"
python3 scripts/check-xml.py
python3 scripts/check-rc.py
python3 scripts/check-redaction.py
```

Result: all plugin PHP files pass `php -l` (PHP 8.4.27), all plugin XML
files parse, the rc.d command split passes its regression check and the
vendored logging redactions are still present. The model was additionally
exercised by the OPNsense MVC runtime, which catches field-type/mask errors
that a plain XML parse cannot.

## 4. QEMU/OPNsense integration

### Image and VM

- OPNsense 26.7 nano image (`OPNsense-26.7-nano-amd64.img.bz2`), expanded to a
  16 GiB qcow2 disk.
- QEMU 11.1.0, 2 vCPU, 2 GiB RAM, virtio disk/NIC.
- WAN on slirp `10.0.2.0/24`, LAN on slirp `192.168.1.0/24` with host forwards
  for 443/80/22.
- Console over QEMU's TCP serial server; a persistent bridge
  (`tests/integration/qemu/console.py`) avoids reconnect races.

The exact launch shape:

```text
qemu-system-x86_64 -m 2048 -smp 2 -machine q35 \
  -drive file=opnsense.qcow2,if=virtio,format=qcow2 \
  -netdev user,id=wan,net=10.0.2.0/24 -device virtio-net-pci,netdev=wan \
  -netdev user,id=lan,net=192.168.1.0/24,hostfwd=tcp:127.0.0.1:8443-192.168.1.1:443,... \
  -device virtio-net-pci,netdev=lan \
  -display none -serial tcp:127.0.0.1:4444,server=on,wait=off
```

### Plugin installation

The plugin source tree was tarred from `plugin/src` and extracted into
`/usr/local`:

```sh
tar -xzf /tmp/plugin.tgz -C /usr/local
chmod 755 /usr/local/sbin/pangolin-agent /usr/local/etc/rc.d/pangolin_*
chmod 700 /usr/local/etc/pangolin/hook.sh
service configd restart
rm -f /var/lib/php/tmp/opnsense_menu_cache.xml /var/lib/php/tmp/opnsense_acl_cache.json
configctl webgui restart
```

### Results

| Check | Result |
|-------|--------|
| Menu appeared under VPN → Pangolin | yes, all seven entries |
| `/ui/pangolin/overview` | HTTP 200, 125,332 bytes |
| `/ui/pangolin/clients` | HTTP 200, 139,630 bytes |
| `/ui/pangolin/sites` | HTTP 200, 136,554 bytes |
| `/ui/pangolin/subnet` | HTTP 200, 133,031 bytes |
| `/ui/pangolin/dns` | HTTP 200, 134,454 bytes |
| `/ui/pangolin/settings` | HTTP 200, 135,068 bytes |
| `/ui/pangolin/logs` | HTTP 200, 121,553 bytes |
| No PHP fatal/parse errors in page bodies | yes |
| `/api/pangolin/settings/get` | HTTP 200, complete `pangolin` JSON |
| Valid settings POST | `{"result":"saved"}` |
| `0.0.0.0/0` local network | rejected: `"0.0.0.0/0" is invalid. Use IPv4 CIDRs with a prefix between /8 and /32` |
| `http://` endpoint without override | rejected: `Machine client: endpoint must use https ...` |
| `configctl pangolin sync` | writes config, syncs DNS/firewall, starts enabled services |
| Service start/stop/status | `service pangolin_client start` → running; `stop` → not running |
| `client.json` mode | 0600, contains expected client/subnet/DNS/HA sections |
| Client runtime | starts on FreeBSD, creates shared UDP socket, retries endpoint, writes status |
| Site runtime | starts on FreeBSD with netstack, writes status, retries endpoint |
| pf anchors registered | `anchor "os-pangolin/fw" all`, `nat-anchor "os-pangolin/nat" all` |
| Simulated connected state | generated and loaded the expected rules (below) |
| Remote allow-list | `remoteNetworks=10.20.0.0/16` filtered out `192.168.50.0/24`; only `10.20.0.0/16` remained in NAT/filter |
| Remote/local overlap | rejected: `remote network 10.10.128.0/17 overlaps local network 10.10.0.0/16` |
| Disconnect cleanup | anchors empty, rule files removed |
| Unbound include | generated, copied to `/var/unbound/etc`, resolves alias |
| Alias update | new record appears in source + chroot, `drill` returns it |
| Disconnect DNS cleanup | include removed, `drill` returns NXDOMAIN |
| Unbound restart | direct kill + `start.sh`, no configd deadlock |

Simulated connected rule set (`10.10.0.0/16` local, routes
`10.20.0.0/16` + `192.168.50.0/24`, default route deliberately supplied and
ignored):

```text
# pfctl -a os-pangolin/nat -sn
nat on pangolin0 inet from 10.10.0.0/16 to 10.20.0.0/16 -> 100.64.0.2
nat on pangolin0 inet from 10.10.0.0/16 to 192.168.50.0/24 -> 100.64.0.2

# pfctl -a os-pangolin/fw -sr
pass out quick on pangolin0 inet from 10.10.0.0/16 to 10.20.0.0/16 flags S/SA keep state
pass out quick on pangolin0 inet from 10.10.0.0/16 to 192.168.50.0/24 flags S/SA keep state
pass in quick inet from 10.10.0.0/16 to 10.20.0.0/16 flags S/SA keep state
pass in quick inet from 10.10.0.0/16 to 192.168.50.0/24 flags S/SA keep state
```

No `0.0.0.0/0`, no `128.0.0.0/1`, no `from any` source rule, no inter-VLAN rule.

Generated Unbound include for simulated aliases:

```text
# Managed by os-pangolin. Do not edit; changes are overwritten.
# Alias records are learned from the running Pangolin machine client.
local-data: "db.internal.example. 60 IN A 100.64.0.2"
local-zone: "apps.internal.example." redirect
local-data: "apps.internal.example. 60 IN A 100.64.0.3"
```

`drill @127.0.0.1 db.internal.example` → `100.64.0.2`;
`drill @127.0.0.1 x.apps.internal.example` → `100.64.0.3`; after adding
`new.internal.example` and syncing, the same query returned `100.64.0.9`; after
disconnect cleanup it returned `NXDOMAIN`.

### Bug found and fixed during integration

- `mwexec` is undefined on OPNsense 26.7; replaced with `mwexecf`/`mwexecfb`.
- Calling `pluginctl`/`configctl` from inside a configd hook deadlocked
  configd. Unbound is now stopped with a direct signal and started with
  `start.sh`.
- Function-name mismatches (`pangolin_sync_dns_do` vs
  `pangolin_dns_sync_do`, `pangolin_sync_firewall_do` vs
  `pangolin_firewall_sync_do`) were caught by the new try/catch and fixed.
- Starting a long-lived daemon through `mwexecf` hung because the supervisor
  inherited the output pipe. Service start/stop now uses `mwexecfb`.
- `rc.subr` checked the agent command while the pidfile held the daemon
  supervisor PID. `procname` is now `/usr/sbin/daemon`.
- PID-only liveness checks were fooled by stale/recycled PIDs. The plugin now
  verifies the PID command name.

## 4.1 Phase 2 package and service validation (2026-11-18)

A second, fresh OPNsense 26.7 VM was used to build a real `.pkg` with
`pkg create`, install it, run the packaged agent, and uninstall it. The full
matrix, commands and log excerpts are in `docs/PHASE2-VALIDATION.md`.

Result summary:

| Check | Result |
|-------|--------|
| `pkg create` inside the VM | `os-pangolin-0.1.0.pkg`, FreeBSD:15:amd64, 24.2 MiB flat |
| Package scripts included | `+POST_INSTALL`/`+POST_DEINSTALL` present and executed |
| `pkg install` | files, modes and configd actions correct |
| Packaged agent start | real process `/usr/local/sbin/pangolin-agent run --config ...` |
| Agent config | `client.json` mode 0600; secret absent from argv and logs |
| Unreachable endpoint | `connected:false`; anchors/DNS not opened; clean stop |
| pf/Unbound fixtures on installed package | scoped NAT/pass rules, aliases resolve, public DNS untouched, disconnect cleanup |
| `pkg delete` | package files, actions, anchors, Unbound include and runtime dirs removed |

The first package attempt exposed two packaging bugs: `pkg create` had silently
left out the `+*_INSTALL` scripts because no metadata directory was supplied,
and both rc.d scripts used `${procname}` (the daemon) as daemon's child command,
so the agent could not start (`daemon: run: No such file or directory`). Both
are fixed; `scripts/check-rc.py` guards the command split.

## 5. Not yet tested

- Authenticated connection to a real Pangolin server (Cloud or self-hosted).
- WireGuard handshake, ping, hole punching, relay fallback.
- Live site resource exposure.
- Live machine-client alias set and private HTTPS resource (TLS/SNI/Host).
- Conditional forwarding against a live Pangolin private resolver; the model
  manipulation/cleanup code was reviewed and linted, but no live resolver was
  available.
- LAN device → OPNsense → remote resource data plane with a real tunnel.
- Two-node CARP failover.
- IPv6.
- OPNsense ports `make package` in a clean ports tree. A real FreeBSD `.pkg`
  was built with `pkg create` and tested; the ports-tree build path is separate
  and still untested.
- Browser E2E: the test VM used a self-signed web GUI certificate; browser
  automation correctly refused it. UI pages were validated with authenticated
  HTTP(S) requests instead.

## 6. Real-world acceptance checklist

Before calling 0.1.0 production-ready:

1. Create a Pangolin site and a machine client; record the credentials.
2. Configure the site connector; verify local resources appear in Pangolin.
3. Configure the machine client; verify `connected: true` and `tunnelIp`.
4. From a LAN host in a selected VLAN, reach a remote private resource by IP
   and by alias.
5. Verify source NAT (`tcpdump`/`pfctl -ss`) and return traffic.
6. Verify an unselected VLAN cannot reach the resource.
7. Verify public HTTPS resources still resolve publicly and their TLS/SNI works.
8. Kill the agent; verify NAT/DNS cleanup and restart.
9. Reboot the firewall; verify automatic start and reconnect.
10. Test CARP failover and stale-PID behavior.
11. Run a 24-hour reconnect/soak test.
12. Record all results in this file.

## 7. Reproducible fixtures

`tests/integration/qemu/` contains the console bridge, package builder and
verifier, model fixture, service-start test, simulated state fixture, DNS
update fixture and cleanup fixture used above. They are development tools, not
production scripts. See its README for the sequence. The live-server harness is
in `tests/e2e/` and is not run by CI.
