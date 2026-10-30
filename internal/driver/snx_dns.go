package driver

import (
	"net"
	"strings"
	"time"

	"granger/internal/config"
)

func snxDNS(up config.Upstream, ctx ApplyContext) []string {
	if len(up.DNS) > 0 {
		return up.DNS
	}
	result := ctx.Runner.RunSoft("Read SNX-RS DNS", 8*time.Second, nil, "env", "LC_ALL=C", "LANG=C", "snxctl", "--config-file", up.Config, "status")
	if result.OK == false {
		return nil
	}
	return parseSNXDNS(result.Output)
}

func parseSNXDNS(output string) []string {
	var servers []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		_, value, found := strings.Cut(line, "DNS servers:")
		if found == false {
			continue
		}
		for _, field := range strings.FieldsFunc(value, func(r rune) bool {
			return r == '[' || r == ']' || r == ',' || r == ' ' || r == '\t'
		}) {
			ip := net.ParseIP(field)
			if ip == nil || ip.To4() == nil {
				continue
			}
			server := ip.String()
			if seen[server] {
				continue
			}
			seen[server] = true
			servers = append(servers, server)
		}
	}
	return servers
}
