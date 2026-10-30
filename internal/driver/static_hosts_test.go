package driver

import (
	"fmt"
	"granger/internal/config"
	"granger/pkg/runner"
	"strings"
	"testing"
)

func TestHostCIDRsAreBatchedWithoutExpiringDNSCacheEntries(t *testing.T) {
	cfg := config.Default("192.0.2.1", "eth0", "192.0.2.254")
	cidrs := []string{"10.0.0.0/16"}
	for i := 0; i < 500; i++ {
		cidrs = append(cidrs, fmt.Sprintf("198.18.%d.%d/32", i/256, i%256))
	}
	cfg.Rules = []config.Rule{{Name: "ru", Via: "direct", CIDRs: cidrs}}
	ctx := BaseContext(cfg, runner.NewDryRun())
	ctx.Output = config.Output{Interface: "awg0"}
	ctx.ClientCIDR = "10.19.84.0/24"
	results := applyUpstreamFirewall("direct", config.Upstream{Interface: "eth0"}, ctx)
	individual := 0
	batches := 0
	for _, result := range results {
		if strings.HasPrefix(result.Title, "Mark CIDR ") {
			individual++
		}
		if result.Title == "Populate static host set direct" {
			batches++
			if strings.Count(result.Output, "add ") != 500 || strings.Contains(result.Output, "swap ") == false || strings.Contains(result.Output, "timeout") {
				t.Fatalf("invalid static host transaction: %s", result.Output)
			}
		}
	}
	if individual != 1 || batches != 1 || len(results) > 20 {
		t.Fatalf("individual=%d batches=%d results=%d", individual, batches, len(results))
	}
}
