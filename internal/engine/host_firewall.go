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
	var body strings.Builder
	body.WriteString("flush chain inet host_security granger_control\n")
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
				fmt.Fprintf(&body, "add rule inet host_security granger_control iifname %q ip saddr %s meta mark & 0x80000000 == 0x80000000 tcp dport { 80, 443 } accept\n", out.Interface, source.String())
				fmt.Fprintf(&body, "add rule inet host_security granger_control iifname %q ip saddr %s meta mark & 0x80000000 == 0x80000000 udp dport 443 accept\n", out.Interface, source.String())
			}
			for _, protocol := range []string{"tcp", "udp"} {
				fmt.Fprintf(&body, "add rule inet host_security granger_control iifname %q ip saddr %s ip daddr %s %s dport 53 accept\n", out.Interface, source.String(), destination.String(), protocol)
			}
			for _, address := range cfg.GUIVPNAddresses() {
				fmt.Fprintf(&body, "add rule inet host_security granger_control iifname %q ip saddr %s ip daddr %s tcp dport %d accept\n", out.Interface, source.String(), address, cfg.Server.GUIVPNPort)
			}
		}
	}
	return body.String()
}

func (e Engine) applyHostControlAccess(cfg config.Config) runner.Result {
	script := `if nft list chain inet host_security input >/dev/null 2>&1; then
 nft list chain inet host_security granger_control >/dev/null 2>&1 || nft add chain inet host_security granger_control
 if nft list chain inet host_security input | grep -q 'jump granger_control'; then
  :
 else
  nft insert rule inet host_security input jump granger_control
 fi
 nft -f -
fi`
	return e.Runner.Run("Update host control access", 15*time.Second, strings.NewReader(renderHostControlAccess(cfg)), "sh", "-c", script)
}
