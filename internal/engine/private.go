package engine

import (
	"fmt"
	"granger/internal/config"
	"granger/pkg/netfilter"
	"strings"
)

func writePrivateAccess(body *strings.Builder, cfg config.Config, client config.Client, chain string, match []string) {
	if chain == netfilter.UserOutputChain {
		body.WriteString(accessRule(chain, append(append([]string{}, match...), "-m", "addrtype", "--dst-type", "LOCAL", "-j", "REJECT")...))
	}

	if cfg.ClientEnabled(client) {
		for _, name := range sortedUpstreamNames(cfg) {
			up := cfg.Upstreams[name]
			if up.IsEnabled() == false || up.AllowPrivate == false || cfg.UserCanUseUpstream(client.User, name) == false {
				continue
			}
			iface := up.Interface
			if up.Type == "direct" && (iface == "" || iface == "auto") {
				iface = cfg.Server.UplinkIF
			}
			for _, subnet := range config.PrivateIPv4Networks() {
				if chain == netfilter.UserOutputChain {
					args := append(append([]string{}, match...), "-m", "mark", "--mark", fmt.Sprintf("0x%x/0xffff", netfilter.RouteTableID(name)), "-d", subnet, "-j", "RETURN")
					body.WriteString(accessRule(chain, args...))
					if cfg.PersonalDefault(client.User) == name && iface != "" && iface != "auto" {
						args = append(append([]string{}, match...), "-m", "mark", "--mark", "0/0xffff", "-o", iface, "-d", subnet, "-j", "RETURN")
						body.WriteString(accessRule(chain, args...))
					}
					continue
				}
				if iface == "" || iface == "auto" {
					continue
				}
				args := append(append([]string{}, match...), "-o", iface, "-d", subnet, "-j", "RETURN")
				body.WriteString(accessRule(chain, args...))
			}
		}
	}
	for _, subnet := range config.PrivateIPv4Networks() {
		args := append(append([]string{}, match...), "-d", subnet, "-j", "REJECT")
		body.WriteString(accessRule(chain, args...))
	}
}
