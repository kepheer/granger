package engine

import (
	"fmt"
	"strings"
	"testing"

	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

func TestForbiddenRuleUsesAllowedPersonalDefault(t *testing.T) {
	cfg := config.Default("192.0.2.1", "eth0", "192.0.2.254")
	cfg.Users = map[string]config.User{"alice": {AccessRestricted: true, AllowedUpstreams: []string{"direct"}, DefaultUpstream: "direct"}}
	cfg.Upstreams["direct"] = config.Upstream{Type: "direct", Interface: "eth0"}
	cfg.Upstreams["corporate"] = config.Upstream{Type: "interface", Interface: "snx0"}
	cfg.Outputs = map[string]config.Output{
		"wg":    {Type: "wireguard", Interface: "wg1", Clients: []config.Client{{Name: "alice", User: "alice", IP: "10.66.0.2/32"}}},
		"proxy": {Type: "xray", Managed: true, Clients: []config.Client{{Name: "alice", User: "alice"}}},
	}
	e := New(runner.NewDryRun())
	results := e.applyPersonalFallback(cfg)
	if len(results) != 2 {
		t.Fatalf("expected kernel and proxy fallback, got %d", len(results))
	}
	for _, result := range results {
		if strings.Contains(result.Command, "--set-xmark") == false || strings.Contains(result.Command, fmt.Sprintf("0x%x/0xffff", netfilter.RouteTableID("direct"))) == false {
			t.Fatalf("personal fallback missing: %s", result.Command)
		}
	}
	user := cfg.Users["alice"]
	user.AllowedUpstreams = nil
	cfg.Users["alice"] = user
	if personalDefault(cfg, "alice") != "" || len(e.applyPersonalFallback(cfg)) != 0 {
		t.Fatal("forbidden default must not be used")
	}
	for _, result := range e.applyUserDefaultRoutes(cfg) {
		if strings.Contains(result.Command, routingGuardTable) == false {
			t.Fatalf("missing closed default: %s", result.Command)
		}
	}
}

func TestRoutingGuardCoversInterfacesAndProxyBeforeRebuild(t *testing.T) {
	cfg := config.Default("192.0.2.1", "eth0", "192.0.2.254")
	cfg.Outputs = map[string]config.Output{"wg": {Type: "wireguard", Interface: "wg1"}}
	plan := New(runner.NewDryRun()).ApplyConfig(cfg)
	if strings.Contains(plan.Results[0].Command, "unreachable default") == false || strings.Contains(plan.Results[0].Command, "iif 'wg1'") == false || strings.Contains(plan.Results[0].Command, "0x40000000/0x40000000") == false {
		t.Fatalf("missing initial guard: %s", plan.Results[0].Command)
	}
	if plan.Results[len(plan.Results)-1].Title != "Release rebuilt routing" {
		t.Fatal("routing must only be released after all apply steps")
	}
}
