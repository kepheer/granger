package config

import "testing"

func TestUserAccessDistinguishesUnrestrictedFromDenyAll(t *testing.T) {
	cfg := Default("", "eth0", "192.0.2.1")
	cfg.Users = map[string]User{"legacy": {}, "blocked": {AccessRestricted: true}, "limited": {AccessRestricted: true, AllowedUpstreams: []string{"direct"}}, "revoked": {Disabled: true}}
	for _, check := range []struct {
		user, upstream string
		want           bool
	}{{"legacy", "corporate", true}, {"blocked", "direct", false}, {"limited", "direct", true}, {"limited", "corporate", false}, {"revoked", "direct", false}, {"missing", "direct", false}} {
		if got := cfg.UserCanUseUpstream(check.user, check.upstream); got != check.want {
			t.Fatalf("%s -> %s: %v", check.user, check.upstream, got)
		}
	}
}
