package utils

import (
	"encoding/binary"
	"errors"
	"github.com/c-robinson/iplib"
	"math/rand"
	"net"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

var (
	errNoAvailableInterface            = errors.New("no available interface")
	errNoAvailableAddress              = errors.New("no available address")
	seededRand              *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func RandomString(length int) string {
	return StringWithCharset(length, charset)
}

func RandomPrivateNet() (iplib.Net, error) {
	ipo, ipnet, err := net.ParseCIDR("10.0.0.0/8")
	var ipne iplib.Net
	if (err == nil) {
		ip := ipo.To4()
		ipb := make(net.IP, net.IPv4len)
		copy(ipb, ip)
		ipn := make(net.IP, net.IPv4len)
		copy(ipn, ip)

		for i, v := range ip {
			ipn[i] = ip[i] & ipnet.Mask[i]
			ipb[i] = v | ^ ipnet.Mask[i]
		}
		ipRaw := make([]byte, 4)
		binary.LittleEndian.PutUint32(ipRaw, seededRand.Uint32())
		ipRaw[3] = 255
		for i, v := range ipRaw {
			ip[i] = ip[i] + (v &^ ipnet.Mask[i])
		}
		_, ipne, err = iplib.ParseCIDR(ip.String() + "/24")
	}
	return ipne, err
}

// RoutedInterface returns a network interface that can route IP
// traffic and satisfies flags.
//
// The provided network must be "IP", "ip4" or "ip6".
func RoutedInterface(network string, flags net.Flags) (*net.Interface, error) {
	switch network {
	case "IP", "ip4", "ip6":
	default:
		return nil, errNoAvailableInterface
	}
	ift, err := net.Interfaces()
	if err != nil {
		return nil, errNoAvailableInterface
	}
	for _, ifi := range ift {
		if ifi.Flags&flags != flags {
			continue
		}
		if _, ok := hasRoutableIP(network, &ifi); !ok {
			continue
		}
		return &ifi, nil
	}
	return nil, errNoAvailableInterface
}

func hasRoutableIP(network string, ifi *net.Interface) (net.IP, bool) {
	ifat, err := ifi.Addrs()
	if err != nil {
		return nil, false
	}
	for _, ifa := range ifat {
		switch ifa := ifa.(type) {
		case *net.IPAddr:
			if ip, ok := routableIP(network, ifa.IP); ok {
				return ip, true
			}
		case *net.IPNet:
			if ip, ok := routableIP(network, ifa.IP); ok {
				return ip, true
			}
		}
	}
	return nil, false
}

func routableIP(network string, ip net.IP) (net.IP, bool) {
	if !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsGlobalUnicast() {
		return nil, false
	}
	switch network {
	case "ip4":
		if ip := ip.To4(); ip != nil {
			return ip, true
		}
	case "ip6":
		if ip.IsLoopback() { // addressing scope of the loopback address depends on each implementation
			return nil, false
		}
		if ip := ip.To16(); ip != nil && ip.To4() == nil {
			return ip, true
		}
	default:
		if ip := ip.To4(); ip != nil {
			return ip, true
		}
		if ip := ip.To16(); ip != nil {
			return ip, true
		}
	}
	return nil, false
}
