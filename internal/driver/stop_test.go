package driver

import (
	"granger/pkg/runner"
	"os"
	"path/filepath"
	"testing"
)

func TestStoppingMissingUnitsIsIdempotentWithoutHidingRealFailures(t *testing.T) {
	for _, test := range []struct {
		name, state, code string
		ok, called        bool
	}{{"missing", "not-found", "1", true, false}, {"failure", "loaded", "1", false, true}, {"success", "loaded", "0", true, true}} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			script := "#!/bin/sh\nif [ \"$1\" = show ]; then printf '%s\\n' \"$GRANGER_TEST_UNIT_STATE\"; exit 0; fi\nprintf 'stop\\n' > \"$GRANGER_TEST_STOP_LOG\"\nexit \"$GRANGER_TEST_STOP_CODE\"\n"
			if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(dir, "stop.log")
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GRANGER_TEST_UNIT_STATE", test.state)
			t.Setenv("GRANGER_TEST_STOP_CODE", test.code)
			t.Setenv("GRANGER_TEST_STOP_LOG", log)
			result := systemctlWithFallback(ApplyContext{Runner: runner.New()}, "Stop test", "stop", "granger-test.service", "")
			_, err := os.Stat(log)
			if result.OK != test.ok || (err == nil) != test.called {
				t.Fatalf("result=%+v, stop log error=%v", result, err)
			}
		})
	}
}
