package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

func sniFixture() config.Config {
	return config.Config{
		Server:    config.Server{UplinkIF: "eth0"},
		Users:     map[string]config.User{"allowed": {AccessRestricted: true, AllowedUpstreams: []string{"direct", "de"}, DefaultUpstream: "de"}, "limited": {AccessRestricted: true, AllowedUpstreams: []string{"de"}, DefaultUpstream: "de"}, "closed": {AccessRestricted: true}},
		Outputs:   map[string]config.Output{"vpn": {Managed: true, Type: "amneziawg", Interface: "awg-test", Clients: []config.Client{{Name: "one", User: "allowed", IP: "10.19.84.2/32"}, {Name: "two", User: "limited", IP: "10.19.84.3/32"}, {Name: "three", User: "closed", IP: "10.19.84.4/32"}, {Name: "four", User: "allowed", IP: "10.19.84.5/32", Disabled: true}}}},
		Upstreams: map[string]config.Upstream{"direct": {Type: "direct", Interface: "eth0"}, "de": {Type: "wireguard", Interface: "wg-up0"}},
		Rules:     []config.Rule{{Name: "ru", Via: "direct", Domains: []string{"ozon.ru", "*.gov.ru"}, DomainMatch: "sni"}, {Name: "default", Via: "de", Default: true}},
	}
}

func TestSNIUsesDomainAndPerUserFallbackWithoutDNSIPExpansion(t *testing.T) {
	cfg := sniFixture()
	body, err := renderSNI(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Routing struct {
			DomainStrategy string
			Rules          []struct {
				Source      []string
				Domain      []string
				OutboundTag string
			}
		}
		Inbounds []struct {
			Listen   string
			Sniffing struct{ RouteOnly bool }
		}
		Outbounds []struct {
			Tag            string
			StreamSettings struct{ Sockopt struct{ Mark int } }
		}
	}
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	if value.Routing.DomainStrategy != "AsIs" || value.Inbounds[0].Sniffing.RouteOnly == false || value.Inbounds[0].Listen != "127.0.0.1" {
		t.Fatal("SNI must select a route without changing the destination or exposing its listener")
	}
	seen := map[string][]string{}
	for _, rule := range value.Routing.Rules {
		seen[rule.Source[0]] = append(seen[rule.Source[0]], rule.OutboundTag)
	}
	if strings.Join(seen["10.19.84.2/32"], ",") != "vpn:one:direct,vpn:one:de" {
		t.Fatal(seen)
	}
	if strings.Join(seen["10.19.84.3/32"], ",") != "vpn:two:de" || strings.Join(seen["10.19.84.4/32"], ",") != "blocked" {
		t.Fatal(seen)
	}
	if len(seen["10.19.84.5/32"]) != 0 {
		t.Fatal("revoked client included")
	}
	for _, out := range value.Outbounds {
		if out.Tag == "vpn:one:direct" && out.StreamSettings.Sockopt.Mark != netfilter.UserMark("vpn", "one")|netfilter.RouteTableID("direct") {
			t.Fatal("outbound lost user or route bits")
		}
	}
	kernel := kernelDomainRules(cfg)
	if len(kernel.Rules[0].Domains) != 0 || len(cfg.Rules[0].Domains) != 2 {
		t.Fatal("SNI domains leaked into kernel IP matching or mutated the source config")
	}
}

func TestSNICapturePreservesClassifiedTrafficAndLimitsSources(t *testing.T) {
	cfg := sniFixture()
	e := New(runner.NewDryRun())
	res := e.applySNICapture(cfg)
	if len(res) != 1 {
		t.Fatal(res)
	}
	body := res[0].Command
	for _, part := range []string{"--on-ip", "127.0.0.1", "0/0xffff", "10.19.84.2/32", "--dst-type", "LOCAL", "TPROXY"} {
		if strings.Contains(body, part) == false {
			t.Fatalf("missing %s", part)
		}
	}
	if strings.Contains(body, "10.19.84.5/32") {
		t.Fatal("revoked client intercepted")
	}
	var policy strings.Builder
	out := cfg.Outputs["vpn"]
	writeSNIOutputAccess(&policy, cfg, "vpn", out, out.Clients[1])
	if strings.Contains(policy.String(), "eth0") || strings.Contains(policy.String(), "wg-up0") == false {
		t.Fatal(policy.String())
	}
}
