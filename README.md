# os-pangolin

Native [Pangolin](https://pangolin.net) integration for OPNsense, written for
FreeBSD and packaged as an OPNsense plugin.

`os-pangolin` runs the official Pangolin client runtimes (`fosrl/newt` and
`fosrl/olm`) directly on the firewall. It does **not** use Linux VMs, Linux
containers, Docker, a Linux compatibility layer or an external Linux gateway.

**Status: 0.1.0 beta preparation.** The FreeBSD build, OPNsense MVC/API layer,
pf integration, Unbound integration, service supervision and the native
subnet-router rule path are implemented and were exercised on OPNsense 26.7.
An end-to-end data-plane test against a live Pangolin instance still requires
credentials and a lab peer; see [docs/TESTING.md](docs/TESTING.md).

## Features

| Area | State |
|------|-------|
| Machine client (`fosrl/olm`) on FreeBSD | Implemented, starts and retries; live tunnel test pending |
| Site connector (`fosrl/newt`, `pangolin up site` equivalent) | Implemented; live site test pending |
| Native FreeBSD TUN backend | Implemented with ioctl(2) + route(8) |
| Native subnet router (`--subnet-router` equivalent) | Implemented for OPNsense pf anchors |
| OPNsense firewall integration | Source-scoped NAT/filter in `os-pangolin/*` anchors |
| Automatic route lifecycle | Agent-owned, cleaned up on disconnect |
| Unbound alias sync | Implemented through `unbound.opnsense.d` include |
| Conditional forwarding | Implemented through OPNsense Unbound domain overrides |
| Manual Unbound conflict protection | Implemented |
| CARP active/passive gate | Implemented (basic fail-closed design) |
| Logs / status / service control | Implemented in the OPNsense UI and API |
| HA configuration sync | Declared for XML-RPC; not validated on two nodes |
| IPv6 subnet routing | Not in 0.1.0 (IPv4 only, by design) |

## Architecture at a glance

```
                       Pangolin server / Gerbil
                                 │
                        WireGuard / WebSocket
                                 │
                 ┌───────────────┴────────────────┐
                 │      os-pangolin-agent         │
                 │  site mode: newt (netstack)    │
                 │  client mode: olm + tun(4)     │
                 └───────┬───────────────┬────────┘
                         │               │
             /var/db/pangolin/*.json     │ OPNsense MVC
                         │               │
        ┌────────────────┴───────┐   ┌───┴────────────────────┐
        │ pangolin.inc (configd) │   │ Views / API / forms    │
        │ pf anchors, Unbound    │   │ VPN → Pangolin         │
        └───────────┬────────────┘   └────────────────────────┘
                    │
          ┌─────────┴──────────┐
          │  pf(4) + Unbound   │
          └────────────────────┘
```

More detail: [ARCHITECTURE.md](ARCHITECTURE.md).

## Repository layout

```
agent/                  Go agent (site + client + FreeBSD backend + status)
  cmd/pangolin-agent/
  internal/{agent,config,ha,hook,logging,pf,state,version}/
plugin/                 OPNsense plugin (MVC, Volt, configd, rc.d, pf, Unbound)
third_party/            Pinned fosrl/newt v1.20.0 and fosrl/olm v1.10.2 + port
patches/                Upstream-ready unified diffs for the FreeBSD port
scripts/                Build, lint, packaging and QEMU helpers
tests/integration/qemu/ Repeatable OPNsense VM integration fixtures
docs/                   Compatibility, deployment, HA, testing, porting
```

## Build

Requirements: Go 1.26+ (1.27 was used), PHP 8.x for linting, Python 3 for XML
checks. The agent cross-compiles on Linux, macOS or Windows:

```sh
make agent            # builds plugin/src/sbin/pangolin-agent (freebsd/amd64)
make test             # go test ./...
make lint             # go vet, freebsd/amd64+arm64 builds, PHP/XML lint
make dist             # release tarball in dist/
```

On Windows PowerShell:

```powershell
.\scripts\build-agent.ps1
.\scripts\build-plugin.ps1
```

The compiled agent is intentionally not committed to git. Release packages
must build it first; `plugin/Makefile` is the ports entry point.

## Install on OPNsense

The supported installation path is the OPNsense plugin package built from
`plugin/Makefile` (ports tree). For a development firewall, the release tarball
can be copied and extracted into `/usr/local`:

```sh
fetch -o /tmp/os-pangolin.tgz http://<your-server>/os-pangolin-0.1.0-amd64.tgz
tar -xzf /tmp/os-pangolin.tgz -C /tmp
cp -R /tmp/os-pangolin-0.1.0-amd64/src/* /usr/local/
chmod 755 /usr/local/sbin/pangolin-agent /usr/local/etc/rc.d/pangolin_*
chmod 700 /usr/local/etc/pangolin/hook.sh
service configd restart
rm -f /var/lib/php/tmp/opnsense_menu_cache.xml /var/lib/php/tmp/opnsense_acl_cache.json
configctl webgui restart
```

Then open **VPN → Pangolin**:

1. **Settings** → enable the plugin and (optionally) the CARP gate.
2. **Clients** → enter the Pangolin endpoint, machine client ID/secret.
3. **Sites** → enter the site endpoint, site ID/secret (independent of the
   client).
4. **Subnet Routing** → select the machine client, enable subnet routing and
   list the allowed local IPv4 CIDRs. Optionally set an allowed remote-network
   allow-list. Select the OPNsense backend.
5. **DNS / Unbound** → enable alias synchronisation if private aliases should
   resolve on the LAN.
6. Click **Apply**. Check **Overview** and **Logs**.

Never select `0.0.0.0/0`; the model, the PHP validator and the Go agent all
reject it. A subnet router is not a full-tunnel exit node.

## Security properties

- The agent runs as root (needed for `tun(4)`); the OPNsense web UI and API run
  as the normal OPNsense user. Privileged work goes through configd.
- Secrets are stored in `config.xml` (root-readable) and written to agent JSON
  files with mode `0600`. They are never placed on a command line, logged or
  written to status files.
- pf changes are restricted to the plugin-owned `os-pangolin/*` anchors. The
  global ruleset is never flushed or edited.
- The subnet router never installs a default route, never allows sources other
  than the selected local networks, and never creates inter-VLAN pass rules.
- Unbound changes are restricted to the plugin-owned include file and
  plugin-owned conditional-forward UUIDs. Manual host/domain overrides win.
- The CARP gate fails closed when CARP state cannot be read.

See [SECURITY.md](SECURITY.md) and [docs/HA.md](docs/HA.md).

## Verified environment

- OPNsense 26.7 (FreeBSD 15.1-RELEASE-p1), amd64 VM.
- Upstream `fosrl/newt` v1.20.0 (`80c9ce7`) and `fosrl/olm` v1.10.2 (`eee8873`).
- Go 1.27.4 cross-compilation, `CGO_ENABLED=0`, `GOOS=freebsd`.
- PHP 8.4 lint for every plugin PHP file; XML parse for every plugin XML file.
- QEMU integration checks: package discovery, MVC pages/API, service start/stop,
  config generation, pf anchor rules, Unbound include + resolution, cleanup.

Full details and test evidence: [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md)
and [docs/TESTING.md](docs/TESTING.md).

## Known limitations

- A live Pangolin server/site/machine credential was not available in the
  development environment. Authentication, WireGuard handshake, hole punching
  and forwarded data plane must be validated in a lab before production use.
- Site mode exposes local resources through Newt's proxy model. The server does
  not send resource aliases to Newt, so the Unbound alias synchroniser is
  client-mode only. This is a Pangolin protocol limitation, not a DNS rewrite.
- IPv6 subnet routing is intentionally disabled. The plugin does not rewrite
  IPv6 routes or firewall rules.
- HA was designed and the CARP gate was implemented, but a two-node failover
  test was not performed. HA configuration sync transmits the plugin section,
  including secrets, over OPNsense's XML-RPC sync.
- The standalone `pf` backend exists in the agent for plain FreeBSD but is not
  exposed in the OPNsense UI; OPNsense uses the official firewall plugin API.

## License

AGPL-3.0. The vendored `fosrl/newt` and `fosrl/olm` sources are AGPL-3.0
(dual-licensed by Fossorial); see `third_party/*/LICENSE`.
