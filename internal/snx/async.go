package snx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

		return m.start(ctx, up, inputs)
	}, reconcile)
}

func (m *Manager) SubmitAsync(inputs map[string]string, reconcile func() []runner.Result) bool {
	return m.SubmitSessionAsync(m.PendingStatus().SessionID, inputs, reconcile)
}

func (m *Manager) SubmitSessionAsync(id string, inputs map[string]string, reconcile func() []runner.Result) bool {
	return m.runAsync(func(context.Context) runner.Result { return m.Submit(inputs) }, reconcile, id)
}

func (m *Manager) runAsync(run func(context.Context) runner.Result, reconcile func() []runner.Result, expectedSession ...string) bool {
	m.mu.Lock()
	if m.operation != nil {
		m.mu.Unlock()
		return false
	}

	if len(expectedSession) > 0 {
		if expectedSession[0] == "" || expectedSession[0] != m.sessionID {
			m.mu.Unlock()
			return false
		}
	}
	if len(expectedSession) == 0 {
		if m.pending != nil {
			m.mu.Unlock()
			return false
		}
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			m.mu.Unlock()
			return false
		}
		m.sessionID = hex.EncodeToString(id[:])
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
