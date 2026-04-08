package osutil

import (
	"net"
	"os"
	"runtime"
)

// Hostname returns the current machine hostname.
func Hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}

// OS returns the current GOOS value.
func OS() string {
	return runtime.GOOS
}

// Arch returns the current GOARCH value.
func Arch() string {
	return runtime.GOARCH
}

// PrivateIPv4 returns the first non-loopback private IPv4 address if present.
func PrivateIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil {
			continue
		}
		return ip.String()
	}
	return ""
}
