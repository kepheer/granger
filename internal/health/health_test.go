package health

import (
	"granger/internal/config"
	"granger/pkg/runner"
	"testing"
)

func TestValidConfigWithMissingRuntimeIsNotHealthy(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	cfg := config.Default("192.0.2.2", "eth0", "192.0.2.1")
	cfg.Outputs["home"] = config.Output{Type: "wireguard", Interface: "wg0", Subnet: "10.66.0.0/24"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	status := New(runner.New()).Check(cfg)
	if status.Overall == Healthy {
		t.Fatal("valid config reported healthy despite missing tunnels, DNS, and forwarding")
	}
	if len(status.Checks) < 4 {
		t.Fatalf("missing runtime health checks: %v", status.Checks)
	}
}
