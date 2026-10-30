package engine

import (
	"fmt"
	"granger/internal/config"
	"granger/pkg/netfilter"
	"strings"
	"testing"
)

func TestPrivateDestinationsRequireAnAllowedOptedInUpstream(t *testing.T) {
	cfg := sniFixture()
	cfg.Upstreams["private"] = config.Upstream{Type: "interface", Interface: "vpn-private", AllowPrivate: true}
	user := cfg.Users["allowed"]
	user.AllowedUpstreams = append(user.AllowedUpstreams, "private")
	cfg.Users["allowed"] = user
	for _, id := range []string{"allowed", "limited"} {
		var body strings.Builder
		match := []string{"-m", "mark", "--mark", "0x40010000/0xffff0000"}
		writePrivateAccess(&body, cfg, config.Client{User: id}, netfilter.UserOutputChain, match)
		if strings.Index(body.String(), `"--dst-type" "LOCAL"`) > strings.Index(body.String(), `"RETURN"`) && id == "allowed" {
			t.Fatal("private permission exposes local services")
		}
		expectedMark := fmt.Sprintf("0x%x/0xffff", netfilter.RouteTableID("private"))
		if strings.Contains(body.String(), expectedMark) != (id == "allowed") {
			t.Fatalf("private upstream permission not enforced: %s", body.String())
		}
		if strings.Contains(body.String(), `"10.0.0.0/8" "-j" "REJECT"`) == false {
			t.Fatal("unselected private destinations are not blocked")
		}
		if id == "allowed" && strings.Index(body.String(), `"RETURN"`) > strings.Index(body.String(), `"10.0.0.0/8" "-j" "REJECT"`) {
			t.Fatal("explicit private route does not precede the default block")
		}
	}
}
