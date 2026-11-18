//go:build !freebsd

package network

import (
	"fmt"
	"net"
)

// Stubs keep the network package buildable on the development host and on
// non-FreeBSD targets. They are never reached at runtime on those platforms
// because the dispatch switches only select the freebsd cases when
// runtime.GOOS == "freebsd".

func configureFreeBSD(interfaceName string, ip net.IP, ipNet *net.IPNet, mtu int) error {
	return fmt.Errorf("FreeBSD network backend is not available on this platform")
}

func addFreeBSDAddress(interfaceName string, ip net.IP, ipNet *net.IPNet) error {
	return fmt.Errorf("FreeBSD network backend is not available on this platform")
}

func removeFreeBSDAddress(interfaceName string, ip net.IP, ipNet *net.IPNet) error {
	return fmt.Errorf("FreeBSD network backend is not available on this platform")
}

func FreeBSDAddRouteWithSource(destination, gateway, interfaceName, sourceIP string) error {
	return fmt.Errorf("FreeBSD network backend is not available on this platform")
}

func FreeBSDRemoveRoute(destination, interfaceName string) error {
	return fmt.Errorf("FreeBSD network backend is not available on this platform")
}

func FreeBSDAddBypassRoute(destIP string, tunnelInterface string) error {
	return fmt.Errorf("FreeBSD network backend is not available on this platform")
}

func freebsdEnsureBypassRoute(destIP string, tunnelInterface string) (bool, error) {
	return false, fmt.Errorf("FreeBSD network backend is not available on this platform")
}

func FreeBSDRemoveBypassRoute(destIP string) error {
	return fmt.Errorf("FreeBSD network backend is not available on this platform")
}
