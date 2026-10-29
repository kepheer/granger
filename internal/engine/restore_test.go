package engine

import (
	"granger/internal/config"
	"granger/pkg/runner"
	"strings"
	"testing"
)

func TestRestoreStartsConfiguredTunnelsBeforeRouting(t *testing.T) {
	cfg := config.Default("192.0.2.2", "eth0", "192.0.2.1")
	cfg.Outputs["clients"] = config.Output{Type: "wireguard", Interface: "wg0", Subnet: "10.66.0.0/24"}
	cfg.Upstreams["exit"] = config.Upstream{Type: "wireguard", Interface: "wg-up0"}
	cfg.Rules = []config.Rule{{Name: "default", Via: "exit", Default: true}}
	plan := New(runner.NewDryRun()).RestoreConfig(cfg)
	started := map[string]bool{}
	for _, result := range plan.Results {
		if strings.Contains(result.Title, "Start WireGuard output") {
			started["output"] = true
		}
		if strings.Contains(result.Title, "Start WireGuard upstream") {
			started["upstream"] = true
		}
		if strings.Contains(result.Title, "Route default") && (started["output"] == false || started["upstream"] == false) {
			t.Fatal("routing restored before configured tunnels were started")
		}
	}
	if started["output"] == false || started["upstream"] == false {
		t.Fatalf("missing startup operations: %v", started)
	}
}

func TestRestoreDoesNotStartDisabledTunnels(t *testing.T) {
	disabled := false
	cfg := config.Default("192.0.2.2", "eth0", "192.0.2.1")
	cfg.Outputs["clients"] = config.Output{Type: "wireguard", Interface: "wg0", Enabled: &disabled}
	cfg.Upstreams["exit"] = config.Upstream{Type: "wireguard", Interface: "wg-up0", Enabled: &disabled}
	plan := New(runner.NewDryRun()).RestoreConfig(cfg)
	stopped := 0
	for _, result := range plan.Results {
		if strings.HasPrefix(result.Title, "Start WireGuard") {
			t.Fatalf("disabled tunnel started: %s", result.Title)
		}
		if strings.HasPrefix(result.Title, "Stop WireGuard") {
			stopped++
		}
	}
	if stopped != 2 {
		t.Fatalf("disabled services not stopped: %d", stopped)
	}
}
