package engine

import (
	"granger/internal/config"
	"strings"
	"testing"
)

func TestHostControlAccessDoesNotAllowRevokedClients(t *testing.T) {
	cfg := config.Config{Server: config.Server{GUIVPNPort: 80}, Users: map[string]config.User{"active": {}, "disabled": {Disabled: true}}, Outputs: map[string]config.Output{
		"awg": {Interface: "awg0", ServerIP: "10.19.84.1", Clients: []config.Client{
			{User: "active", IP: "10.19.84.2/32"},
			{User: "disabled", IP: "10.19.84.3/32"},
			{User: "missing", IP: "10.19.84.4/32"},
			{User: "active", IP: "10.19.84.5/32", Disabled: true},
		}},
	}}
	body := renderHostControlAccess(cfg)
	if strings.Contains(body, `iifname "awg0" ip saddr 10.19.84.2 ip daddr 10.19.84.1 tcp dport 80 accept`) == false {
		t.Fatal("GUI not allowed for active user")
	}
	for _, ip := range []string{"10.19.84.3", "10.19.84.4", "10.19.84.5"} {
		if strings.Contains(body, "ip saddr "+ip) {
			t.Fatalf("inactive client allowed: %s", ip)
		}
	}
	cfg.Server.GUIVPNPort = 0
	if strings.Contains(renderHostControlAccess(cfg), "dport 80") {
		t.Fatal("disabled GUI allowed")
	}
}

func TestHostSNIAllowsOnlyMarkedWebTrafficFromActiveVPNClients(t *testing.T) {
	cfg := sniFixture()
	out := cfg.Outputs["vpn"]
	out.ServerIP = "10.19.84.51"
	cfg.Outputs["vpn"] = out
	body := renderHostControlAccess(cfg)
	if strings.Contains(body, `iifname "awg-test" ip saddr 10.19.84.2 meta mark & 0x80000000 == 0x80000000 tcp dport { 80, 443 } accept`) == false {
		t.Fatal("active transparent web traffic was not allowed")
	}
	if strings.Contains(body, "ip saddr 10.19.84.5") || strings.Contains(body, "19845") {
		t.Fatal("revoked client or direct listener access was allowed")
	}
	cfg.Rules[0].DomainMatch = ""
	if strings.Contains(renderHostControlAccess(cfg), "0x80000000") {
		t.Fatal("SNI access remains when SNI routing is disabled")
	}
}
