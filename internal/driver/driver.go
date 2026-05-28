package driver

import (
	"fmt"
	"sort"

	"granger/internal/config"
	"granger/pkg/runner"
)

type Capability string

const (
	CapDefaultRoute Capability = "default-route"
	CapDomainRoute  Capability = "domain-route"
	CapCIDRRoute    Capability = "cidr-route"
	CapInteractive  Capability = "interactive"
	CapClientConfig Capability = "client-config"
)

type RuntimeState string

const (
	StateHealthy  RuntimeState = "healthy"
	StateDegraded RuntimeState = "degraded"
	StateBroken   RuntimeState = "broken"
	StatePending  RuntimeState = "pending"
	StateUnknown  RuntimeState = "unknown"
)

type RuntimeStatus struct {
	Name         string
	Type         string
	State        RuntimeState
	Summary      string
	Interface    string
	Service      string
	Capabilities []Capability
	Results      []runner.Result
}

type ApplyContext struct {
	Config      config.Config
	Output      config.Output
	OutputName  string
	ClientCIDR  string
	Runner      runner.Runner
	PublicIP    string
	UplinkIF    string
	UplinkGW    string
	RouteTables map[string]string
}

type DNSRule struct {
	Domain string
	Server string
	Set    string
}

type UpstreamDriver interface {
	Type() string
	DisplayName() string
	Capabilities() []Capability
	NormalizeConfig(name string, up config.Upstream, ctx ApplyContext) []runner.Result
	Start(name string, up config.Upstream, ctx ApplyContext) []runner.Result
	Stop(name string, up config.Upstream, ctx ApplyContext) []runner.Result
	Restart(name string, up config.Upstream, ctx ApplyContext) []runner.Result
	Status(name string, up config.Upstream, ctx ApplyContext) RuntimeStatus
	ApplyRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result
	ApplyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result
	RenderDNS(name string, up config.Upstream, rules []config.Rule, ctx ApplyContext) []DNSRule
}

type OutputDriver interface {
	Type() string
	DisplayName() string
	Capabilities() []Capability
	GenerateServerConfig(name string, out config.Output, ctx ApplyContext) []runner.Result
	GenerateClientConfig(name string, out config.Output, ctx ApplyContext) (string, []runner.Result)
	Start(name string, out config.Output, ctx ApplyContext) []runner.Result
	Stop(name string, out config.Output, ctx ApplyContext) []runner.Result
	Restart(name string, out config.Output, ctx ApplyContext) []runner.Result
	Status(name string, out config.Output, ctx ApplyContext) RuntimeStatus
	ApplyIngressFirewall(name string, out config.Output, ctx ApplyContext) []runner.Result
}

type Registry struct {
	upstreams map[string]UpstreamDriver
	outputs   map[string]OutputDriver
}

func NewRegistry() *Registry {
	return &Registry{upstreams: map[string]UpstreamDriver{}, outputs: map[string]OutputDriver{}}
}

func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.RegisterUpstream(DirectUpstream{})
	r.RegisterUpstream(InterfaceUpstream{})
	r.RegisterOutput(WireGuardOutput{})
	r.RegisterOutput(AmneziaWGOutput{})
	r.RegisterOutput(OpenVPNOutput{})
	r.RegisterOutput(SingBoxOutput{})
	r.RegisterOutput(XrayOutput{})
	r.RegisterUpstream(WireGuardUpstream{})
	r.RegisterUpstream(AmneziaWGUpstream{})
	r.RegisterUpstream(OpenVPNUpstream{})
	r.RegisterUpstream(SingBoxUpstream{})
	r.RegisterUpstream(XrayUpstream{})
	return r
}

func (r *Registry) RegisterUpstream(d UpstreamDriver) {
	r.upstreams[d.Type()] = d
}

func (r *Registry) RegisterOutput(d OutputDriver) {
	r.outputs[d.Type()] = d
}

func (r *Registry) Upstream(t string) (UpstreamDriver, error) {
	d, ok := r.upstreams[t]
	if !ok {
		return nil, fmt.Errorf("unknown upstream driver: %s", t)
	}
	return d, nil
}

func (r *Registry) Output(t string) (OutputDriver, error) {
	d, ok := r.outputs[t]
	if !ok {
		return nil, fmt.Errorf("unknown output driver: %s", t)
	}
	return d, nil
}

func (r *Registry) UpstreamTypes() []string {
	var out []string
	for k := range r.upstreams {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) OutputTypes() []string {
	var out []string
	for k := range r.outputs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func BaseContext(cfg config.Config, rr runner.Runner) ApplyContext {
	outName := ""
	var out config.Output
	names := make([]string, 0, len(cfg.Outputs))
	for name := range cfg.Outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 0 {
		outName = names[0]
		out = cfg.Outputs[outName]
	}
	return ApplyContext{
		Config: cfg, Output: out, OutputName: outName, ClientCIDR: out.Subnet, Runner: rr,
		PublicIP: cfg.Server.PublicIP, UplinkIF: cfg.Server.UplinkIF, UplinkGW: cfg.Server.UplinkGW,
		RouteTables: map[string]string{},
	}
}
