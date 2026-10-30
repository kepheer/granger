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

func TestStaleSessionCannotSubmitOrCancelCurrentAuthentication(t *testing.T) {
	manager := New(runner.NewDryRun())
	manager.sessionID = "current"
	if manager.CancelSession("stale") || manager.SubmitSessionAsync("stale", nil, nil) {
		t.Fatal("stale action accepted")
	}
	if manager.PendingStatus().SessionID != "current" {
		t.Fatal("stale action changed the current session")
	}
}
