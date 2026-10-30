package driver

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"granger/internal/config"
	"granger/pkg/runner"
)

func TestSNXDNSOnlyUsesValidAdvertisedIPv4Servers(t *testing.T) {
	got := parseSNXDNS("Server name: 203.0.113.10\nDNS servers: [192.0.2.53, bad, ::1, 192.0.2.54, 192.0.2.53]\n")
	if reflect.DeepEqual(got, []string{"192.0.2.53", "192.0.2.54"}) == false {
		t.Fatalf("DNS = %v", got)
	}
}

func TestSNXDomainRulesHaveTunnelDefaultRoute(t *testing.T) {
	for _, domains := range [][]string{{"*.example.test"}, nil} {
		cfg := config.Config{Rules: []config.Rule{{Via: "office", Domains: domains}}}
		results := (SNXRSUpstream{}).ApplyRoutes("office", config.Upstream{Interface: "tunnel9", DNS: []string{"192.0.2.53"}}, ApplyContext{Config: cfg, Runner: runner.NewDryRun()})
		found := false
		for _, result := range results {
			if strings.Contains(result.Command, "route replace default dev tunnel9") {
				found = true
			}
		}
		if found != (len(domains) > 0) {
			t.Fatalf("default route present = %v for domains %v", found, domains)
		}
	}
}

func TestSNXDomainDNSUsesActiveSessionWithoutOverridingExplicitDNS(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"ip": "#!/bin/sh\nprintf '1: custom0 inet 192.0.2.2/32\\n'\n", "snxctl": "#!/bin/sh\nprintf 'DNS servers: [192.0.2.53]\\n'\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, explicit := range [][]string{nil, {"192.0.2.99"}} {
		up := config.Upstream{Interface: "custom0", Config: filepath.Join(dir, "custom config.conf"), DNS: explicit}
		got := (SNXRSUpstream{}).RenderDNS("office", up, []config.Rule{{Via: "office", Domains: []string{"*.example.test"}}}, ApplyContext{Runner: runner.New()})
		want := "192.0.2.53"
		if len(explicit) > 0 {
			want = explicit[0]
		}
		if len(got) != 1 || got[0].Server != want || got[0].Domain != "example.test" {
			t.Fatalf("DNS rules = %+v", got)
		}
	}
}
