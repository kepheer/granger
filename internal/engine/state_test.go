package engine

import (
	"os"
	"path/filepath"
	"testing"

	"granger/internal/config"
	"granger/pkg/runner"
)

func routingStateFixture(t *testing.T) func(string) {
	t.Helper()
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	script := "#!/bin/sh\n[ -s \"$GRANGER_ROUTING_TEST_STATE\" ] || exit 1\nif [ \"$1\" = '-j' ]; then\n/bin/cat \"$GRANGER_ROUTING_TEST_STATE\"\nexit 0\nfi\nprintf '%s\\n' '7: tun0: <POINTOPOINT,UP> inet 10.44.0.2/24'\n"
	if err := os.WriteFile(filepath.Join(dir, "ip"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GRANGER_ROUTING_TEST_STATE", statePath)
	return func(body string) {
		t.Helper()
		if err := os.WriteFile(statePath, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func routingStateConfig() config.Config {
	return config.Config{
		Upstreams: map[string]config.Upstream{"corp": {Type: "snx-rs", Interface: "tun0"}},
		Rules:     []config.Rule{{Name: "corporate", CIDRs: []string{"10.44.0.0/16"}, Via: "corp"}},
		Users:     map[string]config.User{"alice": {DefaultUpstream: "corp"}},
	}
}

func TestRoutingKeyTracksSNXConnectionWithoutFallback(t *testing.T) {
	writeState := routingStateFixture(t)
	writeState("")
	e := New(runner.New())
	cfg := routingStateConfig()
	pending := e.RoutingKey(cfg)
	if pending != e.RoutingKey(cfg) {
		t.Fatal("unchanged pending state must produce a stable key")
	}
	writeState(`[{"ifindex":7,"flags":["POINTOPOINT","UP"],"operstate":"UNKNOWN","addr_info":[{"family":"inet","local":"10.44.0.2","prefixlen":24}]}]`)
	connected := e.RoutingKey(cfg)
	if connected == pending {
		t.Fatal("SNX connection without fallback must trigger reconciliation")
	}
	if connected != e.RoutingKey(cfg) {
		t.Fatal("unchanged connected state must produce a stable key")
	}
	writeState("")
	if e.RoutingKey(cfg) != pending {
		t.Fatal("SNX disconnection must restore the pending key")
	}
}

func TestRoutingKeyTracksInterfaceReplacementAndAddressChanges(t *testing.T) {
	writeState := routingStateFixture(t)
	e := New(runner.New())
	cfg := routingStateConfig()
	states := []string{
		`[{"ifindex":7,"flags":["UP"],"operstate":"UNKNOWN","addr_info":[{"family":"inet","local":"10.44.0.2","prefixlen":24}]}]`,
		`[{"ifindex":8,"flags":["UP"],"operstate":"UNKNOWN","addr_info":[{"family":"inet","local":"10.44.0.2","prefixlen":24}]}]`,
		`[{"ifindex":8,"flags":["UP"],"operstate":"UNKNOWN","addr_info":[{"family":"inet","local":"10.44.0.3","prefixlen":24}]}]`,
		`[{"ifindex":8,"flags":["UP"],"operstate":"UNKNOWN","addr_info":[{"family":"inet","local":"10.44.0.3","prefixlen":32}]}]`,
		`[{"ifindex":8,"flags":[],"operstate":"DOWN","addr_info":[{"family":"inet","local":"10.44.0.3","prefixlen":32}]}]`,
	}
	previous := ""
	for _, state := range states {
		writeState(state)
		key := e.RoutingKey(cfg)
		if key == previous {
			t.Fatal("interface replacement, address, prefix, and link state changes must trigger reconciliation")
		}
		previous = key
	}
}

func TestRoutingKeyIgnoresAddressOrderingAndDynamicMetadata(t *testing.T) {
	writeState := routingStateFixture(t)
	e := New(runner.New())
	cfg := routingStateConfig()
	writeState(`[{"ifindex":7,"flags":["UP","POINTOPOINT"],"operstate":"UNKNOWN","stats64":{"rx":{"bytes":100}},"addr_info":[{"family":"inet","local":"10.44.0.2","prefixlen":24,"valid_life_time":900},{"family":"inet","local":"10.45.0.2","prefixlen":24,"preferred_life_time":800}]}]`)
	key := e.RoutingKey(cfg)
	writeState(`[{"ifindex":7,"flags":["POINTOPOINT","UP"],"operstate":"UNKNOWN","stats64":{"rx":{"bytes":999}},"addr_info":[{"family":"inet","local":"10.45.0.2","prefixlen":24,"preferred_life_time":799},{"family":"inet","local":"10.44.0.2","prefixlen":24,"valid_life_time":899}]}]`)
	if key != e.RoutingKey(cfg) {
		t.Fatal("address ordering, lifetimes, and traffic counters must not trigger reconciliation")
	}
}

func TestRoutingKeyTracksOutputInterfaceChanges(t *testing.T) {
	writeState := routingStateFixture(t)
	e := New(runner.New())
	cfg := config.Config{Outputs: map[string]config.Output{"clients": {Type: "wireguard", Interface: "wg0"}}}
	writeState(`[{"ifindex":9,"flags":["UP"],"operstate":"UNKNOWN","addr_info":[{"family":"inet","local":"10.66.0.1","prefixlen":24}]}]`)
	key := e.RoutingKey(cfg)
	writeState(`[{"ifindex":10,"flags":["UP"],"operstate":"UNKNOWN","addr_info":[{"family":"inet","local":"10.66.0.1","prefixlen":24}]}]`)
	if key == e.RoutingKey(cfg) {
		t.Fatal("output interface recreation must trigger reconciliation")
	}
}
