package classifier

import (
	"encoding/json"
	"fmt"
	"granger/internal/config"
	"granger/internal/driver"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Service = "granger-sni.service"
const ConfigPath = config.RuntimeDir + "/sni.json"
const ServicePath = "/etc/systemd/system/" + Service

type Xray struct{ Runner runner.Runner }

func (Xray) ID() string         { return "xray" }
func (Xray) Dependency() string { return "xray" }

func NeedsTransparent(cfg config.Config) bool {
	for _, rule := range cfg.Rules {
		if rule.DomainMatch == "sni" {
			for _, out := range cfg.Outputs {
				if RoutedOutput(out) {
					return true
				}
			}
		}
	}
	return false
}

func RoutedOutput(out config.Output) bool {
	return out.IsEnabled() && out.Interface != "" && (out.Type == "wireguard" || out.Type == "amneziawg" || out.Type == "openvpn")
}

func outputNames(cfg config.Config) []string {
	var ids []string
	for id := range cfg.Outputs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func upstreamNames(cfg config.Config) []string {
	var ids []string
	for id := range cfg.Upstreams {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func fail(title string, err error) runner.Result {
	return runner.Result{Title: title, OK: false, Status: "error", Output: err.Error()}
}
func resultsOK(results []runner.Result) bool {
	for _, result := range results {
		if result.OK == false {
			return false
		}
	}
	return true
}
func sniDomain(value string) string {
	value = strings.TrimPrefix(value, "*.")
	if strings.HasPrefix(value, "full:") || strings.HasPrefix(value, "domain:") || strings.HasPrefix(value, "regexp:") {
		return value
	}
	return "domain:" + value
}

func (b Xray) Render(cfg config.Config) ([]byte, error) {
	outbounds := []any{map[string]any{"tag": "blocked", "protocol": "blackhole"}}
	var rules []any
	for _, name := range outputNames(cfg) {
		out := cfg.Outputs[name]
		if RoutedOutput(out) == false {
			continue
		}
		for _, client := range cfg.EnabledClients(out) {
			if client.IP == "" {
				continue
			}
			tags := map[string]string{}
			for _, upstreamName := range upstreamNames(cfg) {
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
			tag := tags[cfg.PersonalDefault(client.User)]
			if tag == "" {
				tag = "blocked"
			}
			rules = append(rules, map[string]any{"type": "field", "source": []string{client.IP}, "outboundTag": tag})
		}
	}
	body := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  []any{map[string]any{"listen": "127.0.0.1", "port": cfg.Server.DomainRouterSettings().Port, "protocol": "dokodemo-door", "settings": map[string]any{"network": "tcp,udp", "followRedirect": true}, "streamSettings": map[string]any{"sockopt": map[string]any{"tproxy": "tproxy"}}, "sniffing": map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}, "routeOnly": true}}},
		"outbounds": outbounds,
		"routing":   map[string]any{"domainStrategy": "AsIs", "rules": rules},
	}
	return json.MarshalIndent(body, "", "  ")
}

func (b Xray) Prepare(cfg config.Config) []runner.Result {
	if NeedsTransparent(cfg) == false {
		if _, err := os.Stat(ServicePath); err == nil && b.Runner.DryRun == false {
			return []runner.Result{b.Runner.Run("Stop unused SNI router", 20*time.Second, nil, "systemctl", "disable", "--now", Service)}
		}
		return nil
	}
	body, err := b.Render(cfg)
	if err != nil {
		return []runner.Result{fail("Render SNI routing", err)}
	}
	if b.Runner.DryRun {
		return []runner.Result{{Title: "Prepare SNI routing", Command: "xray run -test", Output: string(body), OK: true, Status: "dry-run"}}
	}
	binary, err := exec.LookPath("xray")
	if err != nil {
		return []runner.Result{fail("Prepare SNI routing", fmt.Errorf("install the Xray driver before enabling SNI routing"))}
	}
	if err := os.MkdirAll(config.RuntimeDir, 0700); err != nil {
		return []runner.Result{fail("Prepare SNI routing", err)}
	}
	previous, _ := os.ReadFile(ConfigPath)
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
	results := []runner.Result{b.Runner.Run("Validate SNI routing", 10*time.Second, nil, binary, "run", "-test", "-config", path)}
	if resultsOK(results) == false {
		return results
	}
	if err := os.Rename(path, ConfigPath); err != nil {
		return append(results, fail("Write SNI routing", err))
	}
	unit := "[Unit]\nDescription=Granger domain routing\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=simple\nExecStart=" + strconv.Quote(binary) + " run -config " + ConfigPath + "\nRestart=on-failure\nRestartSec=2\n\n[Install]\nWantedBy=multi-user.target\n"
	oldUnit, _ := os.ReadFile(ServicePath)
	if string(oldUnit) != unit {
		if err := os.WriteFile(ServicePath, []byte(unit), 0644); err != nil {
			return append(results, fail("Write SNI service", err))
		}
		changed = true
		results = append(results, b.Runner.Run("Reload SNI service", 10*time.Second, nil, "systemctl", "daemon-reload"))
	}
	results = append(results, b.Runner.Run("Enable SNI router", 10*time.Second, nil, "systemctl", "enable", Service))
	action := "start"
	if changed {
		action = "restart"
	}
	return append(results, b.Runner.Run("Start SNI router", 20*time.Second, nil, "systemctl", action, Service))
}
