package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"granger/internal/classifier"
	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

const sniCaptureMark = "0x80000000/0x80000000"

func hasSNIRules(cfg config.Config) bool {
	for _, rule := range cfg.Rules {
		if rule.DomainMatch == "sni" {
			return true
		}
	}
	return false
}

func kernelDomainRules(cfg config.Config) config.Config {
	cfg.Rules = append([]config.Rule(nil), cfg.Rules...)
	for i := range cfg.Rules {
		if cfg.Rules[i].DomainMatch == "sni" {
			cfg.Rules[i].Domains = nil
		}
	}
	return cfg
}

func routedSNIOutput(out config.Output) bool {
	return out.IsEnabled() && out.Interface != "" && (out.Type == "wireguard" || out.Type == "amneziawg" || out.Type == "openvpn")
}

func renderSNI(cfg config.Config) ([]byte, error) {
	return (classifier.Xray{}).Render(cfg)
}

func (e Engine) prepareSNI(cfg config.Config) []runner.Result {
	backend, err := e.Classifiers.Find(cfg.Server.DomainRouterSettings().Backend)
	if err != nil {
		return []runner.Result{fail("Domain router backend", err)}
	}
	return backend.Prepare(cfg)
}

func (e Engine) applySNICapture(cfg config.Config) []runner.Result {
	settings := cfg.Server.DomainRouterSettings()
	sniTable := strconv.Itoa(settings.Table)
	sniPort := strconv.Itoa(settings.Port)
	if classifier.NeedsTransparent(cfg) == false {
		return []runner.Result{e.Runner.Run("Remove unused SNI routing", 10*time.Second, nil, "sh", "-c", "while ip rule del fwmark "+sniCaptureMark+" table "+sniTable+" priority 100 2>/dev/null; do :; done")}
	}
	var script strings.Builder
	script.WriteString("set -e\nip route replace local 0.0.0.0/0 dev lo table " + sniTable + "\n")
	script.WriteString(netfilter.EnsureIPRule("fwmark "+sniCaptureMark, sniTable, "100") + "\n")
	script.WriteString(netfilter.EnsureAppend("mangle", netfilter.MangleChain, "-m", "addrtype", "--dst-type", "LOCAL", "-j", "RETURN") + "\n")
	for _, name := range sortedOutputNames(cfg) {
		out := cfg.Outputs[name]
		if routedSNIOutput(out) == false {
			continue
		}
		for _, client := range cfg.EnabledClients(out) {
			if client.IP == "" {
				continue
			}
			for _, proto := range []string{"tcp", "udp"} {
				ports := config.PortList(settings.TCPPorts, ",")
				if proto == "udp" {
					ports = config.PortList(settings.UDPPorts, ",")
				}
				if ports == "" {
					continue
				}
				script.WriteString(netfilter.EnsureAppend("mangle", netfilter.MangleChain, "-i", out.Interface, "-s", client.IP, "-m", "mark", "--mark", "0/0xffff", "-p", proto, "-m", "multiport", "--dports", ports, "-j", "TPROXY", "--on-ip", "127.0.0.1", "--on-port", sniPort, "--tproxy-mark", sniCaptureMark) + "\n")
			}
		}
	}
	return []runner.Result{e.Runner.Run("Route web traffic by SNI", 20*time.Second, nil, "sh", "-c", script.String())}
}

func writeSNIOutputAccess(body *strings.Builder, cfg config.Config, name string, out config.Output, client config.Client) {
	match := []string{"-m", "mark", "--mark", fmt.Sprintf("0x%x/0xffff0000", netfilter.UserMark(name, client.Name))}
	writePrivateAccess(body, cfg, client, netfilter.UserOutputChain, match)
	if cfg.ClientEnabled(client) {
		for _, upstreamName := range sortedUpstreamNames(cfg) {
			up := cfg.Upstreams[upstreamName]
			if up.IsEnabled() == false || cfg.UserCanUseUpstream(client.User, upstreamName) == false {
				continue
			}
			iface := up.Interface
			if up.Type == "direct" && (iface == "" || iface == "auto") {
				iface = cfg.Server.UplinkIF
			}
			if iface == "" || iface == "auto" {
				continue
			}
			body.WriteString(accessRule(netfilter.UserOutputChain, append(append([]string{}, match...), "-o", iface, "-j", "RETURN")...))
		}
	}
	body.WriteString(accessRule(netfilter.UserOutputChain, append(match, "-j", "REJECT")...))
}

func (e Engine) RemoveDomainRouterCapture(cfg config.Config) []runner.Result {
	table := strconv.Itoa(cfg.Server.DomainRouterSettings().Table)
	script := "while ip rule del fwmark " + sniCaptureMark + " table " + table + " priority 100 2>/dev/null; do :; done\nip route flush table " + table + " 2>/dev/null || true"
	return []runner.Result{e.Runner.Run("Remove domain router capture", 10*time.Second, nil, "sh", "-c", script)}
}
