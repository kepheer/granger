package classifier

import (
	"fmt"
	"granger/internal/config"
	"granger/internal/driver"
	"granger/pkg/runner"
	"time"
)

type Backend interface {
	ID() string
	Dependency() string
	Render(config.Config) ([]byte, error)
	Prepare(config.Config) []runner.Result
	Status(config.Config) driver.RuntimeStatus
}

type Registry struct{ backends map[string]Backend }

func NewRegistry() *Registry { return &Registry{backends: map[string]Backend{}} }
func DefaultRegistry(r runner.Runner) *Registry {
	registry := NewRegistry()
	registry.Register(Xray{Runner: r})
	return registry
}
func (r *Registry) Register(backend Backend) { r.backends[backend.ID()] = backend }
func (r *Registry) Find(id string) (Backend, error) {
	backend, ok := r.backends[id]
	if ok == false {
		return nil, fmt.Errorf("unsupported domain router backend: %s", id)
	}
	return backend, nil
}

func (b Xray) Status(cfg config.Config) driver.RuntimeStatus {
	result := b.Runner.Run("Domain router readiness", 5*time.Second, nil, "sh", "-c", "systemctl is-active --quiet "+Service+" && ss -H -lnt '( sport = :"+fmt.Sprint(cfg.Server.DomainRouterSettings().Port)+" )' | grep -q '127.0.0.1:' && ss -H -lnu '( sport = :"+fmt.Sprint(cfg.Server.DomainRouterSettings().Port)+" )' | grep -q '127.0.0.1:'")
	state := driver.StateBroken
	if result.OK {
		state = driver.StateHealthy
	}
	return driver.RuntimeStatus{Name: "domain-router", Type: b.ID(), Service: Service, State: state, Summary: "SNI routing backend", Results: []runner.Result{result}}
}
