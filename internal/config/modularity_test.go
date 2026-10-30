package config

import (
	"fmt"
	"granger/pkg/netfilter"
	"strings"
	"testing"
)

func TestUnsupportedModulesAndIgnoredOptionsAreRejected(t *testing.T) {
	for _, up := range []Upstream{{Type: "typo"}, {Type: "xray"}, {Type: "sing-box"}, {Type: "interface", Interface: "tun0", RouteMode: "ignored"}, {Type: "interface", Interface: "tun0", Install: &InstallSpec{Package: "anything"}}} {
		if err := (Config{Upstreams: map[string]Upstream{"test": up}}).Validate(); err == nil {
			t.Fatalf("accepted ineffective configuration: %+v", up)
		}
	}
}

func TestSNXDaemonInstanceConstraintIsExplicit(t *testing.T) {
	cfg := Config{Upstreams: map[string]Upstream{"one": {Type: "snx-rs", Interface: "snx0"}, "two": {Type: "snx-rs", Interface: "snx1"}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("multiple instances silently share a daemon")
	}
	disabled := false
	up := cfg.Upstreams["two"]
	up.Enabled = &disabled
	cfg.Upstreams["two"] = up
	if err := cfg.Validate(); err != nil {
		t.Fatalf("disabled connection should be configurable: %v", err)
	}
}

func TestSNIRoutingIdentitiesCannotCollideAcrossProfiles(t *testing.T) {
	seen := map[int]string{}
	first, second := "", ""
	for index := 0; index < 20000; index++ {
		name := fmt.Sprintf("client%d", index)
		mark := netfilter.UserMark("vpn", name)
		if previous, found := seen[mark]; found {
			first, second = previous, name
			break
		}
		seen[mark] = name
	}
	if first == "" {
		t.Fatal("collision fixture was not found")
	}
	cfg := Config{Outputs: map[string]Output{"vpn": {Type: "wireguard", Interface: "wg0", Subnet: "10.2.0.0/24", ServerIP: "10.2.0.1", Clients: []Client{{Name: first, IP: "10.2.0.2/32"}, {Name: second, IP: "10.2.0.3/32"}}}}, Upstreams: map[string]Upstream{"exit": {Type: "interface", Interface: "eth9"}}, Rules: []Rule{{Name: "domain", DomainMatch: "sni", Domains: []string{"example.test"}, Via: "exit"}}}
	if err := cfg.Validate(); err == nil || strings.Contains(err.Error(), "routing identity collision") == false {
		t.Fatalf("colliding SNI identities accepted: %v", err)
	}
}

func TestIgnoredManagedProtocolOptionsAreRejected(t *testing.T) {
	cfg := Config{Outputs: map[string]Output{"proxy": {Type: "xray", ProtocolOptions: map[string]string{"transport": "unsupported"}}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("ignored protocol options were accepted")
	}
}
