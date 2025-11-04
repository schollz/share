package client

import (
	"net"
)

// GetLocalIPAddresses returns all non-loopback local IP addresses
func GetLocalIPAddresses() ([]string, error) {
	var localIPs []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip loopback IPs and IPv6 addresses (for simplicity)
			if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
				localIPs = append(localIPs, ip.String())
			}
		}
	}

	return localIPs, nil
}
