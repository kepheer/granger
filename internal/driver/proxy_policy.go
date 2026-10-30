package driver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"granger/internal/config"
	"granger/pkg/netfilter"
)

func proxySNIPolicy(cfg config.Config, name string, out config.Output) ([]any, []any) {
	var routes, exits []any
	for _, client := range cfg.EnabledClients(out) {
		names := make([]string, 0, len(cfg.Upstreams))
		for id := range cfg.Upstreams {
			names = append(names, id)
		}
		sort.Strings(names)
		for _, upstreamName := range names {
			up := cfg.Upstreams[upstreamName]
			used := false
			for _, rule := range cfg.Rules {
				if rule.DomainMatch == "sni" && rule.Via == upstreamName {
					used = true
				}
			}
			if used == false {
				continue
			}
			if up.IsEnabled() == false || cfg.UserCanUseUpstream(client.User, upstreamName) == false {
				continue
			}
			tag := "granger-sni:" + client.Name + ":" + upstreamName
			mark := netfilter.UserMark(name, client.Name) | netfilter.RouteTableID(upstreamName)
			if out.Type == "xray" {
				exits = append(exits, map[string]any{"tag": tag, "protocol": "freedom", "settings": map[string]any{"domainStrategy": "UseIPv4"}, "streamSettings": map[string]any{"sockopt": map[string]any{"mark": mark}}})
			}
			if out.Type == "sing-box" {
				exits = append(exits, map[string]any{"tag": tag, "type": "direct", "routing_mark": mark})
			}
		}
		for _, rule := range cfg.Rules {
			up, exists := cfg.Upstreams[rule.Via]
			if rule.DomainMatch != "sni" || len(rule.Domains) == 0 || exists == false || up.IsEnabled() == false || cfg.UserCanUseUpstream(client.User, rule.Via) == false {
				continue
			}
			tag := "granger-sni:" + client.Name + ":" + rule.Via
			var domains []string
			for _, domain := range rule.Domains {
				domain = strings.TrimPrefix(domain, "*.")
				if out.Type == "xray" {
					if strings.Contains(domain, ":") == false {
						domain = "domain:" + domain
					}
				}
				domains = append(domains, domain)
			}
			if out.Type == "xray" {
				routes = append(routes, map[string]any{"type": "field", "user": []string{client.Name}, "domain": domains, "outboundTag": tag})
			}
			if out.Type == "sing-box" {
				route := map[string]any{"auth_user": []string{client.Name}, "outbound": tag}
				matchers := map[string][]string{}
				for _, domain := range domains {
					field := "domain_suffix"
					if strings.HasPrefix(domain, "full:") {
						field = "domain"
						domain = strings.TrimPrefix(domain, "full:")
					}
					if strings.HasPrefix(domain, "regexp:") {
						field = "domain_regex"
						domain = strings.TrimPrefix(domain, "regexp:")
					}
					domain = strings.TrimPrefix(domain, "domain:")
					matchers[field] = append(matchers[field], domain)
				}
				for field, values := range matchers {
					route[field] = values
				}
				routes = append(routes, route)
			}
		}
	}
	return routes, exits
}

func updateProxySNIPolicy(cfg config.Config, name string, out config.Output) (bool, error) {
	body, err := os.ReadFile(out.Config)
	if err != nil {
		return false, err
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		return false, err
	}
	routes, exits := proxySNIPolicy(cfg, name, out)
	hasNativeRules := len(routes) > 0
	key := "routing"
	if out.Type == "sing-box" {
		key = "route"
	}
	routing, ok := document[key].(map[string]any)
	if ok == false {
		return false, fmt.Errorf("managed %s config has no routing object", out.Type)
	}
	oldRules, _ := routing["rules"].([]any)
	for _, value := range oldRules {
		rule, _ := value.(map[string]any)
		action, _ := rule["action"].(string)
		if out.Type == "sing-box" && action == "sniff" {
			continue
		}
		tag, _ := rule["outboundTag"].(string)
		if out.Type == "sing-box" {
			tag, _ = rule["outbound"].(string)
		}
		if strings.HasPrefix(tag, "granger-sni:") == false {
			routes = append(routes, value)
		}
	}
	if out.Type == "sing-box" && hasNativeRules {
		routes = append([]any{map[string]any{"inbound": []string{"clients"}, "action": "sniff", "sniffer": []string{"http", "tls", "quic"}}}, routes...)
	}
	routing["rules"] = routes
	oldExits, _ := document["outbounds"].([]any)
	for _, value := range oldExits {
		exit, _ := value.(map[string]any)
		tag, _ := exit["tag"].(string)
		if strings.HasPrefix(tag, "granger-sni:") == false {
			exits = append(exits, value)
		}
	}
	if len(routes) == 0 {
		routing["rules"] = []any{}
	}
	document["outbounds"] = exits
	if out.Type == "xray" {
		inbounds, _ := document["inbounds"].([]any)
		for _, value := range inbounds {
			inbound, _ := value.(map[string]any)
			if inbound != nil {
				inbound["sniffing"] = map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}, "routeOnly": true}
			}
		}
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil || string(encoded) == string(body) {
		return false, err
	}
	file, err := os.CreateTemp(filepath.Dir(out.Config), ".proxy-policy-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(encoded); err != nil {
		file.Close()
		return false, err
	}
	if err := file.Close(); err != nil {
		return false, err
	}
	return true, os.Rename(file.Name(), out.Config)
}
