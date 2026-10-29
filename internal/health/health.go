package health

import (
	"granger/internal/driver"
	"granger/internal/engine"
	"strconv"
	"strings"
	"time"

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
	for _, status := range engine.New(c.Runner).Runtime(cfg) {
		if out, ok := cfg.Outputs[status.Name]; ok && out.IsEnabled() == false {
			continue
		}
		state := Degraded
		if status.State == driver.StateHealthy {
			state = Healthy
		}
		if status.State == driver.StateBroken {
			state = Broken
		}
		checks = append(checks, Check{Name: status.Type + ":" + status.Name, State: state, Summary: status.Summary, Expected: "enabled tunnel is operational"})
	}
	dns := c.Runner.Run("DNS service", 5*time.Second, nil, "systemctl", "is-active", "dnsmasq.service")
	dnsState := Broken
	if dns.OK {
		dnsState = Healthy
	}
	checks = append(checks, Check{Name: "dnsmasq", State: dnsState, Summary: dns.Output, Expected: "active DNS service"})
	forwarding := c.Runner.Run("IPv4 forwarding", 5*time.Second, nil, "sysctl", "-n", "net.ipv4.ip_forward")
	forwardState := Broken
	if forwarding.OK && (strings.TrimSpace(forwarding.Output) == "1" || c.Runner.DryRun) {
		forwardState = Healthy
	}
	checks = append(checks, Check{Name: "ip_forward", State: forwardState, Summary: forwarding.Output, Expected: "IPv4 forwarding is enabled"})
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
