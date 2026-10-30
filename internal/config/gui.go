package config

import (
	"net"
	"sort"
)

func (c Config) GUIVPNAddresses() []string {
	if c.Server.GUIVPNPort == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, out := range c.Outputs {
		if out.IsEnabled() == false || out.Interface == "" {
			continue
		}
		ip := net.ParseIP(out.ServerIP)
		if ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}
		seen[ip.String()] = true
	}
	addresses := make([]string, 0, len(seen))
	for address := range seen {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	return addresses
}
