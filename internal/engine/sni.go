package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"granger/internal/config"
	"granger/internal/driver"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

const sniCaptureMark = "0x80000000/0x80000000"
const sniService = "granger-sni.service"
const sniConfigPath = config.RuntimeDir + "/sni.json"
const sniServicePath = "/etc/systemd/system/" + sniService
const sniPort = "19845"
const sniTable = "19845"

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

func sniDomain(value string) string {
	value = strings.TrimPrefix(value, "*.")
	if strings.HasPrefix(value, "full:") || strings.HasPrefix(value, "domain:") || strings.HasPrefix(value, "regexp:") {
		return value
	}
	return "domain:" + value
}

func renderSNI(cfg config.Config) ([]byte, error) {
	outbounds := []any{map[string]any{"tag": "blocked", "protocol": "blackhole"}}
	var rules []any
	for _, name := range sortedOutputNames(cfg) {
		out := cfg.Outputs[name]
		if routedSNIOutput(out) == false {
			continue
		}
		for _, client := range cfg.EnabledClients(out) {
			if client.IP == "" {
				continue
			}
			tags := map[string]string{}
			for _, upstreamName := range sortedUpstreamNames(cfg) {
				up := cfg.Upstreams[upstreamName]
				if up.IsEnabled() == false || cfg.UserCanUseUpstream(client.User, upstreamName) == false {
					continue
				}
				tag := name + ":" + client.Name + ":" + upstreamName
				tags[upstreamName] = tag
				mark := netfilter.UserMark(name, client.Name) | driver.RouteTableID(upstreamName)
				outbounds = append(outbounds, map[string]any{"tag": tag, "protocol": "freedom", "settings": map[string]any{}, "streamSettings": map[string]any{"sockopt": map[string]any{"mark": mark, "domainStrategy": "UseIPv4"}}})
			}
			for _, rule := range cfg.Rules {
				tag := tags[rule.Via]
				if rule.DomainMatch != "sni" || tag == "" || len(rule.Domains) == 0 {
					continue
				}
				domains := make([]string, 0, len(rule.Domains))
				for _, domain := range rule.Domains {
					domains = append(domains, sniDomain(domain))
				}
				rules = append(rules, map[string]any{"type": "field", "source": []string{client.IP}, "domain": domains, "outboundTag": tag})
			}
			tag := tags[personalDefault(cfg, client.User)]
			if tag == "" {
				tag = "blocked"
			}
			rules = append(rules, map[string]any{"type": "field", "source": []string{client.IP}, "outboundTag": tag})
		}
	}
	body := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  []any{map[string]any{"listen": "127.0.0.1", "port": 19845, "protocol": "dokodemo-door", "settings": map[string]any{"network": "tcp,udp", "followRedirect": true}, "streamSettings": map[string]any{"sockopt": map[string]any{"tproxy": "tproxy"}}, "sniffing": map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}, "routeOnly": true}}},
		"outbounds": outbounds,
		"routing":   map[string]any{"domainStrategy": "AsIs", "rules": rules},
	}
	return json.MarshalIndent(body, "", "  ")
}

func (e Engine) prepareSNI(cfg config.Config) []runner.Result {
	if hasSNIRules(cfg) == false {
		if _, err := os.Stat(sniServicePath); err == nil && e.Runner.DryRun == false {
			return []runner.Result{e.Runner.Run("Stop unused SNI router", 20*time.Second, nil, "systemctl", "disable", "--now", sniService)}
		}
		return nil
	}
	body, err := renderSNI(cfg)
	if err != nil {
		return []runner.Result{fail("Render SNI routing", err)}
	}
	if e.Runner.DryRun {
		return []runner.Result{{Title: "Prepare SNI routing", Command: "xray run -test", Output: string(body), OK: true, Status: "dry-run"}}
	}
	binary, err := exec.LookPath("xray")
	if err != nil {
		return []runner.Result{fail("Prepare SNI routing", fmt.Errorf("install the Xray driver before enabling SNI routing"))}
	}
	if err := os.MkdirAll(config.RuntimeDir, 0700); err != nil {
		return []runner.Result{fail("Prepare SNI routing", err)}
	}
	previous, _ := os.ReadFile(sniConfigPath)
	changed := string(previous) != string(body)
	temporary, err := os.CreateTemp(config.RuntimeDir, ".sni-*.json")
	if err != nil {
		return []runner.Result{fail("Prepare SNI routing", err)}
	}
	path := temporary.Name()
	defer os.Remove(path)
	if _, err := temporary.Write(body); err != nil {
		temporary.Close()
		return []runner.Result{fail("Write SNI routing", err)}
	}
	if err := temporary.Close(); err != nil {
		return []runner.Result{fail("Write SNI routing", err)}
	}
	results := []runner.Result{e.Runner.Run("Validate SNI routing", 10*time.Second, nil, binary, "run", "-test", "-config", path)}
	if resultsOK(results) == false {
		return results
	}
	if err := os.Rename(path, sniConfigPath); err != nil {
		return append(results, fail("Write SNI routing", err))
	}
	unit := "[Unit]\nDescription=Granger domain routing\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=simple\nExecStart=" + strconv.Quote(binary) + " run -config " + sniConfigPath + "\nRestart=on-failure\nRestartSec=2\n\n[Install]\nWantedBy=multi-user.target\n"
	oldUnit, _ := os.ReadFile(sniServicePath)
	if string(oldUnit) != unit {
		if err := os.WriteFile(sniServicePath, []byte(unit), 0644); err != nil {
			return append(results, fail("Write SNI service", err))
		}
		changed = true
		results = append(results, e.Runner.Run("Reload SNI service", 10*time.Second, nil, "systemctl", "daemon-reload"))
	}
	results = append(results, e.Runner.Run("Enable SNI router", 10*time.Second, nil, "systemctl", "enable", sniService))
	action := "start"
	if changed {
		action = "restart"
	}
	return append(results, e.Runner.Run("Start SNI router", 20*time.Second, nil, "systemctl", action, sniService))
}

func (e Engine) applySNICapture(cfg config.Config) []runner.Result {
	if hasSNIRules(cfg) == false {
		return []runner.Result{e.Runner.Run("Remove unused SNI routing", 10*time.Second, nil, "sh", "-c", "while ip rule del fwmark "+sniCaptureMark+" table "+sniTable+" priority 100 2>/dev/null; do :; done")}
	}
	var script strings.Builder
	script.WriteString("set -e\nip route replace local 0.0.0.0/0 dev lo table " + sniTable + "\n")
	script.WriteString("ip rule show | grep -q 'fwmark 0x80000000/0x80000000.*lookup " + sniTable + "' || ip rule add fwmark " + sniCaptureMark + " table " + sniTable + " priority 100\n")
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
				ports := "80,443"
				if proto == "udp" {
					ports = "443"
				}
				script.WriteString(netfilter.EnsureAppend("mangle", netfilter.MangleChain, "-i", out.Interface, "-s", client.IP, "-m", "mark", "--mark", "0/0xffff", "-p", proto, "-m", "multiport", "--dports", ports, "-j", "TPROXY", "--on-ip", "127.0.0.1", "--on-port", sniPort, "--tproxy-mark", sniCaptureMark) + "\n")
			}
		}
	}
	return []runner.Result{e.Runner.Run("Route web traffic by SNI", 20*time.Second, nil, "sh", "-c", script.String())}
}

func writeSNIOutputAccess(body *strings.Builder, cfg config.Config, name string, out config.Output, client config.Client) {
	match := []string{"-m", "mark", "--mark", fmt.Sprintf("0x%x/0xffff0000", netfilter.UserMark(name, client.Name))}
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
