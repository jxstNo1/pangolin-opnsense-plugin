# End-to-end validation

These tests require a real, authorized Pangolin test instance. They are not run
by CI and must never be pointed at a production firewall without explicit
approval.

## Credentials file

Create a config file **outside this repository**, mode 0600, owned by root:

```json
{
  "mode": "client",
  "logLevel": "info",
  "stateDir": "/tmp/pangolin-e2e/state",
  "hookScript": "",
  "client": {
    "endpoint": "https://pangolin-test.example.invalid",
    "id": "REDACTED",
    "secret": "REDACTED",
    "orgId": "",
    "mtu": 1280,
    "interfaceName": "pangolin0",
    "dns": "1.1.1.1",
    "upstreamDns": ["1.1.1.1:53"],
    "pingInterval": "5s",
    "pingTimeout": "5s",
    "holepunch": true,
    "preferLocalRoutes": false,
    "disableRoutesAndAliasesOnExitNode": false,
    "overrideDns": false
  },
  "subnetRouter": {
    "enabled": false,
    "backend": "none",
    "localNetworks": [],
    "allowedRemoteNetworks": [],
    "nat": false,
    "firewall": false
  },
  "dns": {
    "enabled": false,
    "syncInterval": "300s",
    "outputFile": "/tmp/pangolin-e2e/state/dns-records.json",
    "forwardDomains": [],
    "ttl": 60
  },
  "ha": {
    "enabled": false,
    "interface": "",
    "requiredState": "MASTER"
  }
}
```

`stateDir` must not be `/var/db/pangolin`; the smoke test refuses that.

## Machine-client smoke test

On the FreeBSD/OPNsense test host:

```sh
chmod 600 /root/pangolin-e2e-client.json
PANGOLIN_E2E_CONFIG=/root/pangolin-e2e-client.json tests/e2e/client-smoke.sh
```

Expected evidence:

- `configuration OK`;
- `connected=true` and a non-empty `tunnelIp`;
- a real `pangolin0` TUN interface during the run;
- clean shutdown and `connected=false` afterwards;
- no client secret or bearer token in the captured log.

If the endpoint is unreachable, the script fails after 180 seconds and prints
only a redacted log tail. That is a failed test, not a skipped one.

## Subnet-router test (manual, isolated OPNsense VM)

Prerequisites:

- a second VM or host on a test VLAN whose default gateway is the OPNsense VM;
- a remote private resource reachable through the same Pangolin test account;
- explicit approval to modify only the isolated test network.

1. Configure the machine client and subnet router in **VPN → Pangolin**:
   local network = test VLAN CIDR, remote allow-list = target subnet,
   backend = OPNsense.
2. Apply and verify `status` shows `connected: true`.
3. From the test host, `ping`/`nc`/`curl` the remote resource by IP and alias.
4. `tcpdump -ni <tun> host <resource>` and `pfctl -ss` must show the SNAT.
5. Try the same access from an unselected VLAN; it must fail.
6. Reload the firewall (`configctl filter reload`) and repeat; the rule must
   still be present.
7. Kill the agent and verify anchors/DNS are cleaned; restart and verify
   recovery.

Record the exact commands, before/after outputs and packet captures in
`docs/TESTING.md`. Do not paste secrets.

## HA test

Requires two isolated OPNsense VMs with CARP and a real VIP. Follow
`docs/HA.md`; a single-node QEMU test is not sufficient.

## What the current fixtures do and do not prove

The QEMU fixtures in `tests/integration/qemu/` prove configd/MVC/pf/Unbound
wiring with a simulated connected status. They do **not** prove Pangolin
authentication, WireGuard handshakes, hole punching, relay fallback or the real
data plane. Only the tests above do.
