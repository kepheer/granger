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
