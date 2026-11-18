# Vendored upstream sources

This directory contains the official Pangolin client runtimes, pinned to
released versions and carrying the minimal FreeBSD port needed by
`os-pangolin`.

| Directory | Upstream | Version | Commit | License |
|-----------|----------|---------|--------|---------|
| `newt/` | https://github.com/fosrl/newt | v1.20.0 | `80c9ce7050bad844bebe35ff9b0edc0c88ade9ac` | AGPL-3.0 (dual-licensed by Fossorial) |
| `olm/`  | https://github.com/fosrl/olm  | v1.10.2 | `eee8873576cbc4e015c7b481f3b7948dcbb47596` | AGPL-3.0 |

`olm` is built against the vendored `newt` via a `replace` directive in
`olm/go.mod`. This matches the dependency used by the official Pangolin CLI
(`github.com/fosrl/cli` uses `newt` and `olm` as libraries for `pangolin up
site` and `pangolin up client`).

## Why these are vendored

The official runtime is mostly portable, but the FreeBSD backend is
incomplete:

- `newt/network` has no FreeBSD branch for interface configuration, route
  installation or bypass routes. On FreeBSD the switch silently returned
  success, so a native TUN interface came up without an address or routes.
- `olm/dns/platform` references Linux-only systemd-resolved helpers while its
  build tag includes FreeBSD.
- `olm/device` only has `tun_darwin.go`, `tun_linux.go` and
  `tun_windows.go`, so `CreateTUNFromFD`, `UapiOpen` and `UapiListen` are
  undefined on FreeBSD.
- `olm` has no API to publish the resource aliases held by its DNS proxy; the
  OPNsense Unbound synchroniser needs that snapshot.

The changes are deliberately isolated in new files plus small dispatch
changes. They are suitable for upstream pull requests:

| File | Purpose |
|------|---------|
| `newt/network/interface_freebsd.go` | IPv4 address/MTU/flags via ioctl(2), no ifconfig shell parsing |
| `newt/network/route_freebsd.go` | Route add/delete/bypass via validated route(8) argv, refuses to replace foreign routes |
| `newt/network/freebsd_stub.go` | Build stubs for non-FreeBSD development hosts |
| `newt/network/interface.go`, `route.go` | FreeBSD dispatch cases |
| `newt/websocket/config.go` | Credential cache mode 0600 |
| `olm/dns/platform/systemd_freebsd.go` | FreeBSD stubs for the Linux-only resolver helpers |
| `olm/device/tun_freebsd.go` | Adopt an existing tun(4) fd with wireguard-go |
| `olm/dns/dns_records.go`, `dns_proxy.go` | Stable alias snapshot |
| `olm/olm/olm.go` | `DNSRecords()`/`DNSProxyIP()` and redacted startup logging |
| `olm/peers/peer.go` | Redact WireGuard private key from peer logging |
| `olm/websocket/client.go` | Redact client secret and bearer token from logs |
| `newt/websocket/client.go` | Redact token response and bearer token from logs |
| `olm/go.mod` | Pin the vendored newt |

`patches/` contains the same changes as unified diffs against the upstream
tags, ready to be submitted as pull requests.

## Updating

1. Fetch the new upstream tag and compare `patches/` with the new code.
2. Re-apply the FreeBSD files/patches, run `go test ./...` in a FreeBSD or
   cross-compile check (`GOOS=freebsd GOARCH=amd64 go build ./...`).
3. Update the table above and `docs/COMPATIBILITY.md`.
4. Run `scripts/build-agent.sh` and the integration tests.

Do not edit vendored sources for style-only reasons. Keep the diff small so it
can be upstreamed and reviewed independently.
