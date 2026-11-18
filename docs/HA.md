# High Availability (CARP)

## Scope for 0.1.0

The plugin is prepared for OPNsense HA but 0.1.0 does **not** claim a complete
HA implementation. The implemented part is a fail-closed CARP gate plus
state/PID-based firewall cleanup. A two-node failover test is still required.

## Active/passive gate

Configure in **VPN → Pangolin → Settings**:

- enable the CARP gate;
- select the interface carrying the CARP VIP (e.g. `em0`, `wan`, `lan`);
- select the required state (`MASTER` for the normal primary node).

The agent:

1. waits until `ifconfig <iface>` reports `carp: MASTER` (or the configured
   state);
2. starts the tunnel only in that state;
3. polls every 5 seconds;
4. cancels the tunnel if the state changes;
5. treats three consecutive unreadable states as failure and stops the tunnel.

It never starts a tunnel in an unknown state. This avoids two firewalls
competing for the same Pangolin identity and duplicate NAT/routes.

## Duplicate-route and duplicate-NAT avoidance

- The passive node does not run the agent.
- The plugin's `Pangolin::readAgentStatus()` checks the service PID and command
  name; a stale status file on the passive node cannot keep NAT rules alive.
- When the state changes to non-active, the hook triggers `pangolin sync`,
  which flushes `os-pangolin/nat` and `os-pangolin/fw`.
- The active node's TUN interface disappears on process exit, so kernel routes
  through it are removed automatically.
- The plugin never creates a default route; a stale remote route can only point
  at a dead TUN and is removed with it.

## Configuration synchronization

`pangolin_xmlrpc_sync()` declares the plugin section for OPNsense XML-RPC sync:

```php
[
  'description' => 'Pangolin',
  'section' => 'OPNsense.pangolin',
  'services' => ['pangolin_client', 'pangolin_site'],
]
```

This means the plugin configuration, **including the client/site secrets**, is
sent to the peer over the configured OPNsense sync channel. Protect that channel
and restrict access to both firewalls.

## Different identities per node (not in 0.1.0)

The model currently has one client identity and one site identity. A future
release should support:

- separate credentials per node, or
- an override file excluded from config sync, or
- a per-node identity selector stored outside `config.xml`.

Until then, use one of:

- active/passive with the same identity and the CARP gate (recommended for
  0.1.0);
- disable XML-RPC sync for the Pangolin section and configure each node
  manually if different identities are mandatory.

## Failover sequence

1. Primary loses MASTER; agent cancels the tunnel.
2. Primary hook calls `pangolin sync`; firewall anchors are flushed; DNS
   include is removed and Unbound is restarted.
3. Secondary becomes MASTER; its agent passes the CARP gate and connects.
4. Secondary learns routes and aliases; hook updates pf/Unbound.
5. LAN traffic resumes through the new active node.

DNS caches may retain old answers for the TTL (default 60 s). Keep the TTL low
during failover testing.

## Required tests before claiming HA support

- Two OPNsense VMs or physical nodes with a real CARP VIP.
- Forced failover (shutdown/crash/link loss) while a LAN client pings a remote
  Pangolin resource.
- Verify no duplicate NAT rules and no duplicate routes on the standby.
- Verify DNS cleanup and re-publication.
- Verify the active node reconnect after a WAN flap.
- Verify the standby does not start on boot while it is BACKUP.
- Verify a stale PID/status cannot keep anchors loaded after a crash.

Record the results in `docs/TESTING.md` and update this file.
