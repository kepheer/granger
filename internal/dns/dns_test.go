package dns

import (
	"strings"
	"testing"

	"granger/internal/config"
	"granger/pkg/runner"
)

func TestRenderedConfigStableAcrossOutputMapOrder(t *testing.T) {
	cfg := config.Default("192.0.2.1", "eth0", "192.0.2.254")
	cfg.Outputs = map[string]config.Output{"b": {Interface: "wg2", ServerIP: "10.2.0.1"}, "a": {Interface: "wg1", ServerIP: "10.1.0.1"}}
	renderer := New(runner.NewDryRun())
	expected := renderer.Render(cfg, nil)
	for attempt := 0; attempt < 50; attempt++ {
		if renderer.Render(cfg, nil) != expected {
			t.Fatal("output map order must not trigger DNS restarts")
		}
	}
	if strings.Index(expected, "interface=wg1") > strings.Index(expected, "interface=wg2") {
		t.Fatal("expected sorted output interfaces")
	}
}

func TestLocalGUIMatchesEnabledVPNAddresses(t *testing.T) {
	disabled := false
	cfg := config.Default("192.0.2.1", "eth0", "192.0.2.254")
	cfg.Outputs = map[string]config.Output{
		"awg": {Interface: "awg0", ServerIP: "10.19.84.1"},
		"wg":  {Interface: "wg0", ServerIP: "10.64.0.1"},
		"off": {Interface: "wg2", ServerIP: "10.3.0.1", Enabled: &disabled},
	}
	renderer := New(runner.NewDryRun())
	body := renderer.Render(cfg, nil)
	for _, address := range []string{"10.19.84.1", "10.64.0.1"} {
		if strings.Contains(body, "host-record=granger.local,"+address+"\n") == false {
			t.Fatalf("missing GUI record for %s", address)
		}
	}
	if strings.Contains(body, "host-record=granger.local,10.3.0.1") {
		t.Fatal("disabled output published")
	}
	cfg.Outputs = map[string]config.Output{}
	if strings.Contains(renderer.Render(cfg, nil), "host-record=granger.local,127.0.0.1") == false {
		t.Fatal("base installation lacks local GUI record")
	}
	cfg.Server.GUIVPNPort = 0
	if strings.Contains(renderer.Render(cfg, nil), "granger.local") {
		t.Fatal("disabled GUI published DNS")
	}
}
