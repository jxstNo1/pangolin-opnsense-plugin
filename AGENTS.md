# AGENTS.md — permanent rules for coding agents

These rules apply to every change in this repository. Read them before editing.

## Mission

Build and maintain a native FreeBSD/OPNsense integration for Pangolin. The
runtime must work directly on OPNsense. Linux VMs, containers, Docker,
compatibility layers, external Linux gateways and extra Linux subnet routers
are prohibited.

## Non-negotiable safety invariants

1. Never install or modify a default route (`0.0.0.0/0`, `0.0.0.0/1`,
   `128.0.0.0/1`) as part of subnet routing.
2. Never allow all VLANs. Local networks are an explicit allow-list. Reject
   `0.0.0.0/0`, prefixes shorter than /8, loopback, link-local, multicast,
   reserved and overlapping networks in the model, PHP validator and Go
   validator.
3. Never flush or rewrite the global pf ruleset. Use only the plugin-owned
   `os-pangolin/nat` and `os-pangolin/fw` anchors, registered through the
   OPNsense firewall plugin API.
4. Never create inter-VLAN pass rules. Only selected sources to learned remote
   routes may pass.
5. Never edit, reorder or delete manually managed Unbound entries. The plugin
   owns only `unbound.opnsense.d/os-pangolin.conf` and the domain-override UUIDs
   recorded in `/var/db/pangolin/unbound-dots.json`.
6. Never rewrite public domains to private tunnel addresses. Public HTTPS must
   keep its normal DNS, SNI, Host header and TLS behavior.
7. Never put secrets on a command line, in logs, in status files or in git.
   Agent config files are 0600; `config.xml` is root-readable.
8. Fail closed. Unreadable CARP state, a dead agent PID, an invalid tunnel
   address or a missing interface must disable the tunnel rules, not open them.
9. Do not call `pluginctl`/`configctl` from inside a configd hook. It deadlocks
   configd. Call the required helper directly or use a background command.
10. Do not use `mwexec` on OPNsense 26.7; it is not defined. Use
    `mwexecf`, `mwexecfm` or `mwexecfb`.

## Upstream policy

- `third_party/newt` is pinned to `fosrl/newt` v1.20.0.
- `third_party/olm` is pinned to `fosrl/olm` v1.10.2.
- Keep the port minimal. Put FreeBSD code in new files and add only small
  dispatch changes. Do not reformat upstream files.
- Every vendored change must appear in `patches/` and be described in
  `docs/PORTING.md` and `third_party/README.md`.
- Preserve upstream license headers and AGPL notices.

## OPNsense conventions

- Model mount: `//OPNsense/pangolin`.
- Menu: `VPN → Pangolin`.
- Use MVC models, XML forms, Volt templates, configd actions and rc.d.
- Field ids are `pangolin.<section>.<field>`.
- Configuration writes happen through configd as root, not in the web
  controller.
- Service control uses configd action names; rc.d scripts run under daemon(8)
  with a bounded restart count.
- Use `ApiMutableModelControllerBase` for settings and `ApiControllerBase` for
  status/service APIs.
- Validate cross-field security rules in `Pangolin::validateSecurity()` and in
  the Go config validator; do not rely on the XML mask alone.

## Go conventions

- `gofmt` and `go vet` clean.
- Validate all data that reaches exec, pf, Unbound or filesystem paths.
- Use explicit argv (`exec.Command`) and never `sh -c` with interpolation.
- Do not add cgo; the agent must build with `CGO_ENABLED=0`.
- Keep the agent embeddable/testable; no global state in new packages.

## Testing and evidence

- Run `cd agent && go test ./...` and `go vet ./...`.
- Cross-compile for `freebsd/amd64` and `freebsd/arm64`.
- Run PHP lint and XML parse checks.
- For pf/Unbound changes, run the QEMU fixture or document why it was not
  possible. Never invent test results.
- Update `CHANGELOG.md` and `ROADMAP.md` when behavior or status changes.
- State clearly what was verified on real FreeBSD/OPNsense hardware and what
  remains a lab-only or unverified path.

## Documentation

Keep these files accurate:

- `README.md`
- `ARCHITECTURE.md`
- `ROADMAP.md`
- `SECURITY.md`
- `CONTRIBUTING.md`
- `CHANGELOG.md`
- `docs/COMPATIBILITY.md`
- `docs/DEPLOYMENT.md`
- `docs/HA.md`
- `docs/TESTING.md`
- `docs/PORTING.md`

## Git

- Small, focused commits with imperative subjects.
- Do not commit built binaries, test-VM disks, ISO files, credentials or logs.
- Do not rewrite published history.
