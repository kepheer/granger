package provision

import (
	"granger/internal/config"
	"testing"
)

func TestIdentityDisableDoesNotEraseIndependentProfileRevocations(t *testing.T) {
	out := config.Output{Clients: []config.Client{{Name: "active", User: "alice"}, {Name: "revoked", User: "alice", Disabled: true}, {Name: "other", User: "bob"}}}
	cfg := config.Config{Users: map[string]config.User{"alice": {Disabled: true}, "bob": {}}}
	runtime := RuntimeOutput(cfg, out)
	if runtime.Clients[0].Disabled == false || runtime.Clients[1].Disabled == false || runtime.Clients[2].Disabled {
		t.Fatal("identity access was not applied")
	}
	user := cfg.Users["alice"]
	user.Disabled = false
	cfg.Users["alice"] = user
	runtime = RuntimeOutput(cfg, out)
	if runtime.Clients[0].Disabled || runtime.Clients[1].Disabled == false || out.Clients[0].Disabled {
		t.Fatal("identity toggle changed independent profile state")
	}
}
