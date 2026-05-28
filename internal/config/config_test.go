package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsDuplicateInterfaces(t *testing.T) {
	cfg := Config{
		Outputs: map[string]Output{
			"home": {Type: "wireguard", Interface: "wg0"},
		},
		Upstreams: map[string]Upstream{
			"exit": {Type: "wireguard", Interface: "wg0"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate accepted duplicate interface")
	}
	if !strings.Contains(err.Error(), "conflicts") || !strings.Contains(err.Error(), "wg0") {
		t.Fatalf("duplicate interface error = %q", err)
	}
}

func TestValidateAllowsRepeatedAutoInterface(t *testing.T) {
	cfg := Config{
		Outputs: map[string]Output{
			"home": {Type: "wireguard", Interface: "wg0"},
		},
		Upstreams: map[string]Upstream{
			"direct_a": {Type: "direct", Interface: "auto"},
			"direct_b": {Type: "direct", Interface: "auto"},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate rejected repeated auto interface: %v", err)
	}
}

func TestValidateRejectsDuplicateExplicitServices(t *testing.T) {
	cfg := Config{
		Outputs: map[string]Output{
			"home": {Type: "wireguard", Interface: "wg0", Service: "vpn.service"},
		},
		Upstreams: map[string]Upstream{
			"exit": {Type: "wireguard", Interface: "wg1", Service: "vpn.service"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate accepted duplicate explicit service")
	}
	if !strings.Contains(err.Error(), "conflicts") || !strings.Contains(err.Error(), "vpn.service") {
		t.Fatalf("duplicate service error = %q", err)
	}
}

func TestValidateAllowsProxyOutputsWithoutInterface(t *testing.T) {
	cfg := Config{
		Outputs: map[string]Output{
			"xray_public": {
				Type:    "xray",
				Service: "xray@public.service",
			},
			"singbox_public": {
				Type:    "sing-box",
				Service: "sing-box@public.service",
			},
		},
		Upstreams: map[string]Upstream{},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate rejected proxy output without interface: %v", err)
	}
}

func TestLoadReadsYAMLConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "granger.yaml")
	body := []byte(`
server:
  public_ip: 203.0.113.10
users:
  alice:
    display_name: Alice
outputs:
  home:
    type: wireguard
    interface: wg0
    clients:
      - name: alice-phone
        user: alice
        ip: 10.66.0.2
upstreams:
  direct:
    type: direct
    interface: auto
rules:
  - name: default
    default: true
    via: direct
`)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.PublicIP != "203.0.113.10" {
		t.Fatalf("PublicIP = %q", cfg.Server.PublicIP)
	}
	if cfg.Outputs["home"].Interface != "wg0" {
		t.Fatalf("home interface = %q", cfg.Outputs["home"].Interface)
	}
}

func TestValidateRejectsClientWithMissingUser(t *testing.T) {
	cfg := Config{
		Outputs: map[string]Output{
			"home": {
				Type:      "wireguard",
				Interface: "wg0",
				Clients: []Client{
					{Name: "phone", User: "missing", IP: "10.66.0.2"},
				},
			},
		},
		Upstreams: map[string]Upstream{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate accepted client with missing user")
	}
	if !strings.Contains(err.Error(), "missing user") {
		t.Fatalf("missing user error = %q", err)
	}
}

func TestEnabledClientsSkipsDisabledUsersAndClients(t *testing.T) {
	cfg := Config{
		Users: map[string]User{
			"alice": {},
			"bob":   {Disabled: true},
		},
	}
	out := Output{
		Clients: []Client{
			{Name: "alice-phone", User: "alice", Config: "/alice.conf"},
			{Name: "bob-phone", User: "bob", Config: "/bob.conf"},
			{Name: "guest-phone", Config: "/guest.conf"},
			{Name: "old-phone", Config: "/old.conf", Disabled: true},
		},
	}

	clients := cfg.EnabledClients(out)
	if len(clients) != 2 {
		t.Fatalf("enabled clients = %d, want 2", len(clients))
	}
	if clients[0].Name != "alice-phone" || clients[1].Name != "guest-phone" {
		t.Fatalf("enabled clients = %#v", clients)
	}
}
