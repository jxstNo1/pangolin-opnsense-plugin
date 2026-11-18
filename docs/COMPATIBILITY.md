# Compatibility

## Verified matrix

| Component | Version / commit | Evidence |
|-----------|------------------|----------|
| OPNsense | 26.7 (amd64 VM) | Plugin pages/API, configd, pf, Unbound tested |
| FreeBSD | 15.1-RELEASE-p1 | `uname -a` in the integration VM |
| `fosrl/newt` | v1.20.0, `80c9ce7050bad844bebe35ff9b0edc0c88ade9ac` | Embedded, FreeBSD build |
| `fosrl/olm` | v1.10.2, `eee8873576cbc4e015c7b481f3b7948dcbb47596` | Embedded, FreeBSD build/run |
| `fosrl/cli` reference | HEAD inspected 2026-11-06 (`652991b`) | Confirms `up site` uses Newt and `up client` uses OLM |
| Go toolchain | 1.27.4 (host) | Cross-compiled `freebsd/amd64`, `freebsd/arm64` |
| PHP | 8.4.27 for lint; OPNsense PHP 8.x at runtime | `php -l` for all plugin files |
| Python | 3.14 for XML checks | All plugin XML parsed |
| QEMU | 11.1.0 | OPNsense VM integration |

## Why these upstream versions

The official Pangolin CLI uses both libraries:

- `pangolin up site` embeds `github.com/fosrl/newt` and reuses its config
  loader, websocket, target/proxy runtime and user-space netstack.
- `pangolin up client` embeds `github.com/fosrl/olm` and adds the
  `--subnet-router` option.

`newt` v1.20.0 is the CLI's current dependency and already includes a FreeBSD
permissions check. `olm` v1.10.2 is the latest released OLM. The agent's
`go.mod` pins both and replaces them with the vendored, patched trees, so a
build is reproducible from this repository.

## FreeBSD facilities used

- `tun(4)` through wireguard-go's `tun_freebsd.go`.
- `ioctl(2)` with `SIOCAIFADDR`, `SIOCDIFADDR`, `SIOCSIFFLAGS`,
  `SIOCSIFMTU`, `SIOCGIFFLAGS` for interface configuration.
- `route(8)` for route add/delete and physical-path bypass routes, with explicit
  argument vectors and foreign-route protection.
- `pf(4)`/`pfctl(8)` only through OPNsense-owned anchors on OPNsense, and a
  standalone anchor on plain FreeBSD.
- `daemon(8)` for supervision, `rc.subr` for service control.
- `ifconfig(8)` only for CARP state parsing.
- `pgrep(1)`/`ps(1)` for PID liveness and command-name verification.
- Unbound include directory `/usr/local/etc/unbound.opnsense.d/` and the
  official `OPNsense.Unbound` domain-override model.

## OPNsense/PHP API assumptions

- `OPNsense\Base\BaseModel`, `ApiMutableModelControllerBase`,
  `ApiControllerBase`, `OPNsense\Core\Backend`, `configd`, `mwexecf`,
  `mwexecfb`, `service_log`, `isvalidpid`, `shell_safe`.
- `OPNsense\Pangolin\Pangolin` is autoloaded from
  `models/OPNsense/Pangolin/Pangolin.php`.
- `plugins.inc.d/pangolin.inc` is included from `/usr/local/etc/inc/plugins.inc.d`.
- The firewall plugin API `registerAnchor($name, $type)` exists.
- Unbound `start.sh` copies `*.conf` from `/usr/local/etc/unbound.opnsense.d`
  into `/var/unbound/etc` and the generated config includes
  `include: /var/unbound/etc/*.conf`.
- `daemon(8)` supports `-R`, `-C` and `-P` (FreeBSD 15.1).

## Known incompatibilities / gaps

- `olm`'s subnet router uses `google/nftables` and is Linux-only; the agent
  disables it and implements the equivalent with OPNsense pf/anchors.
- `olm`'s DNS override platform has Linux/systemd assumptions; the FreeBSD
  stubs fall back to file/resolvconf code. The plugin intentionally disables
  system DNS override so Unbound remains authoritative.
- Newt does not receive resource aliases. Automatic alias DNS therefore only
  applies to the machine client. Site mode can still expose resources through
  Pangolin's proxy model.
- The official CLI's `--subnet-router` help still says "Linux only". The
  native OPNsense implementation is separate from that flag and does not rely
  on nftables.
- The plugin does not yet ship a prebuilt OPNsense package repository; build
  from `plugin/Makefile` or use the release tarball.

## Update procedure

1. Update the upstream tag and commit in `third_party/README.md`.
2. Re-vendor `newt`/`olm` from the official tag.
3. Re-apply the patches in `patches/`.
4. Run the checks in `CONTRIBUTING.md` and the QEMU integration fixtures.
5. Update this file, `CHANGELOG.md` and `ROADMAP.md`.
