package utils

import (
	"net"
	"os"
	"runtime"
)

// GetLocalIP 返回第一张非回环 up 网卡的 IPv4 地址；找不到返回空串。
func GetLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if v4 := ipnet.IP.To4(); v4 != nil {
					return v4.String()
				}
			}
		}
	}
	return ""
}

// GetLocalHost 返回主机名。
func GetLocalHost() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}

// GetLocalOS 返回操作系统类型（linux / darwin / windows 等）。
func GetLocalOS() string {
	return runtime.GOOS
}

// ListNetworkInterfaceNames 列出所有网卡名称。
func ListNetworkInterfaceNames() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		names = append(names, iface.Name)
	}
	return names
}
