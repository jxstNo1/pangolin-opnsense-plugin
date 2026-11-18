// Package pf implements the standalone FreeBSD subnet-router backend.
//
// On OPNsense the plugin owns pf integration (backend "opnsense"): it
// registers the same anchors through the official OPNsense firewall plugin
// API and loads rules from a configd action. This package is used on plain
// FreeBSD, where the operator must reference the anchors from /etc/pf.conf:
//
//	nat-anchor "os-pangolin/nat"
//	anchor "os-pangolin/fw"
//
// The backend never edits /etc/pf.conf and never flushes the global ruleset.
package pf

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	binary       = "/sbin/pfctl"
	natAnchor    = "os-pangolin/nat"
	fwAnchor     = "os-pangolin/fw"
	parentAnchor = "os-pangolin"
)

var (
	// ErrNoRemoteRoutes means the tunnel currently has no usable destination.
	// The anchors have been flushed; this is not a failure to log as an error.
	ErrNoRemoteRoutes = errors.New("pf: no usable remote routes")
	// ErrNoRules means both NAT and firewall integration are disabled.
	ErrNoRules = errors.New("pf: NAT and firewall integration disabled")
)

type Config struct {
	TunnelInterface        string
	TunnelIP               string
	LocalNetworks          []string
	LocalInterfaces        []string
	LocalInterfaceNetworks []string
	RemoteNetworks         []string
	AllowedRemoteNetworks  []string
	NAT                    bool
	Firewall               bool
	WorkDir                string
}

// Apply validates the rule inputs, checks that the OPNsense/FreeBSD anchor
// is actually referenced by the main ruleset, and then replaces only our
// anchors. It intentionally contains no default-route logic.
func Apply(ctx context.Context, cfg Config) error {
	if cfg.TunnelInterface == "" {
		return failClosed(ctx, fmt.Errorf("pf: tunnel interface is required"))
	}
	if err := validateInterfaceNames([]string{cfg.TunnelInterface}); err != nil {
		return failClosed(ctx, err)
	}
	if cfg.TunnelIP == "" || net.ParseIP(cfg.TunnelIP).To4() == nil {
		return failClosed(ctx, fmt.Errorf("pf: valid IPv4 tunnel address is required"))
	}
	if len(cfg.LocalNetworks) == 0 {
		return failClosed(ctx, fmt.Errorf("pf: at least one local network is required"))
	}
	if err := validateCIDRs(cfg.LocalNetworks); err != nil {
		return failClosed(ctx, fmt.Errorf("pf: local networks: %w", err))
	}
	if len(cfg.LocalInterfaces) == 0 {
		interfaces, err := resolveLocalInterfaces(cfg.LocalNetworks)
		if err != nil {
			return failClosed(ctx, err)
		}
		cfg.LocalInterfaces = interfaces
	}
	if err := validateInterfaceNames(cfg.LocalInterfaces); err != nil {
		return failClosed(ctx, err)
	}
	if err := validateCIDRs(cfg.RemoteNetworks); err != nil {
		return failClosed(ctx, fmt.Errorf("pf: remote networks: %w", err))
	}
	if err := validateCIDRs(cfg.AllowedRemoteNetworks); err != nil {
		return failClosed(ctx, fmt.Errorf("pf: allowed remote networks: %w", err))
	}
	remote, err := filterRemoteNetworks(cfg.RemoteNetworks, cfg.AllowedRemoteNetworks)
	if err != nil {
		return failClosed(ctx, err)
	}
	cfg.RemoteNetworks = remote

	/* Reject learned destinations that overlap any local interface network.
	 * The selected source networks alone are not enough: a server could
	 * advertise another local VLAN and turn the quick pass rule into an
	 * inter-VLAN bypass. */
	blocked := append([]string(nil), cfg.LocalInterfaceNetworks...)
	if len(blocked) == 0 {
		networks, err := localInterfaceNetworks(cfg.TunnelInterface)
		if err != nil {
			return failClosed(ctx, err)
		}
		blocked = networks
	}
	blocked = append(blocked, cfg.LocalNetworks...)
	cfg.RemoteNetworks = pruneRemoteRoutes(cfg.RemoteNetworks, blocked)

	/* No learned destination means no NAT and no pass rule. Never turn an
	 * empty route set into "any": that would open every destination. */
	if len(cfg.RemoteNetworks) == 0 {
		log.Printf("pf: no remote routes; clearing anchors")
		if err := Clear(ctx); err != nil {
			return err
		}
		return ErrNoRemoteRoutes
	}
	if !cfg.NAT && !cfg.Firewall {
		log.Printf("pf: NAT and firewall disabled; clearing anchors")
		if err := Clear(ctx); err != nil {
			return err
		}
		return ErrNoRules
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = "/var/db/pangolin/pf"
	}
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return failClosed(ctx, err)
	}

	if cfg.NAT {
		if err := ensureAnchor(ctx, "nat"); err != nil {
			return failClosed(ctx, err)
		}
		rules := natRules(cfg)
		path := filepath.Join(workDir, "nat.conf")
		if err := os.WriteFile(path, []byte(rules), 0o600); err != nil {
			return failClosed(ctx, err)
		}
		if out, err := run(ctx, "-a", natAnchor, "-f", path); err != nil {
			return failClosed(ctx, fmt.Errorf("pf: load NAT anchor: %w: %s", err, out))
		}
	} else if err := clearAnchor(ctx, "nat"); err != nil {
		return failClosed(ctx, err)
	}

	if cfg.Firewall {
		if err := ensureAnchor(ctx, "fw"); err != nil {
			return failClosed(ctx, err)
		}
		rules := filterRules(cfg)
		path := filepath.Join(workDir, "fw.conf")
		if err := os.WriteFile(path, []byte(rules), 0o600); err != nil {
			return failClosed(ctx, err)
		}
		if out, err := run(ctx, "-a", fwAnchor, "-f", path); err != nil {
			return failClosed(ctx, fmt.Errorf("pf: load filter anchor: %w: %s", err, out))
		}
	} else if err := clearAnchor(ctx, "fw"); err != nil {
		return failClosed(ctx, err)
	}
	log.Printf("pf: applied subnet router on %s (NAT=%v firewall=%v, %d remote route(s))",
		cfg.TunnelInterface, cfg.NAT, cfg.Firewall, len(cfg.RemoteNetworks))
	return nil
}

// failClosed attempts to remove stale anchors before returning a validation or
// load error. It is better to lose the rules than to keep old ones after the
// configuration became invalid.
func failClosed(ctx context.Context, cause error) error {
	if err := Clear(ctx); err != nil {
		return fmt.Errorf("%w; additionally failed to clear anchors: %v", cause, err)
	}
	return cause
}

// Clear flushes only the os-pangolin anchors. It is idempotent and safe to
// call when pf is not running (the error is returned for logging only).
func Clear(ctx context.Context) error {
	var errs []string
	if err := clearAnchor(ctx, "fw"); err != nil {
		errs = append(errs, err.Error())
	}
	if err := clearAnchor(ctx, "nat"); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("pf: clear anchors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func clearAnchor(ctx context.Context, kind string) error {
	var args []string
	switch kind {
	case "nat":
		args = []string{"-a", natAnchor, "-F", "nat"}
	case "fw":
		args = []string{"-a", fwAnchor, "-F", "rules"}
	default:
		return fmt.Errorf("pf: unknown anchor kind %q", kind)
	}
	if out, err := run(ctx, args...); err != nil {
		return fmt.Errorf("pf: clear %s anchor: %w: %s", kind, err, out)
	}
	return nil
}

func natRules(cfg Config) string {
	if len(cfg.RemoteNetworks) == 0 {
		/* Apply clears the anchors instead of loading this file. Keep the
		 * builder defensive: an empty destination must never become "any". */
		return "# managed by os-pangolin; do not edit\n"
	}
	return fmt.Sprintf(
		"# managed by os-pangolin; do not edit\nnat on %s inet from {%s} to {%s} -> %s\n",
		cfg.TunnelInterface,
		strings.Join(cfg.LocalNetworks, ", "),
		strings.Join(cfg.RemoteNetworks, ", "),
		cfg.TunnelIP,
	)
}

func filterRules(cfg Config) string {
	var b strings.Builder
	b.WriteString("# managed by os-pangolin; do not edit\n")
	if len(cfg.RemoteNetworks) == 0 || len(cfg.LocalInterfaces) == 0 {
		return b.String()
	}
	dst := "{" + strings.Join(cfg.RemoteNetworks, ", ") + "}"
	src := "{" + strings.Join(cfg.LocalNetworks, ", ") + "}"
	ifaceList := cfg.LocalInterfaces[0]
	if len(cfg.LocalInterfaces) > 1 {
		ifaceList = "{ " + strings.Join(cfg.LocalInterfaces, ", ") + " }"
	}
	// Allow LAN clients to forward through the firewall. The source match is
	// restricted to the operator-selected VLANs and the ingress rule is bound
	// to the interface that carries them; no rule is generated for other local
	// networks and no inter-VLAN pass rule is created.
	fmt.Fprintf(&b, "pass in quick on %s inet from %s to %s keep state\n", ifaceList, src, dst)
	fmt.Fprintf(&b, "pass out quick on %s inet from %s to %s keep state\n", cfg.TunnelInterface, src, dst)
	// DHCP/DNS traffic to the firewall itself is intentionally not part of
	// this anchor; OPNsense's normal interface rules continue to govern it.
	return b.String()
}

func ensureAnchor(ctx context.Context, kind string) error {
	var args []string
	var want string
	switch kind {
	case "nat":
		args = []string{"-s", "nat"}
		want = fmt.Sprintf("nat-anchor \"%s\"", natAnchor)
	case "fw":
		args = []string{"-s", "rules"}
		want = fmt.Sprintf("anchor \"%s\"", fwAnchor)
	default:
		return fmt.Errorf("pf: unknown anchor kind %q", kind)
	}
	out, err := run(ctx, args...)
	if err != nil {
		return fmt.Errorf("pf: inspect rules: %w: %s", err, out)
	}
	if !strings.Contains(out, want) {
		return fmt.Errorf("pf: anchor declaration %s is missing; add it to /etc/pf.conf or enable the OPNsense plugin firewall integration", want)
	}
	return nil
}

func validateCIDRs(cidrs []string) error {
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			return fmt.Errorf("empty CIDR")
		}
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("%q: %v", cidr, err)
		}
		if ip.To4() == nil {
			return fmt.Errorf("%q is not IPv4", cidr)
		}
		ones, bits := ipNet.Mask.Size()
		if bits != 32 {
			return fmt.Errorf("%q is not an IPv4 network", cidr)
		}
		if ones < 8 {
			return fmt.Errorf("%q is too broad; prefix must be between /8 and /32 (default routes are not allowed)", cidr)
		}
		if !isRoutableIPv4(ipNet.IP) {
			return fmt.Errorf("%q is not a routable unicast IPv4 network", cidr)
		}
		if strings.ContainsAny(cidr, "\r\n") {
			return fmt.Errorf("CIDR contains newline")
		}
	}
	return nil
}

func isRoutableIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	if ip4[0] == 0 || ip4[0] == 127 || ip4[0] >= 224 {
		return false
	}
	if ip4[0] == 169 && ip4[1] == 254 {
		return false
	}
	if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0 {
		/* 192.0.0.0/24 IETF protocol assignments */
		return false
	}
	if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2 {
		/* 192.0.2.0/24 TEST-NET-1 */
		return false
	}
	if ip4[0] == 192 && ip4[1] == 88 && ip4[2] == 99 {
		/* 192.88.99.0/24 deprecated 6to4 relay anycast */
		return false
	}
	if ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19) {
		/* 198.18.0.0/15 benchmarking */
		return false
	}
	if ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100 {
		/* 198.51.100.0/24 TEST-NET-2 */
		return false
	}
	if ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113 {
		/* 203.0.113.0/24 TEST-NET-3 */
		return false
	}
	return true
}

type ifaceNetwork struct {
	name    string
	network *net.IPNet
}

// resolveLocalInterfaces maps the selected source networks to the interfaces
// that actually carry them. The standalone backend must bind its inbound pass
// rule to those interfaces; otherwise the anchor can bypass interface policy.
func resolveLocalInterfaces(local []string) ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("pf: enumerate interfaces: %w", err)
	}
	var entries []ifaceNetwork
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ipNet, ok := address.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			entries = append(entries, ifaceNetwork{name: iface.Name, network: ipNet})
		}
	}
	out := interfacesForNetworks(local, entries)
	if len(out) == 0 {
		return nil, fmt.Errorf("pf: no interface carries the local networks %v", local)
	}
	for _, cidr := range local {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			continue
		}
		matched := false
		for _, entry := range entries {
			if cidrOverlaps(network, entry.network) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("pf: local network %s is not assigned to an interface", cidr)
		}
	}
	return out, nil
}

func interfacesForNetworks(local []string, entries []ifaceNetwork) []string {
	set := make(map[string]bool)
	for _, cidr := range local {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if cidrOverlaps(network, entry.network) {
				set[entry.name] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// localInterfaceNetworks returns every non-loopback IPv4 network on the host,
// except the tunnel interface itself. Learned remote routes must not overlap
// any of these.
func localInterfaceNetworks(tunnelInterface string) ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("pf: enumerate interfaces: %w", err)
	}
	var out []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Name == tunnelInterface {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ipNet, ok := address.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			out = append(out, ipNet.String())
		}
	}
	sort.Strings(out)
	return out, nil
}

// pruneRemoteRoutes drops routes that overlap any blocked local network. An
// empty result means the caller must flush the anchors, never emit "any".
func pruneRemoteRoutes(remote, blocked []string) []string {
	blockedNets := make([]*net.IPNet, 0, len(blocked))
	for _, cidr := range blocked {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			continue
		}
		blockedNets = append(blockedNets, network)
	}
	out := make([]string, 0, len(remote))
	for _, cidr := range remote {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			continue
		}
		overlaps := false
		for _, blockedNet := range blockedNets {
			if cidrOverlaps(network, blockedNet) {
				overlaps = true
				break
			}
		}
		if overlaps {
			log.Printf("pf: remote route %s overlaps a local interface network; skipped", cidr)
			continue
		}
		out = append(out, cidr)
	}
	return out
}

func cidrOverlaps(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}

var interfaceRE = regexp.MustCompile(`^[A-Za-z0-9_.]{1,15}$`)

func validateInterfaceNames(names []string) error {
	for _, name := range names {
		if !interfaceRE.MatchString(name) {
			return fmt.Errorf("pf: invalid interface name %q", name)
		}
	}
	return nil
}

// filterRemoteNetworks applies the optional allow-list. A remote route is
// accepted only if its entire prefix is contained in one of the allowed
// prefixes. An empty allow-list keeps all validated non-default routes.
func filterRemoteNetworks(remote, allowed []string) ([]string, error) {
	if len(allowed) == 0 {
		return remote, nil
	}
	allowedNets := make([]*net.IPNet, 0, len(allowed))
	for _, cidr := range allowed {
		_, n, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			return nil, fmt.Errorf("pf: allowed remote network %q: %w", cidr, err)
		}
		allowedNets = append(allowedNets, n)
	}
	out := make([]string, 0, len(remote))
	for _, cidr := range remote {
		ip, n, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			continue
		}
		ones, bits := n.Mask.Size()
		for _, allowedNet := range allowedNets {
			aOnes, aBits := allowedNet.Mask.Size()
			if bits == aBits && ones >= aOnes && allowedNet.Contains(ip) {
				out = append(out, cidr)
				break
			}
		}
	}
	return out, nil
}

func run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// ParentAnchorName is exported so the plugin can share the constant without
// duplicating the spelling.
func ParentAnchorName() string { return parentAnchor }
