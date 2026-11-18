# Roadmap

Target: **0.1.0 beta, 31 December 2026**. This roadmap is a planning aid, not a
claim that every item is complete. Status is updated when evidence exists.

## Phase 1 — Analysis, architecture, build environment (17–22 Nov 2026)

- [x] Clone and inspect `fosrl/cli`, `fosrl/newt`, `fosrl/olm`,
      `opnsense/core`, `opnsense/plugins`.
- [x] Confirm current CLI modes: `pangolin up client` (OLM) and
      `pangolin up site` (embedded Newt).
- [x] Confirm `--subnet-router` is an OLM client feature and currently
      Linux/nftables only.
- [x] Confirm Newt's site connector runs in user-space netstack and needs no
      TUN for the default path.
- [x] Verify OPNsense 26.7 hosts FreeBSD 15.1, pf, Unbound and PHP 8.x.
- [x] Create the QEMU/OPNsense integration harness.
- [x] Pin upstream versions and design a vendored-patch workflow.

## Phase 2 — Native FreeBSD port and first connection test (23–29 Nov)

- [x] FreeBSD interface configuration through ioctl(2).
- [x] FreeBSD route add/delete/bypass backend.
- [x] FreeBSD TUN adapter for OLM.
- [x] FreeBSD DNS platform build fix.
- [x] Cross-compile `pangolin-agent` for `freebsd/amd64` and `arm64`.
- [x] Run the binary on OPNsense/FreeBSD and start both modes.
- [x] Build, install and uninstall a real FreeBSD `.pkg` in an isolated
      OPNsense 26.7 VM; verify packaged agent startup, configd actions and
      package-script cleanup.
- [x] Verify the packaged service fails closed and keeps the secret out of
      logs when the endpoint is unreachable.
- [ ] Successful authenticated connection to a live Pangolin endpoint.
- [ ] WireGuard handshake, ping and reconnect against a live endpoint.
- [ ] Hole punching in a NAT lab.

Blocker: no live Pangolin instance/credentials in the development environment.
This is the highest-priority external dependency for beta. See
`docs/PHASE2-VALIDATION.md` for the executed matrix and the exact inputs needed.

## Phase 3 — Native subnet router (30 Nov–6 Dec)

- [x] Select local networks and validate them in Go and PHP.
- [x] Reject default routes, broad prefixes, non-unicast networks and overlaps.
- [x] Learn remote routes from the live tunnel.
- [x] Generate source-scoped pass rules bound to the source interfaces.
- [x] Generate source NAT to the tunnel address.
- [x] Register OPNsense-owned anchors.
- [x] Flush anchors and files on disconnect, empty route sets and errors.
- [x] Reject remote routes overlapping any local interface network.
- [x] Prove rule generation and loading with a simulated connected state.
- [ ] Prove the LAN-to-remote data plane through a real tunnel.
- [ ] Multi-VLAN production test.

## Phase 4 — OPNsense UI, plugin, service control (7–13 Dec)

- [x] MVC model, forms, menu and ACL.
- [x] Overview, Clients, Sites, Subnet Routing, DNS/Unbound, Settings, Logs.
- [x] API status/settings/service/DNS controllers.
- [x] configd actions for both services.
- [x] rc.d services with bounded restart.
- [x] 0600 config generation.
- [x] Service start/stop/restart/status verified on OPNsense 26.7.
- [x] Restart running services only when the generated agent JSON changed.
- [x] Page/API smoke tests and validation rejection tests.
- [ ] Browser E2E test with a trusted certificate (blocked by the development
      VM's self-signed web GUI certificate policy; tested with curl instead).

## Phase 5 — Unbound integration (14–20 Dec)

- [x] Snapshot OLM aliases via a patched exported API.
- [x] Generate a plugin-owned `unbound.opnsense.d` include.
- [x] Detect and skip manual host/alias/domain-override conflicts.
- [x] Support wildcard aliases.
- [x] Restart Unbound only when generated content changes.
- [x] Remove stale aliases on disconnect.
- [x] Optional conditional forwarding through official domain overrides.
- [x] Conditional-forwarding add/remove, manual-entry preservation and
      model-unavailable cleanup verified with a local test resolver.
- [ ] Verify conditional forwarding against a live Pangolin private resolver.
- [x] Uninstall cleanup for plugin-owned domain-override UUIDs.
- [x] Verify resolution and NXDOMAIN cleanup on OPNsense 26.7.
- [x] Protect manual parent zones and wildcard entries from learned subdomains.
- [ ] Verify with a live Pangolin alias set and HTTPS resource.
- [ ] Add automated conflict tests with real Unbound model fixtures.

## Phase 6 — Integration, security, stabilization (21–27 Dec)

- [x] Go unit tests for config, state, pf rules, HA parser, DNS snapshot.
- [x] Stale/dead agent heartbeat tests and fail-closed anchor/DNS removal.
- [x] PHP lint and XML parse checks.
- [x] QEMU integration fixture for pf and Unbound.
- [ ] Full live-server integration matrix.
- [ ] Two-node CARP failover test.
- [ ] Long-running reconnect/crash test.
- [ ] Packet capture proof of SNAT and return traffic.
- [ ] Third-party security review.

## Phase 7 — Documentation, packaging, v0.1.0 (28–31 Dec)

- [x] README, architecture, security, contributing, changelog, AGENTS.
- [x] Compatibility, deployment, HA, testing and porting docs.
- [x] Build/lint/package scripts.
- [x] Reproducible FreeBSD cross-build.
- [x] Real `.pkg` creation, install and uninstall in an isolated OPNsense 26.7
      VM (`pkg create` fixture).
- [ ] OPNsense ports package build (`make package`) and repository publication.
- [ ] Signed release artifacts.
- [ ] Final release notes and known-issues update.

## Post-beta

- IPv6 subnet routing with explicit separation from IPv4.
- CARP-aware independent identities per node.
- Full live HA test and automated failover.
- Upstream the FreeBSD patches to `fosrl/newt` and `fosrl/olm`.
- Optional routing-socket backend instead of route(8).
- Metrics/telemetry export consistent with Pangolin.
