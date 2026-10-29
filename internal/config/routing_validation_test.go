package config

import (
	"strings"
	"testing"

	"granger/pkg/netfilter"
)

func TestValidateRejectsUpstreamRoutingIdentityCollision(t *testing.T) {
	if netfilter.RouteTableID("vpn_23") != netfilter.RouteTableID("vpn_140") {
		t.Fatal("collision fixture no longer collides")
	}
	disabled := false
	cfg := Config{Upstreams: map[string]Upstream{
		"vpn_23":  {Type: "interface", Interface: "tun0"},
		"vpn_140": {Type: "interface", Interface: "tun1", Enabled: &disabled},
	}}
	if err := cfg.Validate(); err == nil || strings.Contains(err.Error(), "routing table collision") == false {
		t.Fatalf("expected routing identity collision, got %v", err)
	}
}

func TestValidateRejectsOverlappingOutputSubnets(t *testing.T) {
	for _, second := range []string{"10.66.0.0/24", "10.66.0.128/25", "10.66.0.0/16"} {
		t.Run(second, func(t *testing.T) {
			cfg := Config{Outputs: map[string]Output{
				"first":  {Type: "wireguard", Interface: "wg0", Subnet: "10.66.0.0/24"},
				"second": {Type: "openvpn", Interface: "tun0", Subnet: second},
			}}
			if err := cfg.Validate(); err == nil || strings.Contains(err.Error(), "overlaps") == false {
				t.Fatalf("expected subnet overlap, got %v", err)
			}
		})
	}
	cfg := Config{Outputs: map[string]Output{
		"first":  {Type: "wireguard", Interface: "wg0", Subnet: "10.66.0.0/25"},
		"second": {Type: "openvpn", Interface: "tun0", Subnet: "10.66.0.128/25"},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("adjacent distinct subnets must remain valid: %v", err)
	}
}

func validRoutedOutput(typ string) Output {
	return Output{Type: typ, Managed: true, Interface: "vpn0", Subnet: "10.66.0.0/24", ServerIP: "10.66.0.1", Clients: []Client{{Name: "alice", IP: "10.66.0.2/32"}}}
}

func TestValidateRoutedClientIdentities(t *testing.T) {
	for _, typ := range []string{"wireguard", "amneziawg", "openvpn"} {
		t.Run(typ, func(t *testing.T) {
			out := validRoutedOutput(typ)
			if err := (Config{Outputs: map[string]Output{"clients": out}}).Validate(); err != nil {
				t.Fatalf("valid managed output rejected: %v", err)
			}
			for _, address := range []string{"", "10.66.0.2", "10.66.0.2/24", "10.66.1.2/32", "10.66.0.0/32", "10.66.0.255/32", "10.66.0.1/32", "127.0.0.2/32", "::1/128", "garbage"} {
				t.Run(address, func(t *testing.T) {
					out := validRoutedOutput(typ)
					out.Clients[0].IP = address
					if err := (Config{Outputs: map[string]Output{"clients": out}}).Validate(); err == nil {
						t.Fatalf("accepted invalid client identity %q", address)
					}
				})
			}
			out.Clients = append(out.Clients, Client{Name: "bob", IP: "10.66.0.2/32", Disabled: true})
			if err := (Config{Outputs: map[string]Output{"clients": out}}).Validate(); err == nil {
				t.Fatal("accepted duplicate client IPv4 address")
			}
		})
	}
}

func TestValidateRejectsBroadImportedClientIdentity(t *testing.T) {
	out := validRoutedOutput("wireguard")
	out.Managed = false
	out.Clients = []Client{{Name: "broad", IP: "10.66.0.0/24"}, {Name: "restricted", IP: "10.66.0.3/32"}}
	if err := (Config{Outputs: map[string]Output{"clients": out}}).Validate(); err == nil {
		t.Fatal("broad imported identity could shadow another client's ACL")
	}
	out.Clients = []Client{{Name: "legacy-host", IP: "10.66.0.2"}, {Name: "metadata-only"}}
	if err := (Config{Outputs: map[string]Output{"clients": out}}).Validate(); err != nil {
		t.Fatalf("legacy exact host and metadata clients must remain supported: %v", err)
	}
}

func TestValidateManagedQuickInterfaceNames(t *testing.T) {
	for _, name := range []string{"abcdefghijklmnop", "vpn/name", "vpn name", "vpn@host", ".", "..", "lo", "auto"} {
		out := validRoutedOutput("amneziawg")
		out.Interface = name
		if err := (Config{Outputs: map[string]Output{"clients": out}}).Validate(); err == nil {
			t.Fatalf("accepted invalid managed quick interface %q", name)
		}
	}
	out := validRoutedOutput("wireguard")
	out.Interface = "abcdefghijklmno"
	if err := (Config{Outputs: map[string]Output{"clients": out}}).Validate(); err != nil {
		t.Fatalf("15-byte interface should be valid: %v", err)
	}
}

func TestValidateManagedRoutedServerAddresses(t *testing.T) {
	for _, address := range []string{"", "10.66.0.0", "10.66.0.255", "10.67.0.1", "127.0.0.1", "0.0.0.0", "::1", "garbage"} {
		out := validRoutedOutput("wireguard")
		out.ServerIP = address
		if err := (Config{Outputs: map[string]Output{"clients": out}}).Validate(); err == nil {
			t.Fatalf("accepted invalid managed server address %q", address)
		}
	}
	for _, subnet := range []string{"", "::/64", "127.0.0.0/24", "::ffff:10.66.0.0/120"} {
		out := validRoutedOutput("openvpn")
		out.Subnet = subnet
		if err := (Config{Outputs: map[string]Output{"clients": out}}).Validate(); err == nil {
			t.Fatalf("accepted invalid managed IPv4 subnet %q", subnet)
		}
	}
}
