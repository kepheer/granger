package engine

import (
	"granger/internal/config"
	"strings"
	"testing"
)

func TestFinalEgressEnforcesActualInterfaceAndPrivateNetworkPermission(t *testing.T) {
	cfg := sniFixture()
	cfg.Upstreams["private"] = config.Upstream{Type: "interface", Interface: "private0", AllowPrivate: true}
	user := cfg.Users["allowed"]
	user.AllowedUpstreams = append(user.AllowedUpstreams, "private")
	cfg.Users["allowed"] = user
	body := renderEgressAccess(cfg)
	if strings.Contains(body, `"-o" "private0" "-j" "RETURN"`) == false {
		t.Fatal("allowed private interface missing")
	}
	if strings.Contains(body, `"-o" "eth0" "-d" "10.0.0.0/8" "-j" "DROP"`) == false {
		t.Fatal("private destination can escape over a public upstream")
	}
	if strings.Contains(body, `"--mark" "0x40000000/0x40000000" "-j" "DROP"`) == false {
		t.Fatal("unknown proxy identities are not blocked")
	}
	out := cfg.Outputs["vpn"]
	for i := range out.Clients {
		out.Clients[i].Disabled = true
	}
	cfg.Outputs["vpn"] = out
	if strings.Contains(renderEgressAccess(cfg), `"RETURN"`) {
		t.Fatal("revoked profiles retain egress access")
	}
}
