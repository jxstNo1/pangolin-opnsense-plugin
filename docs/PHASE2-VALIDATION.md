# Phase 2 validation — 18 November 2026

This document records what was actually executed for Phase 2. It separates
executed tests from tests that still need a live Pangolin instance and lab
network. Nothing in the "blocked" table was simulated or reported as passed.

Source state: commit `e9d5701` (on top of `a5e6b84`).

## Environment

- Development host: Windows, QEMU 11.1.0, Git for Windows, Go 1.26, PHP 8.4.
- Test firewall: OPNsense 26.7 nano, FreeBSD 15.1, amd64, 2 vCPU, 2 GiB RAM,
  16 GiB qcow2 disk, virtio disk/NIC.
- Networking: QEMU slirp. WAN `10.0.2.0/24`; LAN `192.168.1.0/24` with host
  forwards. The LAN interface received `192.168.1.15` from slirp DHCP instead
  of the configured `.1`; all package tests were therefore driven through the
  serial console. This does not affect package contents or pf/Unbound results.
- Console: QEMU TCP serial server plus `tests/integration/qemu/console.py`.
- Package: built **inside the VM** from
  `dist/os-pangolin-0.1.0-amd64.tgz` with `pkg create`
  (`tests/integration/qemu/build-pkg.sh`). This is a real FreeBSD package, not
  a simulated install.

## Test matrix

### Executed and passed

| # | Test | Evidence |
|---|------|----------|
| 1 | Upstream pin parity with official CLI | `fosrl/cli` and this repo both use `newt v1.20.0` / `olm v1.10.2`; machine-client path skips fingerprint when ID/secret are supplied |
| 2 | Go tests, vet, FreeBSD cross-build | `go test ./...` pass, `go vet ./...` clean, `GOOS=freebsd GOARCH=amd64/arm64 CGO_ENABLED=0 go build ./...` pass |
| 3 | PHP, XML and rc.d checks | `php lint failures: 0`, `xml parse failures: 0`, `rc.d script failures: 0` |
| 4 | Credential logging regression | agent debug log with a fixture secret contains 0 occurrences of the secret; token request body, bearer token, WireGuard key and full OLM config are redacted |
| 5 | Real `.pkg` build | `pkg create` produced `os-pangolin-0.1.0.pkg`, 7,710,651 bytes, `FreeBSD:15:amd64`, 24.2 MiB flat |
| 6 | Package scripts are present | `tar -tf` shows `+POST_INSTALL`/`+POST_DEINSTALL`; post-install created runtime dirs and registered configd actions |
| 7 | `pkg install` and file modes | binary `0755`, `hook.sh` `0700`, actions/model/controllers/views present |
| 8 | Packaged agent runs on FreeBSD | `file` shows FreeBSD ELF; `pangolin-agent version` → `0.1.0` |
| 9 | configd actions registered | `configctl pangolin_client status` returns agent JSON without "Action not allowed" |
| 10 | UI route available | `https://127.0.0.1/ui/pangolin/overview` → 302 to login, correct URL |
| 11 | Model save through OPNsense API | `Config::getInstance()->save()` persisted `<pangolin>` in `config.xml`; model reads client/subnet/DNS flags as `1` |
| 12 | Agent config generation | `configctl pangolin sync` wrote `client.json` mode `0600`; agent start argv contains only `--config`, no secret |
| 13 | rc.d service starts the real agent | process `/usr/local/sbin/pangolin-agent run --config ...`; no `daemon: run: No such file` |
| 14 | Unreachable endpoint fails closed | status `connected:false`; no anchors; no Unbound include; agent retries and stops cleanly |
| 15 | pf anchors with simulated connected state | NAT + scoped pass rules for `10.10.0.0/16 → 192.168.50.0/24`, SNAT to `100.64.0.2` |
| 16 | Default-route and allow-list rejection | status supplied `0.0.0.0/0`, `128.0.0.0/1`, `10.20.0.0/16`, `192.168.50.0/24`; only the allowed `192.168.50.0/24` survived |
| 17 | pf rules survive filter reload | `configctl filter reload` → `OK`; both anchors unchanged |
| 18 | Unbound aliases | `db.internal.example → 100.64.0.2`, wildcard `apps.internal.example → 100.64.0.3` |
| 19 | Public DNS is not rewritten | `example.com` resolved to public `104.20.23.154` / `172.66.147.243`, not a tunnel address |
| 20 | Unbound restart persistence | `configctl unbound restart` → aliases still resolve |
| 21 | DNS update | new alias `new.internal.example → 100.64.0.9` appears in source and chroot include and resolves |
| 22 | Disconnect cleanup | anchors empty, rule files gone, include removed, `new.internal.example` → `NXDOMAIN` |
| 23 | `pkg delete` cleanup | package files, configd actions, anchors, Unbound include and runtime state removed; non-empty log directory retained for diagnostics |
| 24 | Clean stop | `configctl pangolin_client stop` → `OK`; no agent process remains |

### Blocked — requires authorized Pangolin instance and lab access

| # | Test | Why it is blocked |
|---|------|-------------------|
| B1 | Machine-client token exchange and authentication | no authorized endpoint, client ID/secret |
| B2 | WireGuard handshake and keepalive | depends on B1 |
| B3 | NAT traversal / hole punching | depends on B1 and two NATed peers |
| B4 | Relay fallback | depends on B1 and a relay-forced path |
| B5 | TCP/UDP access to a private Pangolin resource | needs a real resource IP/host and ports |
| B6 | LAN device → OPNsense → resource data plane | needs a separate test client on an isolated VLAN |
| B7 | Unselected VLAN is blocked | needs the second VLAN/device above |
| B8 | Live alias set plus HTTPS/TLS/SNI/Host | needs a real private HTTPS resource and its certificate name |
| B9 | Reconnect after network loss / agent crash | needs a real tunnel and controlled link failure |
| B10 | CARP failover | needs two OPNsense nodes with CARP and a VIP |
| B11 | 24-hour soak | follows B1–B5 |
| B12 | OPNsense ports `make package` in a clean ports tree | package was built with `pkg create` in the VM; ports-tree build is separate |
| B13 | Browser E2E with trusted certificate | test VM uses a self-signed web GUI certificate; pages were checked with HTTP(S) requests |

## Log excerpts

### Package build and install

```text
-rw-r--r--  1 root wheel 7710651 Nov 18 07:40 /tmp/os-pangolin-0.1.0.pkg
Name           : os-pangolin
Version        : 0.1.0
Origin         : opnsense/os-pangolin
Architecture   : FreeBSD:15:amd64
Prefix         : /usr/local
Flat size      : 24.2MiB
```

```text
present -rwxr-xr-x root:wheel /usr/local/sbin/pangolin-agent
present -rwx------ root:wheel /usr/local/etc/pangolin/hook.sh
present -rw-r--r-- root:wheel /usr/local/opnsense/service/conf/actions.d/actions_pangolin.conf
/usr/local/sbin/pangolin-agent: ELF 64-bit LSB executable, x86-64,
  version 1 (FreeBSD), statically linked, for FreeBSD 12.3
os-pangolin-agent 0.1.0 (commit unknown, built 2026-11-18T07:39:57Z)
```

### Service start (real packaged agent, unreachable test endpoint)

```text
root    46886 ... /usr/local/sbin/pangolin-agent run --config /usr/local/etc/pangolin/client.json
client.json mode: 600
daemon child command: correct
status: {"version":"0.1.0","mode":"client","pid":46886,"connected":false,
  "registered":false,"interface":"pangolin0", ...}
clean stop verified
```

Agent log excerpt:

```text
[os-pangolin] client: connecting to https://pangolin-test.example.invalid
  (interface pangolin0, MTU 1280, subnet-router=true/opnsense)
DEBUG: websocket: Requesting token from https://.../api/v1/auth/olm/get-token
ERROR: websocket: Failed to connect: ... DNS lookup failed ... Retrying in 3s...
INFO: Olm service stopped
[os-pangolin] client: stopped
```

No client secret or bearer token appears in the log:

```text
/var/log/pangolin/client.log:0
/var/db/pangolin/client-status.json:0
```

### Firewall rules with simulated connected state

```text
# pfctl -a os-pangolin/fw -sr
pass in quick inet from 10.10.0.0/16 to 192.168.50.0/24 flags S/SA keep state
pass out quick on pangolin0 inet from 10.10.0.0/16 to 192.168.50.0/24 flags S/SA keep state

# pfctl -a os-pangolin/nat -sn
nat on pangolin0 inet from 10.10.0.0/16 to 192.168.50.0/24 -> 100.64.0.2
```

The status file also contained `0.0.0.0/0`, `128.0.0.0/1` and
`10.20.0.0/16`; none reached pf. `remoteNetworks=192.168.50.0/24` filtered the
learned set down to that prefix.

After `configctl filter reload` the anchors were unchanged.

### Unbound

```text
# Managed by os-pangolin. Do not edit; changes are overwritten.
# Alias records are learned from the running Pangolin machine client.
local-data: "db.internal.example. 60 IN A 100.64.0.2"
local-zone: "apps.internal.example." redirect
local-data: "apps.internal.example. 60 IN A 100.64.0.3"
```

```text
db.internal.example.    60  IN A 100.64.0.2      (after Unbound restart too)
apps.internal.example.  60  IN A 100.64.0.3
example.com.           300  IN A 104.20.23.154  (public answer, not rewritten)
new.internal.example.   60  IN A 100.64.0.9      (after DNS update)
new.internal.example.  NXDOMAIN                  (after disconnect cleanup)
```

Note: `unbound-checkconf` in this VM reports a pre-existing `pythonmod`
`unbound-dnsbl` path error. Unbound itself is running and answers the queries
above; the error is not produced by the plugin include.

## Findings and fixes in this phase

1. **Credential leaks in upstream debug logs.** OLM logged the full startup
   config, the token request body and received bearer token; Newt logged the
   token response and token; OLM peer setup logged the WireGuard private key.
   All are redacted in the vendored port (`a5e6b84`).
2. **Package scripts were omitted.** `pkg create` needs a metadata directory
   (`-m`) containing `+MANIFEST` and the `+*_INSTALL` scripts. The first package
   installed files but never ran `+POST_INSTALL`; configd actions and runtime
   directories were missing. Fixed in `build-pkg.sh`.
3. **`configctl configd sync` is not an action.** Post-install now reloads
   configd through `/usr/local/etc/rc.d/configd restart`, matching OPNsense's
   own plugin packaging, and calls `rc.configure_plugins`.
4. **rc.d launched the daemon recursively.** `command_args` used `${procname}`
   (`/usr/sbin/daemon`) as daemon's child, so the agent never started:
   `daemon: run: No such file or directory`. `procname` must stay the daemon
   path for rc.subr, while `command_args` must use the agent path. Fixed in
   both rc.d scripts, with `scripts/check-rc.py` as a regression check.
5. **Uninstall left empty runtime directories.** `+POST_DEINSTALL` now removes
   `/var/db/pangolin`, `/var/etc/pangolin` and `/var/log/pangolin` when empty.

## Required inputs for the remaining E2E tests

Do **not** paste secrets into chat or commit them. Place a root-readable,
mode 0600 config outside this repository, for example:

```text
C:\Users\Olof\pangolin-e2e\client.json
```

on the Windows host, or `/root/pangolin-e2e-client.json` on a test FreeBSD
host. Use the template in `tests/e2e/README.md`.

Required values:

1. Pangolin endpoint URL (HTTPS) and server version if known.
2. Machine-client ID and secret (and organization ID if required).
3. A private resource reachable through that client: IP or hostname, TCP
   ports, UDP ports.
4. The exact source VLAN CIDR and a separate test device on it.
5. A second, unselected VLAN/device for the negative test.
6. Confirmation that the instance and network are authorized for this test.
7. Whether mTLS client certificates are required, and their paths.
8. Whether relay-only and hole-punch paths can be tested deliberately.

With those inputs, run:

```sh
chmod 600 /root/pangolin-e2e-client.json
PANGOLIN_E2E_CONFIG=/root/pangolin-e2e-client.json tests/e2e/client-smoke.sh
```

The script refuses the production state directory, checks 0600 permissions,
runs the real agent, waits for `connected: true`, prints the tunnel IP and
routes, verifies that the configured secret is absent from the log, and checks
that a clean stop reports disconnected.
