package driver

import (
	"encoding/json"
	"granger/internal/config"
	"strings"
	"testing"
)

func TestNativeProxySNIPolicyKeepsIdentityAndPermissions(t *testing.T) {
	for _, protocol := range []string{"xray", "sing-box"} {
		cfg := config.Config{Users: map[string]config.User{"yes": {AccessRestricted: true, AllowedUpstreams: []string{"exit"}}, "no": {AccessRestricted: true}}, Upstreams: map[string]config.Upstream{"exit": {Type: "interface", Interface: "eth9"}}, Rules: []config.Rule{{Via: "exit", DomainMatch: "sni", Domains: []string{"example.test"}}}}
		out := config.Output{Type: protocol, Clients: []config.Client{{Name: "allowed", User: "yes"}, {Name: "denied", User: "no"}, {Name: "revoked", User: "yes", Disabled: true}}}
		routes, exits := proxySNIPolicy(cfg, "proxy", out)
		body, _ := json.Marshal(routes)
		if len(routes) != 1 || len(exits) != 1 || strings.Contains(string(body), "allowed:exit") == false || strings.Contains(string(body), "denied") {
			t.Fatalf("%s lost policy isolation: %s", protocol, body)
		}
	}
}

func TestPrivateFreedomTargetsRemainExplicit(t *testing.T) {
	if len(freedomSettings(false)) != 0 {
		t.Fatal("private destinations enabled by default")
	}
	body, _ := json.Marshal(freedomSettings(true))
	if strings.Contains(string(body), "10.0.0.0/8") == false || strings.Contains(string(body), "0.0.0.0/0") || strings.Contains(string(body), "127.0.0.0") {
		t.Fatal(string(body))
	}
}
