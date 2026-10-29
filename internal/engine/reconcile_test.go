package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"granger/internal/config"
	"granger/internal/driver"
	"granger/pkg/runner"
)

type reconcileUpstream struct {
	driver.SNXRSUpstream
	events *[]string
	kind   string
}

func (d reconcileUpstream) Type() string {
	if d.kind != "" {
		return d.kind
	}
	return "snx-rs"
}

func (d reconcileUpstream) NormalizeConfig(string, config.Upstream, driver.ApplyContext) []runner.Result {
	return nil
}

func (d reconcileUpstream) Start(name string, _ config.Upstream, _ driver.ApplyContext) []runner.Result {
	*d.events = append(*d.events, "start upstream "+name)
	return nil
}

func (d reconcileUpstream) Stop(name string, _ config.Upstream, _ driver.ApplyContext) []runner.Result {
	*d.events = append(*d.events, "stop upstream "+name)
	return nil
}

func (d reconcileUpstream) Restart(name string, _ config.Upstream, _ driver.ApplyContext) []runner.Result {
	*d.events = append(*d.events, "restart upstream "+name)
	return nil
}

func (d reconcileUpstream) Status(name string, _ config.Upstream, _ driver.ApplyContext) driver.RuntimeStatus {
	return driver.RuntimeStatus{Name: name, State: driver.StateHealthy}
}

type reconcileOutput struct {
	driver.WireGuardOutput
	events *[]string
	kind   string
}

func (d reconcileOutput) Type() string {
	if d.kind != "" {
		return d.kind
	}
	return "wireguard"
}

func (d reconcileOutput) GenerateServerConfig(string, config.Output, driver.ApplyContext) []runner.Result {
	return nil
}

func (d reconcileOutput) Start(name string, _ config.Output, _ driver.ApplyContext) []runner.Result {
	*d.events = append(*d.events, "start output "+name)
	return nil
}

func (d reconcileOutput) Stop(name string, _ config.Output, _ driver.ApplyContext) []runner.Result {
	*d.events = append(*d.events, "stop output "+name)
	return nil
}

func (d reconcileOutput) Restart(name string, _ config.Output, _ driver.ApplyContext) []runner.Result {
	*d.events = append(*d.events, "restart output "+name)
	return nil
}

func reconcileFixture(t *testing.T) (Engine, config.Config, *[]string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tunnel.conf")
	if err := os.WriteFile(path, []byte("test config"), 0600); err != nil {
		t.Fatal(err)
	}
	var events []string
	registry := driver.NewRegistry()
	registry.RegisterUpstream(reconcileUpstream{events: &events})
	registry.RegisterOutput(reconcileOutput{events: &events})
	registry.RegisterUpstream(reconcileUpstream{events: &events, kind: "other-upstream"})
	registry.RegisterOutput(reconcileOutput{events: &events, kind: "other-output"})
	if err := os.WriteFile(path+".new", []byte("new config"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default("192.0.2.2", "eth0", "192.0.2.1")
	cfg.Outputs["clients"] = config.Output{Type: "wireguard", Interface: "wg0", Config: path, Subnet: "10.66.0.0/24"}
	cfg.Upstreams["office"] = config.Upstream{Type: "snx-rs", Interface: "tun0", Config: path}
	return NewWithRegistry(runner.NewDryRun(), registry), cfg, &events
}

func cloneReconcileConfig(cfg config.Config) config.Config {
	copy := cfg
	copy.Outputs = map[string]config.Output{}
	copy.Upstreams = map[string]config.Upstream{}
	copy.Users = map[string]config.User{}
	for name, out := range cfg.Outputs {
		copy.Outputs[name] = out
	}
	for name, up := range cfg.Upstreams {
		copy.Upstreams[name] = up
	}
	for name, user := range cfg.Users {
		copy.Users[name] = user
	}
	return copy
}

func TestReconcilePreservesSessionsForRouteAndAccessChanges(t *testing.T) {
	e, previous, events := reconcileFixture(t)
	next := cloneReconcileConfig(previous)
	next.Rules = []config.Rule{{Name: "office", CIDRs: []string{"192.0.2.0/24"}, Via: "office"}}
	next.Users["alice"] = config.User{AccessRestricted: true, AllowedUpstreams: []string{"office"}}
	up := next.Upstreams["office"]
	up.DNS = []string{"192.0.2.53"}
	up.FallbackWhenDown = "pending"
	up.BlockFallback = true
	next.Upstreams["office"] = up
	plan := e.ReconcileConfig(previous, next)
	if len(*events) != 0 {
		t.Fatalf("route/access change interrupted sessions: %v", *events)
	}
	if resultsOK(plan.Results) == false {
		t.Fatalf("route/access reconciliation failed: %+v", plan.Results)
	}
}

func TestReconcileOperationalChangesRestartResources(t *testing.T) {
	cases := []struct {
		name string
		edit func(*config.Config)
		want []string
	}{
		{"output type", func(cfg *config.Config) {
			out := cfg.Outputs["clients"]
			out.Type = "other-output"
			cfg.Outputs["clients"] = out
		}, []string{"stop output clients", "start output clients"}},
		{"output config", func(cfg *config.Config) {
			out := cfg.Outputs["clients"]
			out.Config += ".new"
			cfg.Outputs["clients"] = out
		}, []string{"stop output clients", "start output clients"}},
		{"upstream type", func(cfg *config.Config) {
			up := cfg.Upstreams["office"]
			up.Type = "other-upstream"
			cfg.Upstreams["office"] = up
		}, []string{"stop upstream office", "start upstream office"}},
		{"upstream config", func(cfg *config.Config) {
			up := cfg.Upstreams["office"]
			up.Config += ".new"
			cfg.Upstreams["office"] = up
		}, []string{"stop upstream office", "start upstream office"}},
		{"output interface", func(cfg *config.Config) {
			out := cfg.Outputs["clients"]
			out.Interface = "wg1"
			cfg.Outputs["clients"] = out
		}, []string{"stop output clients", "start output clients"}},
		{"output service", func(cfg *config.Config) {
			out := cfg.Outputs["clients"]
			out.Service = "custom.service"
			cfg.Outputs["clients"] = out
		}, []string{"stop output clients", "start output clients"}},
		{"output protocol", func(cfg *config.Config) {
			out := cfg.Outputs["clients"]
			out.ListenPort = 51821
			cfg.Outputs["clients"] = out
		}, []string{"stop output clients", "start output clients"}},
		{"upstream interface", func(cfg *config.Config) {
			up := cfg.Upstreams["office"]
			up.Interface = "tun1"
			cfg.Upstreams["office"] = up
		}, []string{"stop upstream office", "start upstream office"}},
		{"upstream service", func(cfg *config.Config) {
			up := cfg.Upstreams["office"]
			up.Service = "new.service"
			cfg.Upstreams["office"] = up
		}, []string{"stop upstream office", "start upstream office"}},
		{"new output", func(cfg *config.Config) { cfg.Outputs["other"] = cfg.Outputs["clients"] }, []string{"start output other"}},
		{"new upstream", func(cfg *config.Config) { cfg.Upstreams["other"] = cfg.Upstreams["office"] }, []string{"start upstream other"}},
		{"remove", func(cfg *config.Config) { delete(cfg.Outputs, "clients"); delete(cfg.Upstreams, "office") }, []string{"stop output clients", "stop upstream office"}},
		{"disable", func(cfg *config.Config) {
			disabled := false
			out := cfg.Outputs["clients"]
			out.Enabled = &disabled
			cfg.Outputs["clients"] = out
			up := cfg.Upstreams["office"]
			up.Enabled = &disabled
			cfg.Upstreams["office"] = up
		}, []string{"stop output clients", "stop upstream office"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, previous, events := reconcileFixture(t)
			next := cloneReconcileConfig(previous)
			tc.edit(&next)
			e.ReconcileConfig(previous, next)
			if reflect.DeepEqual(*events, tc.want) == false {
				t.Fatalf("lifecycle = %v, want %v", *events, tc.want)
			}
		})
	}
}

func TestReconcileEnablesPreviouslyDisabledResources(t *testing.T) {
	e, next, events := reconcileFixture(t)
	previous := cloneReconcileConfig(next)
	disabled := false
	out := previous.Outputs["clients"]
	out.Enabled = &disabled
	previous.Outputs["clients"] = out
	up := previous.Upstreams["office"]
	up.Enabled = &disabled
	previous.Upstreams["office"] = up
	e.ReconcileConfig(previous, next)
	want := []string{"start output clients", "start upstream office"}
	if reflect.DeepEqual(*events, want) == false {
		t.Fatalf("lifecycle = %v, want %v", *events, want)
	}
}

func TestReconcileRejectsMissingConfigsBeforeStarting(t *testing.T) {
	for _, path := range []string{"", "/nonexistent/granger-reconcile.conf"} {
		t.Run(path, func(t *testing.T) {
			e, next, events := reconcileFixture(t)
			out := next.Outputs["clients"]
			out.Config = path
			next.Outputs["clients"] = out
			up := next.Upstreams["office"]
			up.Config = path
			next.Upstreams["office"] = up
			plan := e.ReconcileConfig(config.Default("192.0.2.2", "eth0", "192.0.2.1"), next)
			if len(*events) != 0 {
				t.Fatalf("started resources with missing config: %v", *events)
			}
			if resultsOK(plan.Results) {
				t.Fatal("missing config silently accepted")
			}
		})
	}
}
