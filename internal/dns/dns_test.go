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
