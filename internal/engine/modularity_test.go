package engine

import (
	"granger/internal/config"
	"granger/pkg/runner"
	"strings"
	"testing"
)

func TestSNIToSNXPreservesDNSAndRouteWithoutDNSIPClassification(t *testing.T) {
	cfg := config.Config{Outputs: map[string]config.Output{"clients": {Type: "wireguard", Interface: "wg-test", Subnet: "10.0.0.0/24"}}, Upstreams: map[string]config.Upstream{"private": {Type: "snx-rs", Interface: "snx-test", DNS: []string{"192.0.2.53"}}}, Rules: []config.Rule{{Name: "private-domain", Via: "private", DomainMatch: "sni", Domains: []string{"*.example.test"}}}}
	plan := New(runner.NewDryRun()).ApplyConfig(cfg)
	if len(plan.DNSRules) != 1 || plan.DNSRules[0].Server != "192.0.2.53" || plan.DNSRules[0].Set != "" {
		t.Fatalf("DNS requirements lost or expanded into an IP set: %+v", plan.DNSRules)
	}
	found := false
	for _, result := range plan.Results {
		if strings.Contains(result.Command, "route replace default dev snx-test") {
			found = true
		}
	}
	if found == false {
		t.Fatal("SNI-only SNX upstream has no default route")
	}
}

func TestUnsupportedPoliciesFailBeforeAnyRuntimeCommand(t *testing.T) {
	for _, cfg := range []config.Config{
		{Upstreams: map[string]config.Upstream{"bad": {Type: "unknown"}}},
		{Upstreams: map[string]config.Upstream{"bad": {Type: "xray"}}},
		{Outputs: map[string]config.Output{"external": {Type: "xray"}}, Rules: []config.Rule{{DomainMatch: "sni", Domains: []string{"example.test"}}}},
	} {
		plan := New(runner.NewDryRun()).ApplyConfig(cfg)
		if len(plan.Results) != 1 || plan.Results[0].OK {
			t.Fatalf("unsupported policy reached runtime: %+v", plan.Results)
		}
	}
}

func TestDomainRouterCaptureResourcesAreConfigurable(t *testing.T) {
	cfg := sniFixture()
	cfg.Server.DomainRouter = config.DomainRouter{Port: 19999, Table: 19998, TCPPorts: []int{8443}, UDPPorts: []int{9443}}
	body, err := renderSNI(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"port": 19999`) == false {
		t.Fatal("listener port ignored")
	}
	results := New(runner.NewDryRun()).applySNICapture(cfg)
	for _, part := range []string{"table 19998", "--on-port", "19999", "--dports", "8443", "9443"} {
		if strings.Contains(results[0].Command, part) == false {
			t.Fatalf("capture ignored %s", part)
		}
	}
}
