package engine

import (
	"granger/internal/config"
	"granger/pkg/runner"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersonalDefaultIsIdempotentWhenKernelOmitsHostPrefix(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "rule")
	t.Setenv("GRANGER_RULE_TEST_STATE", state)
	script := "#!/bin/sh\nif [ \"$2\" = show ]; then [ ! -f \"$GRANGER_RULE_TEST_STATE\" ] || cat \"$GRANGER_RULE_TEST_STATE\"; exit 0; fi\nif [ \"$2\" = add ]; then [ ! -f \"$GRANGER_RULE_TEST_STATE\" ] || exit 1; printf '30000: from 10.19.84.2 lookup 25283\\n' > \"$GRANGER_RULE_TEST_STATE\"; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "ip"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cfg := config.Config{Users: map[string]config.User{"user": {DefaultUpstream: "main_wg"}}, Upstreams: map[string]config.Upstream{"main_wg": {Type: "interface", Interface: "exit0"}}, Outputs: map[string]config.Output{"vpn": {Type: "wireguard", Interface: "wg0", Clients: []config.Client{{Name: "one", User: "user", IP: "10.19.84.2/32"}}}}}
	e := New(runner.New())
	for i := 0; i < 2; i++ {
		if results := e.applyUserDefaultRoutes(cfg); resultsOK(results) == false {
			t.Fatalf("apply %d failed for a normalized existing rule: %+v", i, results)
		}
	}
}

func TestGuardHandlesDetachedInterfaceRulesIdempotently(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$2\" = show ]; then printf '50: from all iif wg-missing [detached] lookup 2147483000\\n';exit 0;fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "ip"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	result := runner.New().Run("Guard", time.Second, nil, "sh", "-c", guardRule("iif wg-missing"))
	if result.OK == false {
		t.Fatal(result.Output)
	}
}
