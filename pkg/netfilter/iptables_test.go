package netfilter

import (
	"testing"
)

func TestShellQuote(t *testing.T) {
	if got, want := ShellQuote("a'b"), `'a'"'"'b'`; got != want {
		t.Fatalf("ShellQuote() = %q, want %q", got, want)
	}
}
