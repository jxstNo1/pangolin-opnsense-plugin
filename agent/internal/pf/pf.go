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
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	binary      = "/sbin/pfctl"
	natAnchor   = "os-pangolin/nat"
	fwAnchor    = "os-pangolin/fw"
	parentAnchor = "os-pangolin"
)

type Config struct {
	TunnelInterface string
	TunnelIP        string
	LocalNetworks   []string
	RemoteNetworks  []string
	NAT             bool
	Firewall        bool
	WorkDir         string
}

// Apply validates the rule inputs, checks that the OPNsense/FreeBSD anchor
// is actually referenced by the main ruleset, and then replaces only our
// anchors. It intentionally contains no default-route logic.
func Apply(ctx context.Context, cfg Config) error {
	if cfg.TunnelInterface == "" {
		return fmt.Errorf("pf: tunnel interface is required")
	}
	if cfg.TunnelIP == "" || net.ParseIP(cfg.TunnelIP).To4() == nil {
		return fmt.Errorf("pf: valid IPv4 tunnel address is required")
	}
	if len(cfg.LocalNetworks) == 0 {
		return fmt.Errorf("pf: at least one local network is required")
	}
	if err := validateCIDRs(cfg.LocalNetworks); err != nil {
		return fmt.Errorf("pf: local networks: %w", err)
	}
	if err := validateCIDRs(cfg.RemoteNetworks); err != nil {
		return fmt.Errorf("pf: remote networks: %w", err)
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = "/var/db/pangolin/pf"
	}
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return err
	}

	if cfg.NAT {
		if err := ensureAnchor(ctx, "nat"); err != nil {
			return err
		}
		rules := natRules(cfg)
		path := filepath.Join(workDir, "nat.conf")
		if err := os.WriteFile(path, []byte(rules), 0o600); err != nil {
			return err
		}
		if out, err := run(ctx, "-a", natAnchor, "-f", path); err != nil {
			return fmt.Errorf("pf: load NAT anchor: %w: %s", err, out)
		}
	}

	if cfg.Firewall {
		if err := ensureAnchor(ctx, "fw"); err != nil {
			return err
		}
		rules := filterRules(cfg)
		path := filepath.Join(workDir, "fw.conf")
		if err := os.WriteFile(path, []byte(rules), 0o600); err != nil {
			return err
		}
		if out, err := run(ctx, "-a", fwAnchor, "-f", path); err != nil {
			return fmt.Errorf("pf: load filter anchor: %w: %s", err, out)
		}
	}
	log.Printf("pf: applied subnet router on %s (NAT=%v firewall=%v)", cfg.TunnelInterface, cfg.NAT, cfg.Firewall)
	return nil
}

// Clear flushes only the os-pangolin anchors. It is idempotent and safe to
// call when pf is not running (the error is returned for logging only).
func Clear(ctx context.Context) error {
	var errs []string
	if out, err := run(ctx, "-a", fwAnchor, "-F", "rules"); err != nil {
		errs = append(errs, fmt.Sprintf("filter: %v: %s", err, out))
	}
	if out, err := run(ctx, "-a", natAnchor, "-F", "nat"); err != nil {
		errs = append(errs, fmt.Sprintf("nat: %v: %s", err, out))
	}
	if len(errs) > 0 {
		return fmt.Errorf("pf: clear anchors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func natRules(cfg Config) string {
	dst := "any"
	if len(cfg.RemoteNetworks) > 0 {
		dst = "{" + strings.Join(cfg.RemoteNetworks, ", ") + "}"
	}
	return fmt.Sprintf(
		"# managed by os-pangolin; do not edit\nnat on %s inet from {%s} to %s -> %s\n",
		cfg.TunnelInterface,
		strings.Join(cfg.LocalNetworks, ", "),
		dst,
		cfg.TunnelIP,
	)
}

func filterRules(cfg Config) string {
	dst := "any"
	if len(cfg.RemoteNetworks) > 0 {
		dst = "{" + strings.Join(cfg.RemoteNetworks, ", ") + "}"
	}
	src := "{" + strings.Join(cfg.LocalNetworks, ", ") + "}"
	var b strings.Builder
	b.WriteString("# managed by os-pangolin; do not edit\n")
	// Allow LAN clients to forward through the firewall. The source match is
	// restricted to the operator-selected VLANs; no rule is generated for
	// other local networks and no inter-VLAN pass rule is created.
	fmt.Fprintf(&b, "pass in quick inet from %s to %s keep state\n", src, dst)
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
		if ones == 0 {
			return fmt.Errorf("default route is not allowed")
		}
		if strings.ContainsAny(cidr, "\r\n") {
			return fmt.Errorf("CIDR contains newline")
		}
	}
	return nil
}

func run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// ParentAnchorName is exported so the plugin can share the constant without
// duplicating the spelling.
func ParentAnchorName() string { return parentAnchor }
