//go:build freebsd

package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"unsafe"

	"github.com/fosrl/newt/logger"
	"golang.org/x/sys/unix"
)

// ifaliasreq mirrors FreeBSD's struct ifaliasreq from <net/if.h>. The
// structure is used with SIOCAIFADDR/SIOCDIFADDR to add and remove IPv4
// addresses without invoking ifconfig(8).
type ifaliasreq struct {
	Name      [unix.IFNAMSIZ]byte
	Addr      unix.RawSockaddrInet4
	BroadAddr unix.RawSockaddrInet4
	Mask      unix.RawSockaddrInet4
}

// ifreq is large enough to hold FreeBSD's struct ifreq (IFNAMSIZ bytes of
// name followed by a 16-byte union).
type ifreq [32]byte

// configureFreeBSD assigns the tunnel address, brings the interface up and
// applies the MTU. wireguard-go's FreeBSD TUN driver creates a point-to-point
// interface, so the peer/destination address is set to the local address; this
// is the same convention used by ifconfig(8) for tun(4).
func configureFreeBSD(interfaceName string, ip net.IP, ipNet *net.IPNet, mtu int) error {
	if NativeConfigDisabled {
		return nil
	}
	if interfaceName == "" {
		return fmt.Errorf("freebsd: interface name is required")
	}

	if err := addFreeBSDAddress(interfaceName, ip, ipNet); err != nil {
		return fmt.Errorf("freebsd: assign address on %s: %w", interfaceName, err)
	}
	if err := setFreeBSDFlags(interfaceName, unix.IFF_UP); err != nil {
		return fmt.Errorf("freebsd: bring %s up: %w", interfaceName, err)
	}
	if mtu > 0 {
		if err := setFreeBSDMTU(interfaceName, mtu); err != nil {
			return fmt.Errorf("freebsd: set MTU on %s: %w", interfaceName, err)
		}
	}
	logger.Info("Configured FreeBSD interface %s with %s (MTU %d)", interfaceName, ipNet.String(), mtu)
	return nil
}

// addFreeBSDAddress is used both for the primary address (configureFreeBSD)
// and for secondary addresses on an existing tunnel interface.
func addFreeBSDAddress(interfaceName string, ip net.IP, ipNet *net.IPNet) error {
	req, err := newIfAliasReq(interfaceName, ip, ipNet)
	if err != nil {
		return err
	}
	return ioctlIfalias(unix.SIOCAIFADDR, req)
}

func removeFreeBSDAddress(interfaceName string, ip net.IP, ipNet *net.IPNet) error {
	req, err := newIfAliasReq(interfaceName, ip, ipNet)
	if err != nil {
		return err
	}
	return ioctlIfalias(unix.SIOCDIFADDR, req)
}

func newIfAliasReq(interfaceName string, ip net.IP, ipNet *net.IPNet) (*ifaliasreq, error) {
	if len(interfaceName) >= unix.IFNAMSIZ {
		return nil, fmt.Errorf("interface name %q is too long", interfaceName)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("freebsd IPv4 backend only supports IPv4 addresses, got %s", ip)
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 32 {
		return nil, fmt.Errorf("invalid IPv4 mask for %s", ipNet.String())
	}

	req := &ifaliasreq{}
	copy(req.Name[:], interfaceName)

	req.Addr = unix.RawSockaddrInet4{Len: unix.SizeofSockaddrInet4, Family: unix.AF_INET}
	copy(req.Addr.Addr[:], ip4)
	// Point-to-point peer address. tun(4) uses the same address on both ends.
	req.BroadAddr = req.Addr

	var mask [4]byte
	binary.BigEndian.PutUint32(mask[:], uint32(0xffffffff)<<uint(32-ones))
	req.Mask = unix.RawSockaddrInet4{Len: unix.SizeofSockaddrInet4, Family: unix.AF_INET}
	copy(req.Mask.Addr[:], mask[:])
	return req, nil
}

func ioctlIfalias(request uint, req *ifaliasreq) error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	if err := ioctlPtr(fd, request, unsafe.Pointer(req)); err != nil {
		return err
	}
	return nil
}

func setFreeBSDFlags(interfaceName string, add int16) error {
	req, err := newIfreq(interfaceName)
	if err != nil {
		return err
	}
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	if err := ioctlPtr(fd, unix.SIOCGIFFLAGS, unsafe.Pointer(&req[0])); err != nil {
		return err
	}
	flags := *(*int16)(unsafe.Pointer(&req[16]))
	*(*int16)(unsafe.Pointer(&req[16])) = flags | add
	return ioctlPtr(fd, unix.SIOCSIFFLAGS, unsafe.Pointer(&req[0]))
}

func setFreeBSDMTU(interfaceName string, mtu int) error {
	req, err := newIfreq(interfaceName)
	if err != nil {
		return err
	}
	*(*int32)(unsafe.Pointer(&req[16])) = int32(mtu)

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	return ioctlPtr(fd, unix.SIOCSIFMTU, unsafe.Pointer(&req[0]))
}

func newIfreq(interfaceName string) (ifreq, error) {
	var req ifreq
	if len(interfaceName) >= unix.IFNAMSIZ {
		return req, fmt.Errorf("interface name %q is too long", interfaceName)
	}
	copy(req[:unix.IFNAMSIZ], interfaceName)
	return req, nil
}

func ioctlPtr(fd int, request uint, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(request), uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}
