package health

import (
	"strconv"

	"granger/internal/config"
	"granger/pkg/runner"
)

type State string

const (
	Healthy  State = "healthy"
	Degraded State = "degraded"
	Broken   State = "broken"
	Pending  State = "pending"
)

type Check struct {
	Name     string
	State    State
	Summary  string
	Command  string
	Output   string
	Expected string
	Fix      string
}

type RuntimeStatus struct {
	Overall State
	Checks  []Check
}

type Checker struct{ Runner runner.Runner }

func New(r runner.Runner) Checker { return Checker{Runner: r} }

func (c Checker) Check(cfg config.Config) RuntimeStatus {
	checks := []Check{}
	if err := cfg.Validate(); err != nil {
		checks = append(checks, Check{
			Name:     "config",
			State:    Broken,
			Summary:  "Config validation failed",
			Command:  "config.Validate",
			Output:   err.Error(),
			Expected: "valid declarative config",
			Fix:      "Fix /etc/granger/granger.yaml and rerun the command.",
		})
	} else {
		checks = append(checks, Check{
			Name:     "config",
			State:    Healthy,
			Summary:  "Config is valid",
			Command:  "config.Validate",
			Output:   "outputs: " + strconv.Itoa(len(cfg.Outputs)) + ", upstreams: " + strconv.Itoa(len(cfg.Upstreams)) + ", rules: " + strconv.Itoa(len(cfg.Rules)),
			Expected: "valid declarative config",
			Fix:      "",
		})
	}
	overall := Healthy
	for _, ch := range checks {
		if ch.State == Broken {
			overall = Broken
			break
		}
		if ch.State == Pending || ch.State == Degraded {
			overall = Degraded
		}
	}
	return RuntimeStatus{Overall: overall, Checks: checks}
}
