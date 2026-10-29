package driver

import (
	"granger/pkg/runner"
	"testing"
)

func TestFirewallFailureIsNotSilenced(t *testing.T) {
	result := runShell(ApplyContext{Runner: runner.New()}, "Apply firewall", "exit 7")
	if result.OK {
		t.Fatal("failed firewall command must fail the apply plan")
	}
}
