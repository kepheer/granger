package webgui

import (
	"granger/internal/config"
	"testing"
)

func TestAuditGraphRoundTripPreservesUserIdentity(t *testing.T) {
	cfg := config.Default("", "eth0", "192.0.2.1")
	cfg.Users["usr_123"] = config.User{DisplayName: "Alice"}
	next, err := configFromGraph(buildRoutingGraph(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Users) != 1 {
		t.Fatalf("unchanged graph duplicated user: %#v", next.Users)
	}
}

func TestAuditGraphAllowsDeletingLastRule(t *testing.T) {
	cfg := config.Default("", "eth0", "192.0.2.1")
	cfg.Rules = []config.Rule{{Name: "default", Via: "direct", Default: true}}
	graph := buildRoutingGraph(cfg)
	graph.Nodes = nil
	graph.Edges = nil
	next, err := configFromGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Rules) != 0 {
		t.Fatalf("deleted rule remains: %#v", next.Rules)
	}
}

func TestAuditGraphRewiringChangesRoute(t *testing.T) {
	cfg := config.Default("", "eth0", "192.0.2.1")
	cfg.Rules = []config.Rule{{Name: "default", Via: "primary", Default: true}}
	graph := buildRoutingGraph(cfg)
	graph.Edges[0].Target = "upstream:backup"
	next, err := configFromGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	if next.Rules[0].Via != "backup" {
		t.Fatalf("rewired edge ignored: via=%s", next.Rules[0].Via)
	}
}

func TestRoutingVersionDetectsConcurrentRevocation(t *testing.T) {
	cfg := config.Default("192.0.2.1", "eth0", "192.0.2.254")
	cfg.Users["alice"] = config.User{DisplayName: "Alice", AccessRestricted: true, AllowedUpstreams: []string{"direct"}}
	graph := buildRoutingGraph(cfg)
	user := cfg.Users["alice"]
	user.Disabled = true
	cfg.Users["alice"] = user
	if graph.Version == configVersion(cfg) {
		t.Fatal("graph saved before revocation must be rejected after revocation")
	}
	if graph.Version == "" {
		t.Fatal("graph requires configuration version")
	}
}
