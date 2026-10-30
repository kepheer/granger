package jobs

import (
	"context"
	"errors"
	"granger/pkg/runner"
	"testing"
	"time"
)

func TestJobsSerializeResourcesAndSupportCancellation(t *testing.T) {
	manager := New()
	entered := make(chan struct{})
	job, err := manager.Start("packages", "install", "example", func(ctx context.Context) ([]runner.Result, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err := manager.Start("packages", "install", "other", func(context.Context) ([]runner.Result, error) { return nil, nil }); err == nil {
		t.Fatal("concurrent package mutation accepted")
	}
	if manager.Cancel(job.ID) == false {
		t.Fatal("cancel failed")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		for _, item := range manager.List() {
			if item.ID == job.ID && item.State == "cancelled" {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("operation did not finish after cancellation")
}

func TestFailedJobReleasesResource(t *testing.T) {
	manager := New()
	_, err := manager.Start("packages", "uninstall", "example", func(context.Context) ([]runner.Result, error) { return nil, errors.New("failure") })
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if manager.Removing("example") == false {
			if manager.List()[0].State != "failed" {
				t.Fatal(manager.List())
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("failed operation retained its resource")
}
