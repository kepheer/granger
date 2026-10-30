package engine

import (
	"granger/internal/config"
	"strings"
	"testing"
)

func TestDNSCaptureOnlyMatchesActiveManagedClients(t *testing.T) {
	off := false
	cfg := config.Config{Users: map[string]config.User{"active": {}, "off": {Disabled: true}}, Outputs: map[string]config.Output{
		"awg":       {Managed: true, Interface: "awg0", ServerIP: "10.19.84.1", Clients: []config.Client{{User: "active", IP: "10.19.84.2/32"}, {User: "off", IP: "10.19.84.3/32"}, {User: "missing", IP: "10.19.84.4/32"}, {User: "active", IP: "10.19.84.5/32", Disabled: true}}},
		"disabled":  {Managed: true, Interface: "wg1", ServerIP: "10.2.0.1", Enabled: &off, Clients: []config.Client{{User: "active", IP: "10.2.0.2/32"}}},
		"unmanaged": {Interface: "eth1", ServerIP: "10.3.0.1", Clients: []config.Client{{User: "active", IP: "10.3.0.2/32"}}},
	}}
	body := renderDNSCapture(cfg)
	for _, protocol := range []string{"udp", "tcp"} {
		expected := `"-i" "awg0" "-s" "10.19.84.2/32" "-p" "` + protocol + `" "--dport" "53" "-j" "DNAT" "--to-destination" "10.19.84.1:53"`
		if strings.Contains(body, expected) == false {
			t.Fatalf("missing DNS capture: %s", protocol)
		}
	}
	for _, value := range []string{"10.19.84.3", "10.19.84.4", "10.19.84.5", "wg1", "eth1"} {
		if strings.Contains(body, value) {
			t.Fatalf("captured excluded client/interface: %s", value)
		}
	}
	if strings.Contains(body, `"OUTPUT"`) || strings.Contains(body, `"--dport" "443"`) {
		t.Fatal("capture affected host or HTTPS traffic")
	}
}
