//go:build freebsd

package dns

import (
	"fmt"
	"net/netip"
)

// IsSystemdResolvedAvailable is a FreeBSD stub for the Linux-only
// systemd-resolved detector used by DetectDNSManager. FreeBSD has neither
// systemd nor D-Bus, so the detector must fall through to resolvconf/file
// handling instead of failing to build.
func IsSystemdResolvedAvailable() bool {
	return false
}

// SystemdResolvedDNSConfigurator exists on FreeBSD only so the shared
// Unix DNS override code compiles. It is never selected by DetectDNSManager
// and every operation fails with a clear error if it is constructed directly.
type SystemdResolvedDNSConfigurator struct{}

func NewSystemdResolvedDNSConfigurator(interfaceName string) (*SystemdResolvedDNSConfigurator, error) {
	return nil, fmt.Errorf("systemd-resolved is not available on FreeBSD (interface %q)", interfaceName)
}

func (s *SystemdResolvedDNSConfigurator) SetDNS(servers []netip.Addr) ([]netip.Addr, error) {
	return nil, fmt.Errorf("systemd-resolved is not available on FreeBSD")
}

func (s *SystemdResolvedDNSConfigurator) RestoreDNS() error {
	return fmt.Errorf("systemd-resolved is not available on FreeBSD")
}

func (s *SystemdResolvedDNSConfigurator) GetCurrentDNS() ([]netip.Addr, error) {
	return nil, fmt.Errorf("systemd-resolved is not available on FreeBSD")
}

func (s *SystemdResolvedDNSConfigurator) Name() string {
	return "systemd-resolved-freebsd-stub"
}

func (s *SystemdResolvedDNSConfigurator) CleanupUncleanShutdown() error {
	return nil
}
