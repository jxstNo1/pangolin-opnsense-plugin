//go:build freebsd

package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"

	"github.com/fosrl/newt/logger"
)

// FreeBSD route management uses route(8) with an explicit argument vector.
// No shell is involved, every argument is validated as an address/CIDR or
// interface name before it reaches exec, and add operations refuse to replace
// a route that already points somewhere else. This keeps the backend from
// silently changing the default route or hijacking an operator-managed route.
// Interface addresses are configured through ioctl(2) in interface_freebsd.go.

const freeBSDRouteBinary = "/sbin/route"

var (
	freeBSDBypassMu    sync.Mutex
	freeBSDBypassOwned = make(map[string]bool)
)

// FreeBSDAddRouteWithSource adds a route for destination via gateway or
// interfaceName. sourceIP, when non-empty, is passed to route(8) -ifa so the
// kernel prefers it as the source address (needed when the interface carries
// both a site and an exit-node address).
func FreeBSDAddRouteWithSource(destination, gateway, interfaceName, sourceIP string) error {
	ip, ipNet, isHost, err := parseFreeBSDRouteDestination(destination)
	if err != nil {
		return err
	}
	if gateway == "" && interfaceName == "" {
		return fmt.Errorf("either gateway or interface must be specified")
	}
	if gateway != "" && net.ParseIP(gateway) == nil {
		return fmt.Errorf("invalid gateway address: %s", gateway)
	}
	if sourceIP != "" && net.ParseIP(sourceIP) == nil {
		return fmt.Errorf("invalid source address: %s", sourceIP)
	}

	// Refuse to replace an existing route to the same destination that is not
	// ours. route(8) would otherwise return "File exists", but checking first
	// lets us report the conflicting interface/gateway and makes reconciliation
	// idempotent across reconnects.
	if current, err := freeBSDRouteGet(destination); err == nil {
		if freeBSDRouteMatches(current, gateway, interfaceName) {
			logger.Debug("FreeBSD route to %s already present via %s", destination, current.iface)
			return nil
		}
		return fmt.Errorf(
			"route to %s already exists via %s (gateway %s); refusing to replace it",
			destination, current.iface, current.gateway,
		)
	}

	args := []string{"-n", "add"}
	if isHost {
		args = append(args, "-host", ip.String())
	} else {
		args = append(args, "-net", ipNet.String())
	}
	if gateway != "" {
		args = append(args, gateway)
	} else {
		args = append(args, "-interface", interfaceName)
	}
	if sourceIP != "" {
		args = append(args, "-ifa", sourceIP)
	}

	out, err := exec.Command(freeBSDRouteBinary, args...).CombinedOutput()
	if err != nil {
		// A concurrent add (another reconnect) can race the check above.
		if current, getErr := freeBSDRouteGet(destination); getErr == nil &&
			freeBSDRouteMatches(current, gateway, interfaceName) {
			return nil
		}
		return fmt.Errorf("route add %s failed: %v, output: %s", destination, err, strings.TrimSpace(string(out)))
	}
	logger.Info("Added FreeBSD route to %s via %s", destination, firstNonEmpty(gateway, interfaceName))
	return nil
}

func FreeBSDRemoveRoute(destination, interfaceName string) error {
	current, err := freeBSDRouteGet(destination)
	if err != nil {
		// Already gone. Route cleanup must be idempotent.
		return nil
	}
	if interfaceName != "" && current.iface != interfaceName {
		// Never delete a route that belongs to a different interface.
		return nil
	}

	_, ipNet, isHost, err := parseFreeBSDRouteDestination(destination)
	if err != nil {
		return err
	}
	args := []string{"-n", "delete"}
	if isHost {
		args = append(args, "-host", ipNet.IP.String())
	} else {
		args = append(args, "-net", ipNet.String())
	}
	out, err := exec.Command(freeBSDRouteBinary, args...).CombinedOutput()
	if err != nil {
		if _, getErr := freeBSDRouteGet(destination); getErr != nil {
			return nil
		}
		return fmt.Errorf("route delete %s failed: %v, output: %s", destination, err, strings.TrimSpace(string(out)))
	}
	logger.Info("Removed FreeBSD route to %s", destination)
	return nil
}

// FreeBSDAddBypassRoute installs a /32 host route for destIP via the current
// physical path. See AddBypassRouteForDestination in route.go for why this is
// required before a full-tunnel/default-route-equivalent is installed.
func FreeBSDAddBypassRoute(destIP string, tunnelInterface string) error {
	_, err := freebsdEnsureBypassRoute(destIP, tunnelInterface)
	return err
}

func freebsdEnsureBypassRoute(destIP string, tunnelInterface string) (bool, error) {
	ip := net.ParseIP(destIP).To4()
	if ip == nil {
		return false, fmt.Errorf("invalid IPv4 destination address: %s", destIP)
	}
	host := ip.String()

	current, err := freeBSDRouteGet(host)
	if err != nil {
		return false, fmt.Errorf("%w: %s", ErrNoPhysicalRoute, host)
	}

	// If the current path is already a static host route, it has been pinned
	// (possibly by us) and must be left alone.
	if isFreeBSDHostRoute(current) {
		freeBSDBypassMu.Lock()
		owned := freeBSDBypassOwned[host]
		freeBSDBypassMu.Unlock()
		if owned {
			return false, nil
		}
		// An operator-managed host route exists; do not replace it. It already
		// protects the destination from a broader tunnel route.
		return false, nil
	}

	if tunnelInterface != "" && current.iface == tunnelInterface {
		// The gateway route is already installed and capturing destIP. Use the
		// untouched physical default route instead.
		def, defErr := freeBSDRouteGet("default")
		if defErr != nil || def.iface == "" || def.iface == tunnelInterface {
			return false, fmt.Errorf("%w: %s", ErrNoPhysicalRoute, host)
		}
		current = def
	}

	var args []string
	args = append(args, "-n", "add", "-host", host)
	if gw := net.ParseIP(current.gateway); gw != nil {
		args = append(args, gw.String())
	} else if current.iface != "" {
		args = append(args, "-interface", current.iface)
	} else {
		return false, fmt.Errorf("%w: no gateway or interface for %s", ErrNoPhysicalRoute, host)
	}

	out, err := exec.Command(freeBSDRouteBinary, args...).CombinedOutput()
	if err != nil {
		// If a host route appeared in the meantime, it is doing the job.
		if after, getErr := freeBSDRouteGet(host); getErr == nil && isFreeBSDHostRoute(after) {
			return false, nil
		}
		return false, fmt.Errorf("failed to add bypass route to %s: %v, output: %s", host, err, strings.TrimSpace(string(out)))
	}
	freeBSDBypassMu.Lock()
	freeBSDBypassOwned[host] = true
	freeBSDBypassMu.Unlock()
	logger.Info("Added FreeBSD bypass route to %s via %s", host, firstNonEmpty(current.gateway, current.iface))
	return true, nil
}

func FreeBSDRemoveBypassRoute(destIP string) error {
	ip := net.ParseIP(destIP).To4()
	if ip == nil {
		return fmt.Errorf("invalid IPv4 destination address: %s", destIP)
	}
	host := ip.String()

	freeBSDBypassMu.Lock()
	owned := freeBSDBypassOwned[host]
	delete(freeBSDBypassOwned, host)
	freeBSDBypassMu.Unlock()
	if !owned {
		return nil
	}
	return FreeBSDRemoveRoute(host, "")
}

type freeBSDRoute struct {
	destination string
	mask        string
	gateway     string
	iface       string
	flags       []string
}

func (r freeBSDRoute) hasFlag(flag string) bool {
	for _, f := range r.flags {
		if f == flag {
			return true
		}
	}
	return false
}

func isFreeBSDHostRoute(r freeBSDRoute) bool {
	return r.mask == "255.255.255.255" || r.hasFlag("HOST")
}

// freeBSDRouteMatches reports whether the route is the one we would add.
func freeBSDRouteMatches(r freeBSDRoute, gateway, interfaceName string) bool {
	if interfaceName != "" && r.iface == interfaceName {
		return true
	}
	if gateway != "" && r.gateway == gateway {
		return true
	}
	return false
}

// freeBSDRouteGet parses route(8) get output for a destination or "default".
func freeBSDRouteGet(destination string) (freeBSDRoute, error) {
	if err := validateFreeBSDRouteLookupTarget(destination); err != nil {
		return freeBSDRoute{}, err
	}
	args := []string{"-n", "get"}
	if destination != "default" {
		args = append(args, "-inet")
	}
	args = append(args, destination)

	out, err := exec.Command(freeBSDRouteBinary, args...).CombinedOutput()
	if err != nil {
		return freeBSDRoute{}, fmt.Errorf("route get %s failed: %v, output: %s", destination, err, strings.TrimSpace(string(out)))
	}

	var r freeBSDRoute
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(key) {
		case "destination":
			r.destination = value
		case "mask":
			r.mask = value
		case "gateway":
			r.gateway = value
		case "interface":
			r.iface = value
		case "flags":
			r.flags = strings.Split(strings.Trim(value, "<>"), ",")
		}
	}
	if r.iface == "" && r.gateway == "" {
		return freeBSDRoute{}, fmt.Errorf("could not parse route get output for %s: %s", destination, out)
	}
	return r, nil
}

// parseFreeBSDRouteDestination validates a CIDR (or bare IP) and returns the
// host address, normalized network and whether this is a host route.
func parseFreeBSDRouteDestination(destination string) (net.IP, *net.IPNet, bool, error) {
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return nil, nil, false, fmt.Errorf("empty destination")
	}
	if !strings.Contains(destination, "/") {
		destination += "/32"
	}
	ip, ipNet, err := net.ParseCIDR(destination)
	if err != nil {
		return nil, nil, false, fmt.Errorf("invalid destination %q: %v", destination, err)
	}
	if ip.To4() == nil {
		return nil, nil, false, fmt.Errorf("FreeBSD route backend currently supports IPv4 only, got %s", destination)
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 32 {
		return nil, nil, false, fmt.Errorf("invalid IPv4 route %q", destination)
	}
	return ip, ipNet, ones == 32, nil
}

func validateFreeBSDRouteLookupTarget(destination string) error {
	if destination == "default" {
		return nil
	}
	if _, _, _, err := parseFreeBSDRouteDestination(destination); err != nil {
		return err
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
