package engine

import (
	"fmt"
	"net"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/runner"
)

func renderHostControlAccess(cfg config.Config) string {
	if cfg.Server.HostFirewall == nil {
		return ""
	}
	fw := cfg.Server.HostFirewall
	target := fw.Family + " " + fw.Table + " " + fw.AccessChain
	var body strings.Builder
	body.WriteString("flush chain " + target + "\n")
	for _, name := range sortedOutputNames(cfg) {
		out := cfg.Outputs[name]
		if out.IsEnabled() == false || out.Interface == "" || out.ServerIP == "" {
			continue
		}
		destination := net.ParseIP(out.ServerIP)
		if destination == nil || destination.To4() == nil {
			continue
		}
		for _, client := range out.Clients {
			if cfg.ClientEnabled(client) == false || client.IP == "" {
				continue
			}
			if _, exists := cfg.Users[client.User]; exists == false && client.User != "" {
				continue
			}
			source, _, _ := net.ParseCIDR(client.IP)
			if source == nil {
				source = net.ParseIP(client.IP)
			}
			if source == nil || source.To4() == nil {
				continue
			}
			if hasSNIRules(cfg) && routedSNIOutput(out) {
				settings := cfg.Server.DomainRouterSettings()
				for _, transport := range []struct {
					protocol string
					ports    []int
				}{{"tcp", settings.TCPPorts}, {"udp", settings.UDPPorts}} {
					if len(transport.ports) == 0 {
						continue
					}
					fmt.Fprintf(&body, "add rule %s iifname %q ip saddr %s meta mark & 0x80000000 == 0x80000000 %s dport { %s } accept\n", target, out.Interface, source.String(), transport.protocol, config.PortList(transport.ports, ", "))
				}
			}

			for _, protocol := range []string{"tcp", "udp"} {
				fmt.Fprintf(&body, "add rule %s iifname %q ip saddr %s ip daddr %s %s dport 53 accept\n", target, out.Interface, source.String(), destination.String(), protocol)
			}
			for _, address := range cfg.GUIVPNAddresses() {
				fmt.Fprintf(&body, "add rule %s iifname %q ip saddr %s ip daddr %s tcp dport %d accept\n", target, out.Interface, source.String(), address, cfg.Server.GUIVPNPort)
			}
		}
	}
	return body.String()
}

func (e Engine) applyHostControlAccess(cfg config.Config) runner.Result {
	if cfg.Server.HostFirewall == nil {
		return runner.Result{Title: "Host firewall integration", OK: true, Status: "disabled", Output: "No host firewall adapter configured"}
	}
	fw := cfg.Server.HostFirewall
	table := fw.Family + " " + fw.Table
	input := table + " " + fw.InputChain
	access := table + " " + fw.AccessChain
	script := "set -e\nnft list chain " + input + " >/dev/null\n" +
		"nft list chain " + access + " >/dev/null 2>&1 || nft add chain " + access + "\n" +
		"nft list chain " + input + " | grep -q 'jump " + fw.AccessChain + "' || nft insert rule " + input + " jump " + fw.AccessChain + "\nnft -f -"
	return e.Runner.Run("Update host control access", 15*time.Second, strings.NewReader(renderHostControlAccess(cfg)), "sh", "-c", script)
}

func (e Engine) RemoveHostControlAccess(cfg config.Config) []runner.Result {
	fw := cfg.Server.HostFirewall
	if fw == nil {
		return nil
	}
	table := fw.Family + " " + fw.Table
	script := "set -e\nif nft list chain " + table + " " + fw.InputChain + " >/dev/null 2>&1; then\n" +
		"for handle in $(nft -a list chain " + table + " " + fw.InputChain + " | awk '/jump " + fw.AccessChain + " / {print $NF}'); do nft delete rule " + table + " " + fw.InputChain + " handle \"$handle\"; done\nfi\n" +
		"if nft list chain " + table + " " + fw.AccessChain + " >/dev/null 2>&1; then nft flush chain " + table + " " + fw.AccessChain + "; nft delete chain " + table + " " + fw.AccessChain + "; fi"
	return []runner.Result{e.Runner.Run("Remove host firewall integration", 15*time.Second, nil, "sh", "-c", script)}
}
