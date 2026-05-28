package driver

import (
	"os"
	"path/filepath"
	"testing"

	"granger/internal/config"
	"granger/pkg/runner"
)

func TestGenerateClientConfigSkipsDisabledUser(t *testing.T) {
	dir := t.TempDir()
	disabledPath := filepath.Join(dir, "disabled.conf")
	enabledPath := filepath.Join(dir, "enabled.conf")
	if err := os.WriteFile(disabledPath, []byte("disabled"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(enabledPath, []byte("enabled"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Users: map[string]config.User{
			"blocked": {Disabled: true},
			"active":  {},
		},
	}
	out := config.Output{
		Clients: []config.Client{
			{Name: "blocked-phone", User: "blocked", Config: disabledPath},
			{Name: "active-phone", User: "active", Config: enabledPath},
		},
	}

	body, res := WireGuardOutput{}.GenerateClientConfig("home", out, ApplyContext{Config: cfg, Runner: runner.New()})
	if len(res) != 0 {
		t.Fatalf("unexpected result: %#v", res)
	}
	if body != "enabled" {
		t.Fatalf("client config = %q, want enabled", body)
	}
}
