package engine

import (
	"fmt"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

const routingGuardTable = "2147483000"

func (e Engine) beginRoutingUpdate(cfg config.Config) []runner.Result {
	script := "ip route replace unreachable default table " + routingGuardTable + "\n"
	script += guardRule("fwmark 0x40000000/0x40000000") + "\n"
	seen := map[string]bool{}
	for _, name := range sortedOutputNames(cfg) {
		iface := cfg.Outputs[name].Interface
		if iface == "" || seen[iface] {
			continue
		}
		seen[iface] = true
		script += guardRule("iif "+netfilter.ShellQuote(iface)) + "\n"
	}
	return []runner.Result{e.Runner.Run("Block route fallthrough while rebuilding", 10*time.Second, nil, "sh", "-c", "set -e\n"+script)}
}

func guardRule(match string) string {
	search := strings.ReplaceAll(match, "'", "")
	return "ip rule show | grep -F -- " + netfilter.ShellQuote(search+" lookup "+routingGuardTable) + " | grep -q '^50:' || ip rule add " + match + " table " + routingGuardTable + " priority 50"
}

func (e Engine) finishRoutingUpdate() runner.Result {
	script := "while ip rule del table " + routingGuardTable + " priority 50 2>/dev/null; do :; done"
	return e.Runner.Run("Release rebuilt routing", 10*time.Second, nil, "sh", "-c", script)
}

func personalDefault(cfg config.Config, userID string) string {
	return cfg.PersonalDefault(userID)
}

func (e Engine) applyPersonalFallback(cfg config.Config) []runner.Result {
	var results []runner.Result
	for _, outputName := range sortedOutputNames(cfg) {
		out := cfg.Outputs[outputName]
		if out.IsEnabled() == false {
			continue
		}
		proxy := out.Managed && (out.Type == "xray" || out.Type == "sing-box")
		for _, client := range cfg.EnabledClients(out) {
			target := personalDefault(cfg, client.User)
			if target == "" {
				continue
			}
			targetMark := fmt.Sprintf("0x%x/0xffff", netfilter.RouteTableID(target))
			for _, upstreamName := range sortedUpstreamNames(cfg) {
				if cfg.UserCanUseUpstream(client.User, upstreamName) {
					continue
				}
				chain := netfilter.MangleChain
				match := []string{"-i", out.Interface, "-s", client.IP, "-m", "mark", "--mark", fmt.Sprintf("0x%x/0xffff", netfilter.RouteTableID(upstreamName))}
				if proxy {
					chain = netfilter.OutputChain
					match = []string{"-m", "mark", "--mark", fmt.Sprintf("0x%x", netfilter.UserMark(outputName, client.Name)|netfilter.RouteTableID(upstreamName))}
				}
				if proxy == false && (out.Interface == "" || client.IP == "") {
					continue
				}
				args := append(match, "-j", "MARK", "--set-xmark", targetMark)
				results = append(results, e.Runner.Run("Use personal default for "+client.User+" instead of "+upstreamName, 10*time.Second, nil, "sh", "-c", netfilter.EnsureAppend("mangle", chain, args...)))
			}
		}
	}
	return results
}
