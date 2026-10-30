package engine

import (
	"fmt"
	"granger/internal/config"
	"granger/pkg/netfilter"
	"strings"
)

func renderEgressAccess(cfg config.Config) string {
	var body strings.Builder
	fmt.Fprintf(&body, "*mangle\n:%s - [0:0]\n-F %s\n", netfilter.UserEgressChain, netfilter.UserEgressChain)
	for _, name := range sortedOutputNames(cfg) {
		out := cfg.Outputs[name]
		proxy := out.Managed && (out.Type == "xray" || out.Type == "sing-box")
		if out.IsEnabled() == false || (proxy == false && (hasSNIRules(cfg) == false || routedSNIOutput(out) == false)) {
			continue
		}
		for _, client := range out.Clients {
			match := []string{"-m", "mark", "--mark", fmt.Sprintf("0x%x/0xffff0000", netfilter.UserMark(name, client.Name))}
			if cfg.ClientEnabled(client) {
				for _, target := range sortedUpstreamNames(cfg) {
					up := cfg.Upstreams[target]
					if up.IsEnabled() == false || cfg.UserCanUseUpstream(client.User, target) == false {
						continue
					}
					iface := up.Interface
					if up.Type == "direct" && (iface == "" || iface == "auto") {
						iface = cfg.Server.UplinkIF
					}
					if iface == "" || iface == "auto" {
						continue
					}
					scoped := append(append([]string{}, match...), "-o", iface)
					if up.AllowPrivate == false {
						for _, subnet := range config.PrivateIPv4Networks() {
							body.WriteString(accessRule(netfilter.UserEgressChain, append(append([]string{}, scoped...), "-d", subnet, "-j", "DROP")...))
						}
					}
					body.WriteString(accessRule(netfilter.UserEgressChain, append(scoped, "-j", "RETURN")...))
				}
			}
			body.WriteString(accessRule(netfilter.UserEgressChain, append(match, "-j", "DROP")...))
		}
	}
	body.WriteString(accessRule(netfilter.UserEgressChain, "-m", "mark", "--mark", "0x40000000/0x40000000", "-j", "DROP"))
	body.WriteString("COMMIT\n")
	return body.String()
}
