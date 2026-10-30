package snx

import (
	"context"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/runner"
)

type authOperation struct {
	cancel context.CancelFunc
}

func (m *Manager) StartAsync(up config.Upstream, inputs map[string]string, reconcile func() []runner.Result) bool {
	return m.runAsync(func(ctx context.Context) runner.Result {
		m.mu.Lock()
		pending := m.pending != nil
		m.mu.Unlock()
		if pending {
			return runner.Result{Title: "SNX-RS sign-in", Output: "Finish or cancel the current sign-in first", OK: false, Status: "error"}
		}
		return m.start(ctx, up, inputs)
	}, reconcile)
}

func (m *Manager) SubmitAsync(inputs map[string]string, reconcile func() []runner.Result) bool {
	return m.runAsync(func(context.Context) runner.Result {
		return m.Submit(inputs)
	}, reconcile)
}

func (m *Manager) runAsync(run func(context.Context) runner.Result, reconcile func() []runner.Result) bool {
	m.mu.Lock()
	if m.operation != nil {
		m.mu.Unlock()
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	op := &authOperation{cancel: cancel}
	m.operation = op
	m.lastResult = nil
	m.mu.Unlock()

	go func() {
		result := run(ctx)
		if result.OK && result.Status == "ok" && reconcile != nil && ctx.Err() == nil {
			for _, item := range reconcile() {
				if item.OK == false {
					result = item
					break
				}
			}
		}

		m.mu.Lock()
		if m.operation == op {
			m.lastResult = &result
			m.operation = nil
		}
		m.mu.Unlock()

		if strings.HasPrefix(result.Status, "pending:") == false {
			cancel()
		}
	}()

	return true
}
