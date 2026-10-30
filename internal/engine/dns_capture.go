package engine

import (
	"fmt"
	"net"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

func renderDNSCapture(cfg config.Config) string {
	var body strings.Builder
	fmt.Fprintf(&body, "*nat\n:%s - [0:0]\n-F %s\n", netfilter.DNSChain, netfilter.DNSChain)
	for _, name := range sortedOutputNames(cfg) {
		out := cfg.Outputs[name]
		if out.Managed == false || out.IsEnabled() == false || out.Interface == "" {
			continue
		}
		destination := net.ParseIP(out.ServerIP)
		if destination == nil || destination.To4() == nil {
			continue
		}
		for _, client := range cfg.EnabledClients(out) {
			source, _, _ := net.ParseCIDR(client.IP)
			if source == nil || source.To4() == nil {
				continue
			}
			for _, protocol := range []string{"udp", "tcp"} {
				body.WriteString(accessRule(netfilter.DNSChain, "-i", out.Interface, "-s", source.String()+"/32", "-p", protocol, "--dport", "53", "-j", "DNAT", "--to-destination", destination.String()+":53"))
			}
		}
	}
	body.WriteString("COMMIT\n")
	return body.String()
}

func (e Engine) applyDNSCapture(cfg config.Config) runner.Result {
	body := renderDNSCapture(cfg)
	if e.Runner.DryRun {
		return runner.Result{Title: "Route VPN DNS through Granger", Command: "iptables-restore --noflush --wait 5", Output: body, OK: true, Status: "dry-run"}
	}
	return e.Runner.Run("Route VPN DNS through Granger", 15*time.Second, strings.NewReader(body), "iptables-restore", "--noflush", "--wait", "5")
}
