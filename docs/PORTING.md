# FreeBSD port notes

## Upstream baseline

| Module | Tag | Commit |
|--------|-----|--------|
| `github.com/fosrl/newt` | v1.20.0 | `80c9ce7050bad844bebe35ff9b0edc0c88ade9ac` |
| `github.com/fosrl/olm` | v1.10.2 | `eee8873576cbc4e015c7b481f3b7948dcbb47596` |

The official `fosrl/cli` uses both libraries. Its `up site` command embeds
Newt; its `up client` command embeds OLM and exposes `--subnet-router`. This
project follows that split rather than reimplementing either protocol.

## What already worked on FreeBSD

- wireguard-go's `tun_freebsd.go` (kernel TUN support).
- `wireguard-go/ipc` UAPI on BSD.
- Newt's user-space netstack path (site connector).
- OLM's WebSocket, WireGuard device, peer management and DNS proxy after the
  build fixes below.
- FreeBSD `tun(4)` creation/name assignment through wireguard-go.

## Gaps found in the upstream source

### 1. Newt `network` has no FreeBSD branches

`ConfigureInterface`, `AddSecondaryAddress`, `RemoveSecondaryAddress`,
`AddRoutes`, `RemoveRoutes`, `AddRouteForServerIP*`, bypass routes and
`ManagesHostRoutes` only handled Linux/Darwin/Windows. On FreeBSD the switch
returned success without configuring anything, so a native TUN came up with no
address and no routes.

Fix (new files):

- `network/interface_freebsd.go`
  - `SIOCAIFADDR`/`SIOCDIFADDR` for addresses;
  - `SIOCGIFFLAGS`/`SIOCSIFFLAGS` for up;
  - `SIOCSIFMTU` for MTU;
  - point-to-point destination set to the local address, matching tun(4).
- `network/route_freebsd.go`
  - validated `/sbin/route` argv for add/delete;
  - refuses to replace a route that already points at a different
    interface/gateway;
  - `-ifa <source>` for secondary-address/source pinning;
  - bypass host routes track only routes the package added;
  - no shell, no default-route manipulation.
- `network/freebsd_stub.go`
  - non-FreeBSD stubs so the package still builds on the development host.

Small dispatch changes in `network/interface.go` and `network/route.go` select
the FreeBSD cases.

### 2. OLM DNS platform references Linux-only helpers

`dns/platform/detect_unix.go` builds on FreeBSD but calls
`IsSystemdResolvedAvailable`, defined only in `systemd.go` (Linux). Likewise,
`dns/override/dns_override_unix.go` references
`NewSystemdResolvedDNSConfigurator`.

Fix: `dns/platform/systemd_freebsd.go` provides a fail-returning FreeBSD stub
for both. Detection falls through to resolvconf/file handling. The plugin
disables system DNS override anyway; Unbound owns port 53.

### 3. OLM device layer lacks FreeBSD

`olm/device` only had `tun_darwin.go`, `tun_linux.go`, `tun_windows.go`, so
`CreateTUNFromFD`, `UapiOpen` and `UapiListen` were undefined.

Fix: `olm/device/tun_freebsd.go` — dup the fd, set non-blocking, and call
`tun.CreateTUNFromFile`. The same logic as Darwin, but with the correct build
tag and FreeBSD-specific comments.

### 4. No alias snapshot for embedders

OPNsense needs the private aliases held by OLM's DNS proxy to publish them in
Unbound. OLM only exposed point lookups.

Fix:

- `dns.RecordSnapshot` and `DNSRecordStore.Snapshot()` (stable order);
- `DNSProxy.SnapshotRecords()`;
- `Olm.DNSRecords()` and `Olm.DNSProxyIP()`.

These are additive APIs and suitable for an upstream PR.

### 5. Newt credential cache mode

Newt's websocket `saveConfig` wrote credentials with mode 0644. Changed to
0600. The agent also points Newt at `/var/db/pangolin/site-newt.json`, so it
never falls back to `$HOME/.config/...` (which can be `/` under daemon(8)).

## Why vendor instead of a runtime `go get`

The port changes method dispatch and adds platform files. A `go mod` version
cannot carry those changes. Vendoring:

- makes offline/reproducible builds possible;
- keeps the patch reviewable and ready for upstream PRs;
- avoids a fork-only module path or a private proxy;
- documents the exact upstream commit.

`third_party/olm/go.mod` replaces `github.com/fosrl/newt` with `../newt` so
both modules are built from the same patched tree.

## Generating the upstream patches

From the repository root, with pristine upstream trees at `_upstream/newt`
and `_upstream/olm`:

```sh
git diff --no-index \
  --src-prefix=a/upstream/ --dst-prefix=b/vendor/ \
  --output=patches/newt/0001-freebsd-network-support.patch \
  _upstream/newt third_party/newt

git diff --no-index \
  --src-prefix=a/upstream/ --dst-prefix=b/vendor/ \
  --output=patches/olm/0001-freebsd-support.patch \
  _upstream/olm third_party/olm
```

Apply with `git apply -p2` from a directory containing `newt/` and `olm/`
trees. The patches include the new files and the small dispatch changes.

## Build constraints

- Go 1.26+ (current build used 1.27.4).
- `CGO_ENABLED=0`; the agent is statically linked.
- `GOOS=freebsd`, `GOARCH=amd64` (arm64 also builds).
- No Linux headers, libc, nftables, systemd or D-Bus at runtime.

## What should go upstream first

1. The FreeBSD network backend (newt) — highest value, self-contained.
2. The FreeBSD TUN adapter and DNS platform stub (olm).
3. The additive alias snapshot API (olm).
4. The 0600 credential cache mode (newt).

The OPNsense-specific pf/Unbound integration stays in this repository; it is
not an upstream client concern.

## Remaining port work

- A routing-socket backend to replace route(8) on plain FreeBSD.
- IPv6 address/route support in the newt backend.
- Upstream CI coverage for `GOOS=freebsd`.
- A native FreeBSD packaging recipe for the agent outside OPNsense.
