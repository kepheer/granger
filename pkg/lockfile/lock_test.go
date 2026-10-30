package lockfile

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestLockSerializesIndependentCallersAndWaitCanBeCanceled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routing.lock")
	release, err := Acquire(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx, path); errors.Is(err, context.DeadlineExceeded) == false {
		t.Fatalf("concurrent caller acquired the lock: %v", err)
	}
	release()
	next, err := Acquire(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	next()
}
