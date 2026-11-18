# Changelog

All notable changes are documented here. The project follows a pragmatic
`MAJOR.MINOR.PATCH` scheme during beta.

## [0.1.0] - 2026-11-18 (beta preparation)

### Added

- Native FreeBSD Go agent (`pangolin-agent`) for OPNsense.
- Site-connector mode embedding `fosrl/newt` v1.20.0.
- Machine-client mode embedding `fosrl/olm` v1.10.2.
- FreeBSD TUN adapter for OLM and ioctl(2)-based address/MTU/flag handling.
- FreeBSD route add/delete/bypass backend with foreign-route protection.
- Native subnet router for OPNsense:
  - explicit local-network allow-list;
  - optional allowed remote-network allow-list;
  - IPv4-only route learning from the live tunnel;
  - source NAT to the tunnel address;
  - source-scoped pass rules in `os-pangolin/fw`;
  - default-route, broad-prefix, overlap and non-unicast rejection;
  - automatic cleanup on disconnect/disable.
- OPNsense MVC plugin:
  - Overview, Clients, Sites, Subnet Routing, DNS/Unbound, Settings and Logs;
  - settings/status/service/DNS APIs;
  - validated XML models and cross-field security checks;
  - configd actions and rc.d services with bounded restart.
- Unbound integration:
  - plugin-owned `unbound.opnsense.d/os-pangolin.conf`;
  - conflict detection against manual host/alias/domain-override entries;
  - wildcard alias support;
  - conditional forwarding through official domain overrides;
  - stale alias removal on disconnect;
  - uninstall cleanup for plugin-owned conditional-forwarding entries.
- CARP active/passive gate with fail-closed behavior.
- Status, DNS snapshots and logs in `/var/db/pangolin` and `/var/log/pangolin`.
- Build, lint, packaging and QEMU integration scripts.
- Documentation: README, architecture, security, contributing, roadmap,
  compatibility, deployment, HA, testing and porting.
- AGENTS.md rules for future coding agents.

### Verified in this release cycle

- Go unit tests for config validation, state, pf rule generation, HA parsing and
  DNS snapshots.
- FreeBSD amd64/arm64 cross-compilation with `CGO_ENABLED=0`.
- PHP lint for every plugin PHP file; XML parse for every model/form/menu/ACL.
- OPNsense 26.7/FreeBSD 15.1 VM:
  - plugin discovery, menu and all seven pages;
  - settings GET/POST, validation rejection of `0.0.0.0/0` and plain HTTP;
  - config generation with 0600 mode;
  - service start/stop/restart/status;
  - packaged rc.d startup of the real agent with an unreachable endpoint
    (`connected:false`, no anchors/DNS opened);
  - pf anchor registration and source-scoped rules;
  - Unbound include generation, resolution, update and NXDOMAIN cleanup;
  - conditional-forwarding path code review (not exercised with a live model).
- Real FreeBSD `.pkg` built with `pkg create` inside a fresh OPNsense 26.7 VM,
  installed, exercised and uninstalled; package scripts, configd action
  registration and runtime-directory cleanup verified.
- QEMU fixtures for simulated connected state, rule generation and cleanup.

### Known issues / not yet verified

- No live Pangolin server was available: authentication, WireGuard handshake,
  hole punching, live alias delivery and the LAN-to-remote data plane still
  need a lab test.
- No two-node CARP failover test.
- Site mode does not receive resource aliases from the server, so automatic
  Unbound alias sync is client-mode only.
- IPv6 subnet routing is not implemented.
- Browser automation could not use the test VM's self-signed certificate; UI
  behavior was validated with authenticated HTTP(S) API calls instead.
- A real `.pkg` was built with `pkg create`; the OPNsense ports-tree
  `make package` path was not exercised.
- Uninstalling the package leaves the `OPNsense/pangolin` model section in
  `config.xml` and its backups; remove it manually when decommissioning and
  rotate the client secret.

### Fixed

- rc.d services passed `${procname}` (`/usr/sbin/daemon`) as daemon's child
  command, so the agent never started (`daemon: run: No such file or
  directory`). `procname` remains the daemon for rc.subr liveness checks;
  `command_args` now uses the agent path. Added `scripts/check-rc.py`.
- The package-build fixture called `pkg create` without a metadata directory,
  which silently omitted `+POST_INSTALL`/`+POST_DEINSTALL`; configd actions and
  runtime directories were not created.
- `+POST_INSTALL` called the nonexistent `configctl configd sync` action;
  it now reloads configd the same way OPNsense plugin packaging does.
- Uninstall left empty `/var/db/pangolin`, `/var/etc/pangolin` and
  `/var/log/pangolin` directories behind.

### Security notes

- Redacted credential material from all debug logging paths: client secret in
  the token request, bearer tokens in auth responses, the WireGuard private key
  in peer configuration, and the full OLM startup config. These were latent
  debug-level leaks in the upstream runtimes and are part of the vendored port.
- Fixed a configd deadlock: plugin hooks must not call `pluginctl`/`configctl`
  recursively; Unbound is stopped directly and started through `start.sh`.
- Fixed service detection to verify the PID command, not just PID existence.
- Fixed HTTP endpoint and default-route validation in both PHP and Go.
