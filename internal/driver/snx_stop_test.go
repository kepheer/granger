package driver

import (
	"os"
	"path/filepath"
	"testing"

	"granger/internal/config"
	"granger/pkg/runner"
)

func TestSNXStopHandlesUninstalledDriverWithoutMaskingStopFailure(t *testing.T) {
	for _, tc := range []struct {
		name, state, exit string
		wantOK, wantStop  bool
	}{
		{"uninstalled", "not-found", "0", true, false},
		{"installed", "loaded", "0", true, true},
		{"stop failure", "loaded", "1", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "stop-called")
			stub := "#!/bin/sh\nif [ \"$1\" = show ]; then printf '%s\\n' \"$GRANGER_TEST_STATE\"; exit 0; fi\nprintf stop > \"$GRANGER_TEST_LOG\"\nexit \"$GRANGER_TEST_EXIT\"\n"
			if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(stub), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GRANGER_TEST_STATE", tc.state)
			t.Setenv("GRANGER_TEST_LOG", log)
			t.Setenv("GRANGER_TEST_EXIT", tc.exit)
			results := (SNXRSUpstream{}).Stop("pending", config.Upstream{}, ApplyContext{Runner: runner.New()})
			if len(results) != 1 || results[0].OK != tc.wantOK {
				t.Fatalf("unexpected stop result: %+v", results)
			}
			_, err := os.Stat(log)
			if (err == nil) != tc.wantStop {
				t.Fatalf("stop called: %v; want %v", err == nil, tc.wantStop)
			}
		})
	}
}

func TestDisabledSNXConnectionCannotStopAnotherActiveDaemon(t *testing.T) {
	disabled := false
	cfg := config.Config{Upstreams: map[string]config.Upstream{"active": {Type: "snx-rs"}, "saved": {Type: "snx-rs", Enabled: &disabled}}}
	results := (SNXRSUpstream{}).Stop("saved", cfg.Upstreams["saved"], ApplyContext{Config: cfg, Runner: runner.NewDryRun()})
	if len(results) != 1 || results[0].Status != "shared" || results[0].Command != "" {
		t.Fatal(results)
	}
}
