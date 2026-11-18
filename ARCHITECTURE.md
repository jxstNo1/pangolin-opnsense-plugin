# Architecture

## Goals and non-goals

`os-pangolin` makes OPNsense a first-class Pangolin participant without a
Linux sidecar:

- one agent binary, cross-compiled for FreeBSD, embedding the official client
  runtimes;
- a native FreeBSD network backend for tun(4), addresses and routes;
- OPNsense-native configuration, service control, firewall and DNS;
- fail-closed cleanup and no global pf or Unbound rewrites.

Non-goals for 0.1.0: IPv6 subnet routing, a full-tunnel exit node, a second
routing daemon, a proprietary protocol implementation, and a Linux runtime.

## Components

### `pangolin-agent` (Go)

One binary with two independent service modes.

**Site mode** (`site.json`)

- Embeds `github.com/fosrl/newt` v1.20.0, the same runtime used by
  `pangolin up site`.
- Default transport is wireguard-go's user-space netstack. No TUN device is
  created, which is ideal for a firewall that only exposes local resources.
- Optional `useNativeMainInterface` uses the FreeBSD TUN backend.
- Newt's downstream WireGuard client hub and SSH/auth daemon are disabled by
  default.
- A health file is written by Newt's ping loop; the agent turns its freshness
  into `connected` status.

**Client mode** (`client.json`)

- Embeds `github.com/fosrl/olm` v1.10.2, the same runtime used by
  `pangolin up client`.
- Creates a native `tun(4)` interface through wireguard-go's FreeBSD TUN
  driver, assigns IPv4/MTU through ioctl(2), and installs routes through a
  validated route(8) backend.
- OLM's system DNS override is deliberately disabled. Unbound remains the LAN
  resolver; private aliases are exported by the agent and consumed by the
  plugin.
- OLM's Linux nftables subnet router is deliberately disabled. The agent
  exports tunnel IP, interface and remote routes; the OPNsense plugin owns the
  NAT/filter rules and the agent can use a standalone pf backend on plain
  FreeBSD.

The agent writes two atomic JSON documents per mode:

```
/var/db/pangolin/client-status.json
/var/db/pangolin/site-status.json
/var/db/pangolin/dns-records.json      # client mode aliases
```

Status files contain no secrets. They include PID, timestamps, connection
state, tunnel address, interface, learned routes, DNS proxy address, DNS
records, HA state and the last error.

A debounced hook (`/usr/local/etc/pangolin/hook.sh`) calls
`configctl pangolin sync` after state changes. This is the only path from the
agent back into OPNsense.

### OPNsense plugin (PHP/Volt)

- MVC model at `//OPNsense/pangolin`.
- Controllers: Overview, Clients, Sites, Subnet Routing, DNS/Unbound, Settings,
  Logs plus API controllers.
- Volt forms validate endpoint URLs, interface names, durations, MTU, CIDRs and
  secrets.
- `Pangolin::validateSecurity()` adds cross-field checks that XML models cannot
  express:
  - HTTPS unless the explicit advanced override is set;
  - client/site interface names must differ;
  - subnet routing requires the client and at least one local network;
  - no default route, no prefix shorter than /8, no overlapping networks;
  - conditional forwarding requires at least one private suffix.
- `Pangolin::syncAgentConfigs()` writes the two JSON files with mode 0600.
  This runs through configd as root, never from the web process.

### configd and rc.d

Configd actions:

| Action | Purpose |
|--------|---------|
| `pangolin sync` | write agent config, reconcile DNS/firewall, start enabled services |
| `pangolin dns-sync` | reconcile only the Unbound include/overrides |
| `pangolin firewall-sync` | reconcile only the pf anchors |
| `pangolin_client start/stop/restart/status/test/logs` | machine client |
| `pangolin_site start/stop/restart/status/test/logs` | site connector |

Both services run under `daemon(8)` with a restart delay and a bounded restart
count. TUN/routing state is owned by the agent; a crash destroys the TUN
interface and the kernel removes its routes.

## Subnet-router data flow

```
LAN device 10.10.5.20
        │ default gateway = OPNsense
        ▼
OPNsense kernel route 10.20.0.0/16 dev pangolin0
        │
        ▼
pf: pass in on LAN from {10.10.0.0/16} to {remote routes}
    pass out on pangolin0 ...
    nat on pangolin0 from {10.10.0.0/16} to {remote routes} -> 100.64.0.2
        │
        ▼
WireGuard tunnel (OLM identity 100.64.0.2)
        │
        ▼
Pangolin / remote site resource
```

The plugin generates NAT and filter rules only from:

- the **selected** local networks in the model;
- the **learned** remote routes in the agent status;
- the **live** tunnel address and interface.

It skips `0.0.0.0/0`, `0.0.0.0/1`, `128.0.0.0/1`, invalid addresses, non-IPv4
routes and remote routes overlapping a local network. Rule changes are loaded
only into:

```
os-pangolin/nat
os-pangolin/fw
```

`Pangolin::pangolin_firewall()` registers those anchors through the official
OPNsense firewall plugin API, so the main ruleset references them without
editing generated pf files.

When the tunnel is down, the status has no valid tunnel address, the PID is not
alive, or the feature is disabled, the anchors are flushed and the rule files
are deleted.

## Unbound data flow

The client's DNS proxy holds the resource aliases received from Pangolin. The
agent snapshots them into `dns-records.json`. The plugin:

1. reads only aliases for a connected client;
2. checks every domain against manually managed Unbound host entries, aliases
   and domain overrides;
3. skips conflicts (manual entries always win);
4. writes `/usr/local/etc/unbound.opnsense.d/os-pangolin.conf` with
   `local-data`/`local-zone` directives;
5. restarts Unbound only when the generated content changed, so OPNsense's
   `start.sh` copies the include into the chroot;
6. removes the include when the client disconnects or DNS sync is disabled.

Optional conditional forwarding uses the official `OPNsense.Unbound` domain
override model. Only UUIDs recorded in `/var/db/pangolin/unbound-dots.json` are
modified or deleted; foreign entries are never touched.

Public domains are never rewritten: the server only sends private resource
aliases to the machine client. Public HTTPS resources continue to use their
normal public DNS records; SNI, Host headers and TLS are unaffected.

## HA and CARP

The agent can require a CARP state (`MASTER` or `BACKUP`) on a named interface:

- it waits before starting the tunnel;
- it monitors the state and cancels the tunnel when the state changes;
- unreadable CARP state after three checks is fail-closed.

Combined with the state/pid checks in the plugin, a standby node does not keep
NAT rules pointing at a dead tunnel. See [HA.md](HA.md).

## Upstream port

The port is intentionally small and isolated:

- new FreeBSD files for interface ioctl and route handling;
- FreeBSD dispatch cases;
- FreeBSD stubs for Linux-only resolver detection;
- FreeBSD TUN adapter for OLM;
- an alias snapshot API for embedders;
- a 0600 mode for Newt's credential cache.

See [../docs/PORTING.md](../docs/PORTING.md) and `third_party/README.md`.
