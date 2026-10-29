package engine

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"granger/internal/config"
	"granger/internal/driver"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

func accessRule(chain string, args ...string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, strconv.Quote(arg))
	}
	return "-A " + chain + " " + strings.Join(quoted, " ") + "\n"
}

func (e Engine) applyUserAccess(cfg config.Config) []runner.Result {
	var body strings.Builder
	fmt.Fprintf(&body, "*filter\n:%s - [0:0]\n:%s - [0:0]\n:%s - [0:0]\n-F %s\n-F %s\n-F %s\n", netfilter.UserForwardChain, netfilter.UserOutputChain, netfilter.UserInputChain, netfilter.UserForwardChain, netfilter.UserOutputChain, netfilter.UserInputChain)
	for _, outputName := range sortedOutputNames(cfg) {
		out := cfg.Outputs[outputName]
		if out.IsEnabled() == false {
			writeInterfaceDeny(&body, out.Interface)
			continue
		}
		for _, client := range out.Clients {
			user, exists := cfg.Users[client.User]
			proxy := out.Managed && (out.Type == "xray" || out.Type == "sing-box")
			match := []string{"-i", out.Interface, "-s", client.IP}
			chain := netfilter.UserForwardChain
			if proxy {
				chain = netfilter.UserOutputChain
				match = []string{"-m", "mark", "--mark", fmt.Sprintf("0x%x/0xffff0000", netfilter.UserMark(outputName, client.Name))}
			}
			if proxy == false && (out.Interface == "" || client.IP == "") {
				continue
			}
			if proxy == false {
				writeClientInput(&body, cfg, out, client)
			}
			if cfg.ClientEnabled(client) && user.AccessRestricted == false && (exists || client.User == "") {
				body.WriteString(accessRule(chain, append(append([]string{}, match...), "-j", "RETURN")...))
				if proxy == false {
					body.WriteString(accessRule(chain, "-o", out.Interface, "-d", client.IP, "-j", "RETURN"))
				}
				continue
			}
			if exists && cfg.ClientEnabled(client) {
				if proxy {
					body.WriteString(accessRule(chain, append(append([]string{}, match...), "-m", "addrtype", "--dst-type", "LOCAL", "-j", "REJECT")...))
				}
				for _, upstreamName := range sortedUpstreamNames(cfg) {
					up := cfg.Upstreams[upstreamName]
					if cfg.UserCanUseUpstream(client.User, upstreamName) == false || up.IsEnabled() == false {
						continue
					}
					iface := up.Interface
					if up.Type == "direct" && (iface == "" || iface == "auto") {
						iface = cfg.Server.UplinkIF
					}
					if iface == "" || iface == "auto" {
						continue
					}
					args := append(append([]string{}, match...), "-o", iface, "-j", "RETURN")
					body.WriteString(accessRule(chain, args...))
					if proxy == false {
						body.WriteString(accessRule(chain, "-o", out.Interface, "-d", client.IP, "-i", iface, "-j", "RETURN"))
					}
				}
			}
			body.WriteString(accessRule(chain, append(append([]string{}, match...), "-j", "REJECT")...))
			if proxy == false {
				body.WriteString(accessRule(chain, "-o", out.Interface, "-d", client.IP, "-j", "REJECT"))
			}
		}
		if out.Managed && out.Interface != "" {
			writeInterfaceDeny(&body, out.Interface)
		}
	}
	body.WriteString(accessRule(netfilter.UserOutputChain, "-m", "mark", "--mark", "0x40000000/0x40000000", "-j", "REJECT"))
	body.WriteString("COMMIT\n")
	if e.Runner.DryRun {
		return []runner.Result{{Title: "Apply personal access atomically", Command: "iptables-restore --noflush --wait 5", Output: body.String(), OK: true, Status: "dry-run"}}
	}
	return []runner.Result{e.Runner.Run("Apply personal access atomically", 15*time.Second, strings.NewReader(body.String()), "iptables-restore", "--noflush", "--wait", "5")}
}

func writeInterfaceDeny(body *strings.Builder, iface string) {
	if iface == "" {
		return
	}
	body.WriteString(accessRule(netfilter.UserForwardChain, "-i", iface, "-j", "REJECT"))
	body.WriteString(accessRule(netfilter.UserForwardChain, "-o", iface, "-j", "REJECT"))
	body.WriteString(accessRule(netfilter.UserInputChain, "-i", iface, "-j", "REJECT"))
}

func writeClientInput(body *strings.Builder, cfg config.Config, out config.Output, client config.Client) {
	match := []string{"-i", out.Interface, "-s", client.IP}
	user, exists := cfg.Users[client.User]
	active := cfg.ClientEnabled(client)
	if active && (user.Admin || (user.AccessRestricted == false && (exists || client.User == ""))) {
		body.WriteString(accessRule(netfilter.UserInputChain, append(match, "-j", "RETURN")...))
		return
	}
	if active && exists {
		addresses := []string{out.ServerIP}
		if ip := net.ParseIP(cfg.Server.DNSListen); ip != nil && ip.IsLoopback() == false && cfg.Server.DNSListen != out.ServerIP {
			addresses = append(addresses, cfg.Server.DNSListen)
		}
		for _, address := range addresses {
			dnsMatch := append([]string{}, match...)
			if address != "" {
				dnsMatch = append(dnsMatch, "-d", address)
			}
			for _, protocol := range []string{"udp", "tcp"} {
				body.WriteString(accessRule(netfilter.UserInputChain, append(append([]string{}, dnsMatch...), "-p", protocol, "--dport", "53", "-j", "ACCEPT")...))
			}
		}
	}
	body.WriteString(accessRule(netfilter.UserInputChain, append(match, "-j", "REJECT")...))
}

func (e Engine) applyUserDefaultRoutes(cfg config.Config) []runner.Result {
	var results []runner.Result
	for _, outputName := range sortedOutputNames(cfg) {
		out := cfg.Outputs[outputName]
		if out.IsEnabled() == false {
			continue
		}
		proxy := out.Managed && (out.Type == "xray" || out.Type == "sing-box")
		for _, client := range cfg.EnabledClients(out) {
			target := personalDefault(cfg, client.User)
			table := routingGuardTable
			if target != "" {
				table = fmt.Sprint(driver.RouteTableID(target))
			}
			match := "from " + netfilter.ShellQuote(client.IP)
			if proxy {
				match = fmt.Sprintf("fwmark 0x%x/0xffff0000", netfilter.UserMark(outputName, client.Name))
			}
			if proxy == false && (client.IP == "" || out.Interface == "") {
				continue
			}
			search := strings.ReplaceAll(match, "'", "")
			script := "ip rule show | grep -q " + netfilter.ShellQuote(search+".*lookup "+table) + " || ip rule add " + match + " table " + table + " priority 30000"
			results = append(results, e.Runner.Run("Default route for user "+client.User, 10*time.Second, nil, "sh", "-c", script))
		}
	}
	return results
}
