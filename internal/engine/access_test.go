package engine

import (
	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestPersonalAccessBlocksDisallowedUpstreamBeforeSharedRules(t *testing.T) {
	cfg := config.Default("192.0.2.1", "eth0", "192.0.2.254")
	cfg.Users = map[string]config.User{"alice": {AccessRestricted: true, AllowedUpstreams: []string{"direct"}, DefaultUpstream: "direct"}}
	cfg.Outputs["clients"] = config.Output{Type: "wireguard", Interface: "wg0", Subnet: "10.66.0.0/24", Clients: []config.Client{{Name: "alice", User: "alice", IP: "10.66.0.2/32"}}}
	cfg.Upstreams["direct"] = config.Upstream{Type: "direct", Interface: "eth0", Gateway: "192.0.2.254"}
	cfg.Upstreams["corporate"] = config.Upstream{Type: "interface", Interface: "snx0"}
	cfg.Rules = []config.Rule{{Name: "corporate", CIDRs: []string{"10.44.0.0/16"}, Via: "corporate"}}
	plan := New(runner.NewDryRun()).ApplyConfig(cfg)
	policyAt := -1
	sharedAt := -1
	hasDefault := false
	for index, result := range plan.Results {
		if result.Title == "Apply personal access atomically" {
			policyAt = index
			allow := strings.Index(result.Output, `"-o" "eth0" "-j" "RETURN"`)
			deny := strings.Index(result.Output, `-A GRANGER_USER_FORWARD "-i" "wg0" "-s" "10.66.0.2/32" "-j" "REJECT"`)
			if allow < 0 || deny < allow || strings.Contains(result.Output, `"-o" "snx0"`) {
				t.Fatalf("invalid personal policy: %s", result.Output)
			}
		}
		if strings.HasPrefix(result.Title, "FORWARD output ->") {
			sharedAt = index
		}
		if result.Title == "Default route for user alice" {
			hasDefault = true
		}
	}
	if policyAt < 0 || sharedAt < policyAt || hasDefault == false {
		t.Fatalf("incomplete policy: %d %d %v", policyAt, sharedAt, hasDefault)
	}
}

func inputDecision(t *testing.T, policy, iface, source, destination, protocol, port string) string {
	t.Helper()
	packet := map[string]string{"-i": iface, "-s": source, "-d": destination, "-p": protocol, "--dport": port}
	for _, line := range strings.Split(policy, "\n") {
		if strings.HasPrefix(line, "-A "+netfilter.UserInputChain+" ") == false {
			continue
		}
		parts := strings.Fields(line)[2:]
		matches := true
		for index := 0; index < len(parts); index += 2 {
			key, err := strconv.Unquote(parts[index])
			if err != nil {
				t.Fatal(err)
			}
			value, err := strconv.Unquote(parts[index+1])
			if err != nil {
				t.Fatal(err)
			}
			if key == "-j" {
				if matches {
					return value
				}
				break
			}
			if key == "-s" && strings.Contains(value, "/") {
				_, subnet, err := net.ParseCIDR(value)
				if err != nil {
					t.Fatal(err)
				}
				matches = matches && subnet.Contains(net.ParseIP(source))
				continue
			}
			matches = matches && packet[key] == value
		}
	}
	return "RETURN"
}

func TestManagedClientInputAllowsDNSAndBlocksLocalServices(t *testing.T) {
	cfg := config.Config{
		Users: map[string]config.User{
			"restricted": {AccessRestricted: true},
			"legacy":     {},
			"admin":      {Admin: true, AccessRestricted: true},
			"disabled":   {Disabled: true},
		},
		Outputs: map[string]config.Output{"clients": {
			Managed: true, Type: "wireguard", Interface: "wg0", ServerIP: "10.66.0.1",
			Clients: []config.Client{
				{Name: "restricted", User: "restricted", IP: "10.66.0.2/32"},
				{Name: "legacy", User: "legacy", IP: "10.66.0.3/32"},
				{Name: "admin", User: "admin", IP: "10.66.0.4/32"},
				{Name: "disabled", User: "disabled", IP: "10.66.0.5/32"},
				{Name: "missing", User: "missing", IP: "10.66.0.6/32"},
				{Name: "revoked", User: "legacy", IP: "10.66.0.7/32", Disabled: true},
			},
		}},
	}
	policy := New(runner.NewDryRun()).applyUserAccess(cfg)[0].Output
	cases := []struct{ source, destination, protocol, port, want string }{
		{"10.66.0.2", "10.66.0.1", "udp", "53", "ACCEPT"},
		{"10.66.0.2", "10.66.0.1", "tcp", "53", "ACCEPT"},
		{"10.66.0.2", "10.66.0.1", "tcp", "2288", "REJECT"},
		{"10.66.0.2", "10.44.0.1", "tcp", "8080", "REJECT"},
		{"10.66.0.2", "10.44.0.1", "udp", "53", "REJECT"},
		{"10.66.0.3", "10.66.0.1", "tcp", "2288", "RETURN"},
		{"10.66.0.4", "10.66.0.1", "tcp", "2288", "RETURN"},
		{"10.66.0.5", "10.66.0.1", "udp", "53", "REJECT"},
		{"10.66.0.6", "10.66.0.1", "udp", "53", "REJECT"},
		{"10.66.0.7", "10.66.0.1", "udp", "53", "REJECT"},
		{"10.66.0.99", "10.66.0.1", "tcp", "2288", "REJECT"},
	}
	for _, test := range cases {
		if got := inputDecision(t, policy, "wg0", test.source, test.destination, test.protocol, test.port); got != test.want {
			t.Errorf("%+v: got %s", test, got)
		}
	}
	if got := inputDecision(t, policy, "eth0", "192.0.2.8", "192.0.2.1", "tcp", "2288"); got != "RETURN" {
		t.Fatalf("VPN policy affected host management interface: %s", got)
	}
}

func TestDisabledOutputsRetainInputAndForwardDeny(t *testing.T) {
	disabled := false
	for _, managed := range []bool{true, false} {
		cfg := config.Config{Outputs: map[string]config.Output{"clients": {
			Managed: managed, Type: "wireguard", Interface: "wg0", Enabled: &disabled,
			Clients: []config.Client{{Name: "legacy", IP: "10.66.0.2/32"}},
		}}}
		policy := New(runner.NewDryRun()).applyUserAccess(cfg)[0].Output
		if got := inputDecision(t, policy, "wg0", "10.66.0.2", "10.66.0.1", "udp", "53"); got != "REJECT" {
			t.Fatalf("disabled output allowed DNS: %s", got)
		}
		for _, direction := range []string{"-i", "-o"} {
			if strings.Contains(policy, accessRule(netfilter.UserForwardChain, direction, "wg0", "-j", "REJECT")) == false {
				t.Fatalf("disabled output lacks FORWARD deny: %s", policy)
			}
		}
		if strings.Contains(policy, `"-j" "RETURN"`) {
			t.Fatalf("disabled output retained client allow: %s", policy)
		}
	}
}

func TestRestrictedProxyRejectsLocalServicesBeforeAllowedInterface(t *testing.T) {
	cfg := config.Config{
		Users:     map[string]config.User{"alice": {AccessRestricted: true, AllowedUpstreams: []string{"direct"}}},
		Outputs:   map[string]config.Output{"proxy": {Managed: true, Type: "xray", Clients: []config.Client{{Name: "alice", User: "alice"}}}},
		Upstreams: map[string]config.Upstream{"direct": {Type: "direct", Interface: "eth0"}},
	}
	policy := New(runner.NewDryRun()).applyUserAccess(cfg)[0].Output
	localDeny := strings.Index(policy, `"--dst-type" "LOCAL" "-j" "REJECT"`)
	upstreamAllow := strings.Index(policy, `"-o" "eth0" "-j" "RETURN"`)
	if localDeny < 0 || upstreamAllow <= localDeny {
		t.Fatalf("restricted proxy must reject host services before allowing its upstream: %s", policy)
	}
}
