package webgui

import (
	"context"
	"granger/pkg/lockfile"
	"path/filepath"
)

func lockRouting(ctx context.Context) (func(), error) {
	return lockfile.Acquire(ctx, filepath.Join(filepath.Dir(configPath), ".routing.lock"))
}
