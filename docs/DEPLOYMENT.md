# Deployment

## Supported installation paths

### 1. OPNsense ports plugin (recommended)

`plugin/Makefile` is a standard OPNsense plugin Makefile. Build it in an
OPNsense ports tree:

```sh
cd /usr/ports/net/os-pangolin   # or wherever the plugin is placed
make package
pkg install work/pkg/os-pangolin-0.1.0.pkg
```

The package must contain the FreeBSD agent binary at
`/usr/local/sbin/pangolin-agent`. Build it first with
`scripts/build-agent.sh` or let the port do it; a package without the binary
will fail at service start with `missing /usr/local/sbin/pangolin-agent`.

### 2. Development tarball

`make dist` produces `dist/os-pangolin-0.1.0-amd64.tgz`. On a lab firewall:

```sh
fetch -o /tmp/os-pangolin.tgz http://<host>/os-pangolin-0.1.0-amd64.tgz
tar -xzf /tmp/os-pangolin.tgz -C /tmp
cp -R /tmp/os-pangolin-0.1.0-amd64/src/* /usr/local/
chmod 755 /usr/local/sbin/pangolin-agent /usr/local/etc/rc.d/pangolin_*
chmod 700 /usr/local/etc/pangolin/hook.sh
service configd restart
```

Clear the OPNsense menu/ACL cache if the new menu entry does not appear:

```sh
rm -f /var/lib/php/tmp/opnsense_menu_cache.xml /var/lib/php/tmp/opnsense_acl_cache.json
configctl webgui restart
```

## First configuration

1. **VPN → Pangolin → Settings**
   - enable the plugin;
   - choose a log level;
   - leave `Allow plain HTTP endpoints` off;
   - optionally enable the CARP gate and select the CARP interface/state.
2. **Clients**
   - endpoint: `https://pangolin.example.com`;
   - machine client ID and secret from the Pangolin dashboard;
   - interface name `pangolin0`, MTU 1280 (or 1420 on a clean path);
   - save and restart.
3. **Sites** (optional, independent of the client)
   - site ID/secret;
   - default user-space netstack is recommended; native TUN is advanced.
4. **Subnet Routing**
   - enable;
   - choose `OPNsense firewall integration`;
   - add the local VLAN CIDRs, e.g. `10.10.0.0/16,10.20.0.0/16`;
   - optionally add an allowed remote-network allow-list;
   - keep NAT and filter rules enabled.
5. **DNS / Unbound** (optional)
   - enable alias synchronisation;
   - choose a reconciliation interval;
   - leave conditional forwarding off unless the deployment uses a private
     Pangolin resolver zone.
6. Click **Apply** on each page. The Apply action saves the model, calls
   `configctl pangolin sync`, writes 0600 agent JSON and starts/restarts the
   enabled service.

## Validation commands

```sh
/usr/local/sbin/pangolin-agent validate --config /usr/local/etc/pangolin/client.json
/usr/local/sbin/pangolin-agent validate --config /usr/local/etc/pangolin/site.json
/usr/local/sbin/pangolin-agent status --mode client --json
/usr/local/sbin/pangolin-agent status --mode site --json
configctl pangolin sync
configctl pangolin firewall-sync
configctl pangolin dns-sync
```

Inspect the firewall/DNS state:

```sh
pfctl -a os-pangolin/fw -sr
pfctl -a os-pangolin/nat -sn
cat /usr/local/etc/unbound.opnsense.d/os-pangolin.conf
cat /var/unbound/etc/os-pangolin.conf
drill @127.0.0.1 <alias>
tail -50 /var/log/pangolin/client.log
```

## Service control

```sh
service pangolin_client start|stop|restart|status
service pangolin_site start|stop|restart|status
```

Both rc.d scripts run `daemon(8)` with `-R 10 -C 128 -P <pidfile> -o <log>`.
The supervisor pidfile is the one exposed to rc.subr, so `stop` terminates the
supervisor and its child cleanly.

## Firewall expectations

The plugin registers two anchors through the OPNsense firewall plugin API:

```
anchor "os-pangolin/fw" all
nat-anchor "os-pangolin/nat" all
```

The generated rules are scoped to the configured local networks and the remote
routes learned from the client. There is no default route and no all-VLAN rule.
When the client is disconnected or the PID is dead, both anchors are flushed.

## Unbound expectations

The plugin writes only:

```
/usr/local/etc/unbound.opnsense.d/os-pangolin.conf
```

OPNsense copies that include to `/var/unbound/etc` on the next Unbound start.
The plugin stops Unbound directly and invokes
`/usr/local/opnsense/scripts/unbound/start.sh`; it does not call `configctl`
from inside configd, which would deadlock.

Conditional forwarding adds entries to the official Unbound domain-override
model. Only UUIDs stored in `/var/db/pangolin/unbound-dots.json` are touched.
The plugin does not widen Unbound's access control; only clients already
allowed to query Unbound can resolve the published records.

## Uninstall

Remove the package (or the files copied into `/usr/local`), then run the
`+POST_DEINSTALL` steps manually:

```sh
service pangolin_client stop
service pangolin_site stop
pfctl -a os-pangolin/fw -F rules
pfctl -a os-pangolin/nat -F nat
rm -f /usr/local/etc/unbound.opnsense.d/os-pangolin.conf
rm -f /usr/local/etc/pangolin/client.json /usr/local/etc/pangolin/site.json
rm -f /var/db/pangolin/*.json
rm -rf /var/db/pangolin/pf
```

Restart Unbound and reload the filter afterwards.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Menu missing | files under `/usr/local/opnsense/mvc/app/...`; clear menu/ACL cache; restart web GUI |
| `configctl pangolin` unknown | restart configd; verify `actions_pangolin*.conf` installed and readable |
| Service exits immediately | `/var/log/pangolin/*.log`; agent `validate`; endpoint reachability |
| `missing pangolin-agent` | build/copy `/usr/local/sbin/pangolin-agent`, mode 0755 |
| No NAT rules | client connected? `status` shows `tunnelIp`; `configctl pangolin firewall-sync`; check syslog |
| Anchor not referenced | run `configctl filter reload`; verify `pfctl -s rules`/`-s nat` |
| DNS alias missing | client mode and connected; DNS sync enabled; no manual conflict; check include and `drill` |
| Unbound not updated | check `/var/log/system/latest.log` for `pangolin:`; run `configctl pangolin dns-sync` |
| pf rule rejected | run `pfctl -a os-pangolin/fw -f /var/etc/pangolin/pf_fw.conf` manually and read the error |
