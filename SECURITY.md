# Security policy and threat model

## Supported versions

`0.1.x` beta. Security fixes are applied to the latest 0.1 release. The plugin
is not yet recommended for unattended production firewalls until a live-server
integration test and a two-node HA test have been completed.

## Reporting a vulnerability

Do not open a public issue for a vulnerability. Send a private report to the
maintainers listed in `plugin/Makefile` with:

- affected version/commit;
- OPNsense and FreeBSD version;
- reproduction steps or a minimal configuration;
- impact (confidentiality, integrity, availability);
- whether firewall, Unbound or credential data is exposed.

We aim to acknowledge within 72 hours and to publish a coordinated fix. Do not
include live Pangolin secrets in a report; redact them.

## Assets

- Pangolin machine client ID/secret and site ID/secret.
- WireGuard private keys generated in memory by OLM/Newt.
- The firewall's pf ruleset and NAT state.
- Unbound configuration and DNS answers.
- Routing table and the LAN-to-tunnel forwarding path.

## Trust boundaries

1. **Web UI / API user** → OPNsense MVC. The user is authenticated by OPNsense
   and authorized by the plugin ACL. The web process does not write agent
   configuration directly.
2. **OPNsense controller** → configd (root). Configd writes 0600 agent files,
   starts services and loads plugin-owned pf anchors.
3. **Agent (root)** → kernel tun(4)/route(8)/pf(8) and Pangolin. The agent
   treats all server data as untrusted input and validates routes, addresses
   and DNS records.
4. **Pangolin server** → agent. A compromised or malicious server can send
   routes, target ports and aliases. The agent/plugin restrict what reaches pf
   and Unbound; they do not blindly install default routes or arbitrary DNS.
5. **LAN device** → OPNsense forwarding. Only explicitly selected local
   networks are allowed and NATed; other VLANs remain blocked by the normal
   OPNsense policy.

## Key properties

- **No default route.** The subnet router never installs `0.0.0.0/0`,
  `0.0.0.0/1` or `128.0.0.0/1`. Full tunnel/exit-node behavior is the machine
  client's own feature and is not enabled by the plugin's subnet router.
- **No all-VLAN exposure.** Local networks are an explicit allow-list. Prefixes
  shorter than /8, `0.0.0.0/0`, loopback, link-local, multicast and reserved
  networks are rejected in three places: XML model mask, PHP validator and Go
  config validator. An optional remote allow-list further restricts which
  learned destination routes are installed; remote/local overlaps are rejected.
- **Interface-scoped forwarding.** The OPNsense inbound pass rule is bound to
  the interfaces that carry the selected local networks, and a learned remote
  route that overlaps **any** local interface network (not only the selected
  sources) is discarded. The standalone pf backend resolves local interfaces
  and applies the same overlap rejection. An empty or fully filtered route set
  flushes the anchors instead of producing `to any` rules.
- **No global pf rewrite.** The plugin registers `os-pangolin/nat` and
  `os-pangolin/fw` through OPNsense's firewall plugin API and loads only those
  anchors. It never runs `pfctl -f` on the global ruleset.
- **No foreign Unbound changes.** Static aliases go into a dedicated
  `unbound.opnsense.d` include. Conditional forwarding uses only UUIDs recorded
  in `/var/db/pangolin/unbound-dots.json`. Manual host/alias entries, domain
  overrides and wildcard entries win; a manual parent zone protects all of its
  subdomains. If the manual-entry model cannot be inspected, the plugin removes
  the alias include (source and chroot) and restarts Unbound; it never leaves
  stale aliases published.
- **Fail closed.** If CARP state cannot be read, the tunnel waits/stops. The
  plugin requires the status file's agent PID to be alive and the heartbeat to
  be recent; a live `daemon(8)` supervisor with a dead or stale child is treated
  as disconnected, and anchors/DNS aliases are removed. If the tunnel has no
  valid address/interface, no rules are generated.
- **Secrets at rest.** `config.xml` is root-readable. Agent JSON is mode 0600.
  Newt's credential cache is mode 0600 and lives under `/var/db/pangolin`, not
  in a user home. Status and DNS snapshot files contain no secrets.
- **Uninstall scope.** `pkg delete` removes generated agent JSON, runtime state,
  pf anchors and the Unbound include. The `OPNsense/pangolin` model section is
  intentionally left in `config.xml` and its backups, as is normal for OPNsense
  plugins. If the firewall is decommissioned or the client is retired, remove
  that section manually and rotate the Pangolin client secret.
- **No secrets in argv or logs.** The agent is started with `--config path`
  only. It does not log secrets. The vendored runtimes are patched to redact
  the token request body, bearer tokens, WireGuard private keys and the full
  OLM startup config from debug logs. The UI uses password fields.
- **Least privilege for the web tier.** The web/API layer calls configd; it does
  not receive a root shell or write pf/Unbound files itself.
- **Bounded blast radius.** pf anchors are named, can be inspected with
  `pfctl -a os-pangolin/...`, and are flushed on disable/uninstall.

## Known risks and limitations

- The agent runs as root because tun(4) and route changes require it. The
  attack surface is the Go runtime plus the vendored Newt/OLM code. The build is
  `CGO_ENABLED=0` and statically linked; keep the binary root:wheel 0755.
- OPNsense's XML-RPC HA sync sends the plugin's `config.xml` section, including
  secrets, to the peer over the configured sync channel. Secure the sync
  channel and restrict access to both firewalls.
- The DNS alias path trusts the server's alias records. A malicious server
  could publish an alias for a domain the operator cares about unless a manual
  Unbound entry exists. For high-value names, create manual Unbound entries;
  they take precedence.
- The plugin cannot verify that a Pangolin server is honest about a resource's
  remote IP. It can only ensure the local firewall rules are scoped and the
  default route is untouched.
- A compromised root account on the firewall can do anything, including editing
  the plugin files. This is outside the plugin's threat model.
- The standalone `pf` backend in the agent is for plain FreeBSD and is not
  exposed in OPNsense. It refuses to act unless `/etc/pf.conf` references its
  anchors; it never edits that file.
- The QEMU test environment used a self-signed OPNsense certificate. Browser
  automation correctly refused it, so UI tests used authenticated HTTP(S) API
  calls with certificate verification disabled only for the test harness. That
  testing caveat does not weaken the production code.

## Operational hardening checklist

- Use HTTPS for every Pangolin endpoint.
- Keep the plugin's advanced `allowInsecureHttp` off.
- Run OPNsense on a supported release with current security patches.
- Restrict the `VPN: Pangolin` ACL to firewall administrators.
- Keep `/usr/local/etc/pangolin` 0700 and its JSON files 0600.
- Monitor `/var/log/pangolin/*.log` and the OPNsense system log for
  `pangolin:` messages.
- Review `pfctl -a os-pangolin/fw -sr` and `pfctl -a os-pangolin/nat -sn` after
  changing subnet-routing settings.
- Do not add `0.0.0.0/0` to local networks; it is rejected, but keep it out of
  automation that may patch the model directly.
- Test failover with CARP before relying on HA.
