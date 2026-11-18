package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	newtLogger "github.com/fosrl/newt/logger"
	"github.com/fosrl/newt/network"
	olmdns "github.com/fosrl/olm/dns"
	olm "github.com/fosrl/olm/olm"

	"github.com/os-pangolin/os-pangolin/agent/internal/config"
	"github.com/os-pangolin/os-pangolin/agent/internal/pf"
	"github.com/os-pangolin/os-pangolin/agent/internal/state"
	"github.com/os-pangolin/os-pangolin/agent/internal/version"
)

// RunClient runs the Pangolin machine client (olm) with the FreeBSD network
// backend. The olm DNS override is always disabled: on OPNsense, Unbound owns
// port 53 and the plugin synchronises aliases through Unbound's include
// directory instead.
func RunClient(ctx context.Context, cfg *config.Config, store *state.Store, notify func(string)) error {
	if cfg.Client == nil {
		return fmt.Errorf("client configuration missing")
	}
	c := cfg.Client

	newtLogger.Init(nil)
	newtLogger.GetLogger().SetLevel(parseNewtLevel(firstNonEmpty(c.LogLevel, cfg.LogLevel)))

	// olm creates its DNS proxy even when the system override is disabled, and
	// requires at least one upstream resolver. The plugin normally supplies
	// one; keep a safe fallback for hand-written development configs.
	upstream := append([]string(nil), c.UpstreamDNS...)
	if len(upstream) == 0 {
		upstream = []string{"1.1.1.1:53", "8.8.8.8:53"}
	}
	dnsServer := c.DNS
	if dnsServer == "" {
		dnsServer = "1.1.1.1"
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var connected atomic.Bool
	var registered atomic.Bool

	srStatus := &state.SubnetRouterStatus{
		Enabled:       cfg.SubnetRouter != nil && cfg.SubnetRouter.Enabled,
		Backend:       subnetBackend(cfg),
		LocalNetworks: append([]string(nil), localNetworks(cfg)...),
	}

	onStateChange := func() {
		settings := network.GetSettings()
		tunnelIP := firstAddress(settings.IPv4Addresses)
		routes := includedRoutes(settings.IPv4IncludedRoutes)
		iface := c.InterfaceName
		prev, _, err := store.SetConnected(connected.Load(), tunnelIP, iface, routes)
		if err != nil {
			log.Printf("client: write status: %v", err)
		}
		if connected.Load() && prev.TunnelIP != tunnelIP && tunnelIP != "" {
			notify("routes")
		}
		if !connected.Load() {
			notify("down")
		}
	}

	olmCfg := olm.OlmConfig{
		LogLevel: strings.ToUpper(firstNonEmpty(c.LogLevel, cfg.LogLevel)),
		Version:  version.Version,
		Agent:    "os-pangolin",
		OnRegistered: func() {
			registered.Store(true)
			_, _, _ = store.Update(func(st *state.Status) { st.Registered = true })
		},
		OnConnected: func() {
			connected.Store(true)
			registered.Store(true)
			onStateChange()
			notify("up")
		},
		OnTerminated: func() {
			connected.Store(false)
			registered.Store(false)
			_, _, _ = store.SetConnected(false, "", "", nil)
			notify("down")
			// olm has already torn the tunnel down; exit so daemon(8) can
			// restart us with a clean process after auth/termination errors.
			cancel()
		},
		OnAuthError: func(statusCode int, message string) {
			_, _, _ = store.SetError(fmt.Sprintf("authentication failed (%d): %s", statusCode, message))
		},
		OnOlmError: func(code, message string) {
			_, _, _ = store.SetError(fmt.Sprintf("%s: %s", code, message))
		},
	}

	o, err := olm.Init(runCtx, olmCfg)
	if err != nil {
		_, _, _ = store.SetError(err.Error())
		return fmt.Errorf("initialize machine client: %w", err)
	}

	tunnelCfg := olm.TunnelConfig{
		Endpoint:                          c.Endpoint,
		ID:                                c.ID,
		Secret:                            c.Secret,
		OrgID:                             c.OrgID,
		MTU:                               c.MTU,
		DNS:                               dnsServer,
		UpstreamDNS:                       upstream,
		InterfaceName:                     c.InterfaceName,
		Holepunch:                         c.Holepunch,
		PingIntervalDuration:              mustDuration(c.PingInterval, 5*time.Second),
		PingTimeoutDuration:               mustDuration(c.PingTimeout, 5*time.Second),
		PreferLocalRoutes:                 c.PreferLocalRoutes,
		DisableRoutesAndAliasesOnExitNode: c.DisableRoutesAndAliasesOnExitNode,
		// Disabled intentionally: FreeBSD subnet routing is implemented by the
		// agent/plugin (OPNsense backend or the pf anchor backend), and DNS is
		// owned by Unbound.
		OverrideDNS:  false,
		TunnelDNS:    false,
		SubnetRouter: false,
		EnableUAPI:   false,
	}

	log.Printf("client: connecting to %s (interface %s, MTU %d, subnet-router=%v/%s)",
		c.Endpoint, c.InterfaceName, c.MTU, srStatus.Enabled, srStatus.Backend)

	go o.StartTunnel(tunnelCfg)

	dnsPath := cfg.DNS.OutputFile
	if dnsPath == "" {
		dnsPath = store.DNSRecordsPath()
	}
	_ = os.MkdirAll(filepath.Dir(dnsPath), 0o755)

	var lastDNS []state.DNSEntry
	var lastApplied bool

	stopPoller := startPoller(runCtx, 2*time.Second, func() {
		settings := network.GetSettings()
		tunnelIP := firstAddress(settings.IPv4Addresses)
		routes := includedRoutes(settings.IPv4IncludedRoutes)
		iface := c.InterfaceName
		prev := store.Snapshot()
		_, changed, err := store.SetConnected(connected.Load(), tunnelIP, iface, routes)
		if err != nil {
			log.Printf("client: write status: %v", err)
		}
		if changed && connected.Load() && tunnelIP != "" && prev.TunnelIP != tunnelIP {
			notify("routes")
		}

		records := snapshotDNS(o.DNSRecords())
		if !sameDNSEntries(records, lastDNS) {
			lastDNS = records
			if err := store.WriteDNSRecords(records); err != nil {
				log.Printf("client: write DNS snapshot: %v", err)
			} else if cfg.DNS != nil && cfg.DNS.Enabled {
				notify("dns")
			}
		}
		_, _, _ = store.SetDNSProxyIP(o.DNSProxyIP())

		if cfg.SubnetRouter != nil && cfg.SubnetRouter.Enabled {
			srStatus.TunnelIP = tunnelIP
			applied := connected.Load() && tunnelIP != ""
			srStatus.Applied = applied
			if applied && cfg.SubnetRouter.Backend == config.SubnetRouterPF {
				pfCfg := pf.Config{
					TunnelInterface: c.InterfaceName,
					TunnelIP:        tunnelIP,
					LocalNetworks:   cfg.SubnetRouter.LocalNetworks,
					RemoteNetworks:  routes,
					NAT:             cfg.SubnetRouter.NAT,
					Firewall:        cfg.SubnetRouter.Firewall,
					WorkDir:         filepath.Join(cfg.StateDir, "pf"),
				}
				if err := pf.Apply(runCtx, pfCfg); err != nil {
					srStatus.Applied = false
					srStatus.LastError = err.Error()
					log.Printf("client: %v", err)
				} else {
					srStatus.LastError = ""
				}
			}
			if applied != lastApplied {
				lastApplied = applied
				notify("routes")
			}
			_, _, _ = store.SetSubnetRouter(srStatus)
		}
	})

	<-runCtx.Done()
	stopPoller()
	if cfg.SubnetRouter != nil && cfg.SubnetRouter.Enabled && cfg.SubnetRouter.Backend == config.SubnetRouterPF {
		clearCtx, clearCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := pf.Clear(clearCtx); err != nil {
			log.Printf("client: clear pf anchors: %v", err)
		}
		clearCancel()
	}
	o.Close()
	_, _, _ = store.SetConnected(false, "", "", nil)
	notify("down")
	log.Printf("client: stopped")
	return nil
}

func subnetBackend(cfg *config.Config) string {
	if cfg.SubnetRouter == nil {
		return config.SubnetRouterNone
	}
	return cfg.SubnetRouter.Backend
}

func localNetworks(cfg *config.Config) []string {
	if cfg.SubnetRouter == nil {
		return nil
	}
	out := make([]string, 0, len(cfg.SubnetRouter.LocalNetworks))
	for _, n := range cfg.SubnetRouter.LocalNetworks {
		out = append(out, strings.TrimSpace(n))
	}
	sort.Strings(out)
	return out
}

func snapshotDNS(records []olmdns.SnapshotRecord) []state.DNSEntry {
	out := make([]state.DNSEntry, 0, len(records))
	for _, r := range records {
		out = append(out, state.DNSEntry{
			Domain: r.Domain,
			IP:     r.IP,
			Type:   r.Type,
			SiteID: r.SiteID,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Domain != out[j].Domain {
			return out[i].Domain < out[j].Domain
		}
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].IP < out[j].IP
	})
	return out
}

func sameDNSEntries(a, b []state.DNSEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
