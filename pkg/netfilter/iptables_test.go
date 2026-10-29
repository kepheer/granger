package netfilter

import (
	"strings"
	"testing"
)

func TestAccessHooksProtectInputBeforeExistingHostAcceptRules(t *testing.T) {
	script := AccessHooks()
	for _, expected := range []string{
		ShellJoin("iptables", "-N", UserInputChain),
		ShellJoin("iptables", "-I", "INPUT", "1", "-j", UserInputChain),
		ShellJoin("iptables", "-I", "FORWARD", "1", "-j", UserForwardChain),
		ShellJoin("iptables", "-I", "OUTPUT", "1", "-j", UserOutputChain),
	} {
		if strings.Contains(script, expected) == false {
			t.Fatalf("missing hook %s in %s", expected, script)
		}
	}
	if strings.Contains(ResetScript(), ShellJoin("iptables", "-F", UserInputChain)) {
		t.Fatal("firewall reset must preserve the live INPUT ACL until atomic replacement")
	}
}

func TestShellQuote(t *testing.T) {
	if got, want := ShellQuote("a'b"), `'a'"'"'b'`; got != want {
		t.Fatalf("ShellQuote() = %q, want %q", got, want)
	}
}

func TestProxyUserMarksFitXraySignedInt32AndPreserveRoutingBits(t *testing.T) {
	for _, user := range []string{"alice", "bob", "usr_b4b6e8907b0b", "user-with-a-long-name"} {
		mark := UserMark("proxy", user)
		if mark <= 0 || int64(mark) > 2147483647 || mark&0xffff != 0 || mark&0x40000000 == 0 {
			t.Fatalf("unsafe proxy routing mark: %x", mark)
		}
	}
}
