package config

import "testing"

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
