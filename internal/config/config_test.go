package config

import (
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
