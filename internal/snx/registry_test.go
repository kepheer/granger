package snx

import (
	"granger/pkg/runner"
	"testing"
)

func TestConnectionRegistryIsolatesResultsAndCancellation(t *testing.T) {
	registry := NewRegistry(runner.NewDryRun())
	a, b := registry.Connection("one"), registry.Connection("two")
	a.lastResult = &runner.Result{Output: "private challenge"}
	b.Cancel()
	if a == b || registry.Connection("one") != a || a.PendingStatus().Result == nil || b.PendingStatus().Result != nil {
		t.Fatal("connection state is shared")
	}
}
