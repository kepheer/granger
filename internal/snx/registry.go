package snx

import (
	"sync"

	"granger/pkg/runner"
)

type Registry struct {
	mu       sync.Mutex
	managers map[string]*Manager
	runner   runner.Runner
}

func NewRegistry(r runner.Runner) *Registry {
	return &Registry{managers: map[string]*Manager{}, runner: r}
}

func (r *Registry) Connection(id string) *Manager {
	r.mu.Lock()
	defer r.mu.Unlock()

	if manager, ok := r.managers[id]; ok {
		return manager
	}

	manager := New(r.runner)
	r.managers[id] = manager
	return manager
}
