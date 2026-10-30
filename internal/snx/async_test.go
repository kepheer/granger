package snx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"granger/internal/config"
	"granger/pkg/runner"
)

func waitAsync(t *testing.T, manager *Manager) PendingStatus {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		status := manager.PendingStatus()
		if status.Busy == false {
			return status
		}
		select {
		case <-deadline:
			t.Fatal("authentication did not finish")
		case <-ticker.C:
		}
	}
}

func TestAsyncAuthenticationReturnsImmediatelyAndRejectsDuplicates(t *testing.T) {
	manager := New(runner.New())
	release := make(chan struct{})
	defer close(release)
	started := make(chan bool, 1)
	run := func(ctx context.Context) runner.Result {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return runner.Result{OK: false, Output: "Gateway did not respond", Status: "error"}
	}
	go func() { started <- manager.runAsync(run, nil) }()
	select {
	case accepted := <-started:
		if accepted == false || manager.PendingStatus().Busy == false {
			t.Fatal("operation was not accepted as busy")
		}
	case <-time.After(time.Second):
		t.Fatal("start blocked on authentication")
	}
	if manager.runAsync(run, nil) {
		t.Fatal("duplicate operation accepted")
	}
	manager.Cancel()
	if manager.PendingStatus().Busy {
		t.Fatal("cancel left authentication busy")
	}
}

func TestAsyncFailureIsAvailableToPollingAndDoesNotReconcile(t *testing.T) {
	manager := New(runner.New())
	manager.runAsync(func(context.Context) runner.Result {
		return runner.Result{OK: false, Status: "error", Output: "Gateway did not respond"}
	}, func() []runner.Result {
		t.Error("failed authentication changed routes")
		return nil
	})
	status := waitAsync(t, manager)
	if status.Result == nil || status.Result.OK || status.Result.Output != "Gateway did not respond" {
		t.Fatalf("failure unavailable to polling: %#v", status)
	}
}

func TestCanceledOperationCannotOverwriteNewResult(t *testing.T) {
	manager := New(runner.New())
	finished := make(chan struct{})
	manager.runAsync(func(ctx context.Context) runner.Result {
		<-ctx.Done()
		defer close(finished)
		return runner.Result{OK: false, Output: "old", Status: "error"}
	}, nil)
	manager.Cancel()
	manager.runAsync(func(context.Context) runner.Result {
		return runner.Result{OK: true, Output: "new", Status: "ok"}
	}, nil)
	<-finished
	status := waitAsync(t, manager)
	if status.Result == nil || status.Result.Output != "new" {
		t.Fatalf("canceled result replaced new operation: %#v", status)
	}
}

func TestAsyncSuccessWaitsForRoutingAndReportsRoutingFailure(t *testing.T) {
	manager := New(runner.New())
	manager.runAsync(func(context.Context) runner.Result {
		return runner.Result{OK: true, Status: "ok"}
	}, func() []runner.Result {
		return []runner.Result{{OK: false, Output: "Could not restore routes", Status: "error"}}
	})
	status := waitAsync(t, manager)
	if status.Result == nil || status.Result.OK || status.Result.Output != "Could not restore routes" {
		t.Fatalf("routing failure hidden: %#v", status)
	}
}

func TestAsyncSNXStartAndSubmitKeepInteractiveProcess(t *testing.T) {
	dir := t.TempDir()
	cli := "#!/bin/sh\nif [ \"$3\" = disconnect ]; then exit 0; fi\nprintf 'Password:'\nread password\nprintf 'Verification Code:'\nread code\nprintf 'Connected'\n"
	if err := os.WriteFile(filepath.Join(dir, "snxctl"), []byte(cli), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vpn.conf")
	if err := os.WriteFile(path, []byte("server-name = example.test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	manager := New(runner.New())
	defer manager.Cancel()
	up := config.Upstream{Config: path}
	if manager.StartAsync(up, map[string]string{"password": "fixture"}, nil) == false {
		t.Fatal("start rejected")
	}
	status := waitAsync(t, manager)
	if status.Pending == false || status.StepType != "sms" {
		t.Fatalf("SMS prompt unavailable: %#v", status)
	}
	if manager.SubmitAsync(map[string]string{"sms": "123456"}, nil) == false {
		t.Fatal("submit rejected")
	}
	status = waitAsync(t, manager)
	if status.Pending || status.Result == nil || status.Result.OK == false {
		t.Fatalf("could not complete interactive sign-in: %#v", status)
	}
}
