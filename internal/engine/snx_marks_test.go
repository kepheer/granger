package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"granger/internal/config"
	"granger/pkg/runner"
)

func TestSNXConnectionMarksAreScopedAndPreserveAccessBits(t *testing.T) {
	disabled := false
	cfg := config.Config{
		Upstreams: map[string]config.Upstream{
			"office": {Type: "snx-rs", Interface: "custom-vpn0"},
			"unused": {Type: "snx-rs", Interface: "disabled0", Enabled: &disabled},
			"direct": {Type: "direct", Interface: "eth0"},
		},
		Outputs: map[string]config.Output{
			"clients": {Type: "amneziawg", Interface: "custom-clients0"},
			"unused":  {Type: "wireguard", Interface: "disabled-client0", Enabled: &disabled},
		},
	}
	results := New(runner.NewDryRun()).applySNXConnectionMarks(cfg)
	if len(results) != 5 {
		t.Fatalf("unexpected results: %+v", results)
	}
	if strings.Contains(results[1].Command, "custom-vpn0/src_valid_mark=1") == false {
		t.Fatalf("source validation not scoped to SNX: %+v", results[1])
	}
	for _, result := range results[2:] {
		if strings.Contains(strings.ReplaceAll(result.Command, "'", ""), "--nfmask 0xffff --ctmask 0xffff") == false {
			t.Fatalf("access bits could be overwritten: %+v", result)
		}
		if strings.Contains(result.Command, "disabled") || strings.Contains(result.Command, "eth0") {
			t.Fatalf("unrelated interface changed: %+v", result)
		}
	}
	if strings.Contains(results[4].Command, "GRANGER_OUTPUT") == false {
		t.Fatal("local SNX connections do not preserve their return mark")
	}
	if strings.Contains(results[2].Command, "custom-vpn0") == false || strings.Contains(results[3].Command, "custom-clients0") == false {
		t.Fatalf("wrong interface scope: %+v", results)
	}
}

func TestSNXMarksApplyWhenInterfaceAppearsAfterStartup(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "applied")
	t.Setenv("GRANGER_SNX_TEST_LOG", log)
	t.Setenv("GRANGER_SNX_TEST_CONNECTED", "no")
	for name, body := range map[string]string{
		"ip":     "#!/bin/sh\n[ \"$GRANGER_SNX_TEST_CONNECTED\" = yes ]\n",
		"sysctl": "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GRANGER_SNX_TEST_LOG\"\n",
		"sh":     "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GRANGER_SNX_TEST_LOG\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cfg := config.Config{
		Upstreams: map[string]config.Upstream{"office": {Type: "snx-rs", Interface: "late-vpn0"}},
		Outputs:   map[string]config.Output{"clients": {Type: "amneziawg", Interface: "clients0"}},
	}
	engine := New(runner.New())
	if results := engine.applySNXConnectionMarks(cfg); len(results) != 0 {
		t.Fatalf("missing interface changed network settings: %+v", results)
	}
	if _, err := os.Stat(log); os.IsNotExist(err) == false {
		t.Fatal("network settings were touched before interface existed")
	}
	t.Setenv("GRANGER_SNX_TEST_CONNECTED", "yes")
	results := engine.applySNXConnectionMarks(cfg)
	if len(results) != 5 || resultsOK(results) == false {
		t.Fatalf("interface appearance did not apply settings: %+v", results)
	}
	body, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"late-vpn0/forwarding=1", "late-vpn0/src_valid_mark=1", "--save-mark", "--restore-mark"} {
		if strings.Contains(string(body), value) == false {
			t.Fatalf("missing restored setting %s: %s", value, body)
		}
	}
}
