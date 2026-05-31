package engine

import (
	"fmt"
	"sort"

	"granger/internal/config"
	"granger/internal/dns"
	"granger/internal/driver"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

type ApplyPlan struct {
	DNSRules []driver.DNSRule
	Results  []runner.Result
	DryRun   bool
	Firewall string
}

type Engine struct {
	Runner   runner.Runner
	Registry *driver.Registry
}

func New(r runner.Runner) Engine {
	return Engine{Runner: r, Registry: driver.DefaultRegistry()}
}

func NewWithRegistry(r runner.Runner, reg *driver.Registry) Engine {
	return Engine{Runner: r, Registry: reg}
}

func (e Engine) ApplyConfig(cfg config.Config) ApplyPlan {
	ctx := driver.BaseContext(cfg, e.Runner)
	var res []runner.Result
	outputNames := sortedOutputNames(cfg)
	upstreamNames := sortedUpstreamNames(cfg)
	for _, name := range outputNames {
		out := cfg.Outputs[name]
		outCtx := ctx
		outCtx.OutputName = name
		outCtx.Output = out
		outCtx.ClientCIDR = out.Subnet
		d, err := e.Registry.Output(out.Type)
		if err != nil {
			res = append(res, fail("Output driver "+name, err))
			continue
		}
		res = append(res, d.GenerateServerConfig(name, out, outCtx)...)
		res = append(res, d.ApplyIngressFirewall(name, out, outCtx)...)
	}
	for _, name := range upstreamNames {
		up := cfg.Upstreams[name]
		if !up.IsEnabled() {
			res = append(res, runner.Result{Title: "Upstream driver " + name, Command: "config", Output: "upstream is disabled", OK: true, Status: "disabled"})
			continue
		}
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			res = append(res, fail("Upstream driver "+name, err))
			continue
		}
		res = append(res, d.NormalizeConfig(name, up, ctx)...)
		for _, outName := range outputNames {
			out := cfg.Outputs[outName]
			outCtx := ctx
			outCtx.OutputName = outName
			outCtx.Output = out
			outCtx.ClientCIDR = out.Subnet
			res = append(res, d.ApplyFirewall(name, up, outCtx)...)
		}
	}
	for _, name := range upstreamNames {
		up := cfg.Upstreams[name]
		if !up.IsEnabled() {
			continue
		}
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			res = append(res, fail("Route driver "+name, err))
			continue
		}
		for _, outName := range outputNames {
			out := cfg.Outputs[outName]
			outCtx := ctx
			outCtx.OutputName = outName
			outCtx.Output = out
			outCtx.ClientCIDR = out.Subnet
			res = append(res, d.ApplyRoutes(name, up, outCtx)...)
		}
	}
	dnsRules := e.DNSRules(cfg, ctx)
	res = append(res, dns.New(e.Runner).Apply(cfg, dnsRules)...)
	return ApplyPlan{DNSRules: dnsRules, Results: res, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
}

func (e Engine) DNSRules(cfg config.Config, ctx driver.ApplyContext) []driver.DNSRule {
	var out []driver.DNSRule
	for _, name := range sortedUpstreamNames(cfg) {
		up := cfg.Upstreams[name]
		if !up.IsEnabled() {
			continue
		}
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			continue
		}
		out = append(out, d.RenderDNS(name, up, cfg.Rules, ctx)...)
	}
	return out
}

func (e Engine) RestartOutput(name string, cfg config.Config) []runner.Result {
	if name == "" {
		names := sortedOutputNames(cfg)
		if len(names) > 0 {
			name = names[0]
		}
	}
	out, ok := cfg.Outputs[name]
	if !ok {
		return []runner.Result{fail("Restart output", fmt.Errorf("unknown output: %s", name))}
	}
	d, err := e.Registry.Output(out.Type)
	if err != nil {
		return []runner.Result{fail("Restart output", err)}
	}
	return d.Restart(name, out, driver.BaseContext(cfg, e.Runner))
}

func (e Engine) RestartUpstream(name string, cfg config.Config) []runner.Result {
	if name == "" {
		names := sortedUpstreamNames(cfg)
		if len(names) > 0 {
			name = names[0]
		}
	}
	up, ok := cfg.Upstreams[name]
	if !ok {
		return []runner.Result{fail("Restart upstream", fmt.Errorf("unknown upstream: %s", name))}
	}
	d, err := e.Registry.Upstream(up.Type)
	if err != nil {
		return []runner.Result{fail("Restart upstream", err)}
	}
	return d.Restart(name, up, driver.BaseContext(cfg, e.Runner))
}

func (e Engine) TestDomain(domain string, cfg config.Config) []runner.Result {
	return []runner.Result{
		e.Runner.Run("dig", 8_000_000_000, nil, "dig", domain, "A", "+short"),
	}
}

func (e Engine) Runtime(cfg config.Config) []driver.RuntimeStatus {
	ctx := driver.BaseContext(cfg, e.Runner)
	var out []driver.RuntimeStatus
	for _, name := range sortedOutputNames(cfg) {
		o := cfg.Outputs[name]
		d, err := e.Registry.Output(o.Type)
		if err != nil {
			out = append(out, driver.RuntimeStatus{Name: name, Type: o.Type, State: driver.StateBroken, Summary: err.Error()})
			continue
		}
		out = append(out, d.Status(name, o, ctx))
	}
	for _, name := range sortedUpstreamNames(cfg) {
		up := cfg.Upstreams[name]
		if !up.IsEnabled() {
			continue
		}
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			out = append(out, driver.RuntimeStatus{Name: name, Type: up.Type, State: driver.StateBroken, Summary: err.Error()})
			continue
		}
		out = append(out, d.Status(name, up, ctx))
	}
	return out
}

func (e Engine) SupportedDrivers() (upstreams, outputs []string) {
	return e.Registry.UpstreamTypes(), e.Registry.OutputTypes()
}

func fail(title string, err error) runner.Result {
	return runner.Result{Title: title, Command: "driver registry", Output: err.Error(), OK: false, Status: "error"}
}

func sortedOutputNames(cfg config.Config) []string {
	names := make([]string, 0, len(cfg.Outputs))
	for name := range cfg.Outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedUpstreamNames(cfg config.Config) []string {
	names := make([]string, 0, len(cfg.Upstreams))
	for name := range cfg.Upstreams {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
