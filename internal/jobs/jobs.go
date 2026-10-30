package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"granger/pkg/runner"
	"sort"
	"sync"
	"time"
)

type Job struct {
	ID       string          `json:"id"`
	Resource string          `json:"resource"`
	Action   string          `json:"action"`
	Module   string          `json:"module"`
	State    string          `json:"state"`
	Started  time.Time       `json:"started"`
	Finished *time.Time      `json:"finished,omitempty"`
	Results  []runner.Result `json:"results,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type entry struct {
	job    Job
	cancel context.CancelFunc
}
type Manager struct {
	mu      sync.Mutex
	entries map[string]*entry
}

func New() *Manager { return &Manager{entries: map[string]*entry{}} }

func (m *Manager) Start(resource, action, module string, run func(context.Context) ([]runner.Result, error)) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range m.entries {
		if item.job.Resource == resource && item.job.State == "running" {
			return Job{}, errors.New("another operation is using this resource")
		}
	}
	if len(m.entries) >= 100 {
		var oldest *entry
		for _, item := range m.entries {
			if item.job.State != "running" && (oldest == nil || item.job.Started.Before(oldest.job.Started)) {
				oldest = item
			}
		}
		if oldest != nil {
			delete(m.entries, oldest.job.ID)
		}
	}
	var key [16]byte
	if _, err := rand.Read(key[:]); err != nil {
		return Job{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	item := &entry{job: Job{ID: hex.EncodeToString(key[:]), Resource: resource, Action: action, Module: module, State: "running", Started: time.Now()}, cancel: cancel}
	m.entries[item.job.ID] = item
	initial := item.job
	go func() {
		results, err := run(ctx)
		state := "succeeded"
		message := ""
		if err != nil {
			state = "failed"
			message = err.Error()
		}
		for _, result := range results {
			if result.OK == false {
				state = "failed"
			}
		}
		if ctx.Err() != nil {
			state = "cancelled"
			message = ctx.Err().Error()
		}
		finished := time.Now()
		m.mu.Lock()
		item.job.State, item.job.Error, item.job.Results, item.job.Finished = state, message, results, &finished
		m.mu.Unlock()
		cancel()
	}()
	return initial, nil
}

func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Job, 0, len(m.entries))
	for _, item := range m.entries {
		job := item.job
		job.Results = append([]runner.Result(nil), job.Results...)
		out = append(out, job)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out
}

func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, exists := m.entries[id]
	if exists == false || item.job.State != "running" {
		return false
	}
	item.cancel()
	return true
}

func (m *Manager) Removing(module string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range m.entries {
		if item.job.Module == module && item.job.Action == "uninstall" && item.job.State == "running" {
			return true
		}
	}
	return false
}
