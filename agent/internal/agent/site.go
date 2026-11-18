package agent

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fosrl/newt/logger"
	"github.com/fosrl/newt/network"
	newtpkg "github.com/fosrl/newt/newt"

	"github.com/os-pangolin/os-pangolin/agent/internal/config"
	"github.com/os-pangolin/os-pangolin/agent/internal/state"
	"github.com/os-pangolin/os-pangolin/agent/internal/version"
)

// RunSite runs the Newt site connector. The default, fully supported mode is
// wireguard-go's user-space netstack: it runs natively on FreeBSD without a
// kernel TUN device and therefore without additional privileges. Native
// main-interface mode is available for advanced deployments and uses the
// FreeBSD TUN backend added in third_party/newt/network.
func RunSite(ctx context.Context, cfg *config.Config, store *state.Store, notify func(string)) error {
	if cfg.Site == nil {
		return fmt.Errorf("site configuration missing")
	}
	s := cfg.Site

	logger.Init(nil)
	logger.GetLogger().SetLevel(parseNewtLevel(firstNonEmpty(s.LogLevel, cfg.LogLevel)))

	healthFile := store.HealthPath()
	_ = os.Remove(healthFile)

	newtCfg := newtpkg.Config{
		Version:                 version.Version,
		Platform:                "freebsd",
		Agent:                   "os-pangolin",
		AgentVersion:            version.Version,
		LogLevel:                strings.ToUpper(firstNonEmpty(s.LogLevel, cfg.LogLevel)),
		Endpoint:                s.Endpoint,
		ID:                      s.ID,
		Secret:                  s.Secret,
		NewtName:                s.Name,
		MTU:                     s.MTU,
		InterfaceName:           s.InterfaceName,
		UseNativeInterface:      s.UseNativeClientInterface,
		UseNativeMainInterface:  s.UseNativeMainInterface,
		NativeMainInterfaceName: s.InterfaceName,
		DNS:                     firstNonEmpty(s.DNS, "1.1.1.1"),
		DisableSSH:              s.DisableSSH,
		DisableClients:          s.DisableClients,
		PingInterval:            mustDuration(s.PingInterval, 15*time.Second),
		PingTimeout:             mustDuration(s.PingTimeout, 7*time.Second),
		HealthFile:              healthFile,
		ConfigFile:              filepath.Join(cfg.StateDir, "site-newt.json"),
	}

	log.Printf("site: starting connector for %s (MTU %d, interface %s, native-main=%v)",
		s.Endpoint, s.MTU, s.InterfaceName, s.UseNativeMainInterface)

	n, err := newtpkg.Init(ctx, newtCfg)
	if err != nil {
		_, _, _ = store.SetError(err.Error())
		return fmt.Errorf("initialize site connector: %w", err)
	}

	prevConnected := false
	stopPoller := startPoller(ctx, 2*time.Second, func() {
		healthy := store.Healthy(90 * time.Second)
		settings := network.GetSettings()
		routes := includedRoutes(settings.IPv4IncludedRoutes)
		tunnelIP := firstAddress(settings.IPv4Addresses)
		iface := s.InterfaceName
		if !s.UseNativeMainInterface {
			iface = "netstack"
		}
		_, _, err := store.SetConnected(healthy, tunnelIP, iface, routes)
		if err != nil {
			log.Printf("site: write status: %v", err)
		}
		if healthy != prevConnected {
			prevConnected = healthy
			if healthy {
				notify("up")
			} else {
				notify("down")
			}
		}
	})

	// Newt blocks until the context is cancelled. It handles reconnects
	// internally; daemon(8) restarts the process if the library exits
	// unexpectedly (logger.Fatal calls os.Exit).
	n.Start(ctx)
	stopPoller()

	_ = os.Remove(healthFile)
	_, _, _ = store.SetConnected(false, "", "", nil)
	notify("down")
	log.Printf("site: connector stopped")
	return nil
}

func parseNewtLevel(level string) logger.LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return logger.DEBUG
	case "warn", "warning":
		return logger.WARN
	case "error":
		return logger.ERROR
	case "fatal":
		return logger.FATAL
	default:
		return logger.INFO
	}
}

func mustDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstAddress(addrs []string) string {
	for _, a := range addrs {
		if a != "" {
			return a
		}
	}
	return ""
}

func includedRoutes(routes []network.IPv4Route) []string {
	out := make([]string, 0, len(routes))
	for _, r := range routes {
		if r.IsDefault || r.DestinationAddress == "" || r.DestinationAddress == "0.0.0.0" {
			continue
		}
		dst := r.DestinationAddress
		if ip := net.ParseIP(r.SubnetMask).To4(); ip != nil {
			if ones, bits := net.IPMask(ip).Size(); bits == 32 {
				dst = fmt.Sprintf("%s/%d", dst, ones)
			}
		}
		out = append(out, dst)
	}
	return out
}

// startPoller runs fn immediately and then on every tick until ctx is done.
func startPoller(ctx context.Context, interval time.Duration, fn func()) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fn()
			}
		}
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
}
