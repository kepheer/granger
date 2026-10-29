package engine

import (
	"granger/internal/config"
	"granger/pkg/runner"
	"strings"
	"testing"
)

func TestAuditCIDRFallbackOnDisabledPrimary(t *testing.T) {
	disabled := false
	cfg := config.Default("192.0.2.2", "eth0", "192.0.2.1")
	cfg.Outputs["clients"] = config.Output{Type: "wireguard", Interface: "wg0", Subnet: "10.66.0.0/24"}
	cfg.Upstreams["primary"] = config.Upstream{Type: "interface", Interface: "tun0", Enabled: &disabled, FallbackWhenDown: "backup"}
	cfg.Upstreams["backup"] = config.Upstream{Type: "direct", Interface: "eth0", Gateway: "192.0.2.1"}
	cfg.Rules = []config.Rule{{Name: "corp", CIDRs: []string{"10.44.0.0/16"}, Via: "primary"}}
	plan := New(runner.NewDryRun()).ApplyConfig(cfg)
	for _, result := range plan.Results {
		if strings.Contains(result.Title, "Mark CIDR 10.44.0.0/16 via backup") {
			return
		}
	}
	t.Fatal("disabled primary has fallback configured, but no CIDR routing to backup is generated")
}
