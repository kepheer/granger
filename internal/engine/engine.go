package engine

import (
	"fmt"

	"granger/internal/config"
	"granger/internal/driver"
	"granger/pkg/runner"
)

type ApplyPlan struct {
	DNSRules []driver.DNSRule
	Results  []runner.Result
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
	for name, out := range cfg.Outputs {
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
	for name, up := range cfg.Upstreams {
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			res = append(res, fail("Upstream driver "+name, err))
			continue
		}
		res = append(res, d.NormalizeConfig(name, up, ctx)...)
		for outName, out := range cfg.Outputs {
			outCtx := ctx
			outCtx.OutputName = outName
			outCtx.Output = out
			outCtx.ClientCIDR = out.Subnet
			res = append(res, d.ApplyFirewall(name, up, outCtx)...)
		}
	}
	for name, up := range cfg.Upstreams {
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			res = append(res, fail("Route driver "+name, err))
			continue
		}
		for outName, out := range cfg.Outputs {
			outCtx := ctx
			outCtx.OutputName = outName
			outCtx.Output = out
			outCtx.ClientCIDR = out.Subnet
			res = append(res, d.ApplyRoutes(name, up, outCtx)...)
		}
	}
	return ApplyPlan{DNSRules: e.DNSRules(cfg, ctx), Results: res}
}

func (e Engine) DNSRules(cfg config.Config, ctx driver.ApplyContext) []driver.DNSRule {
	var out []driver.DNSRule
	for name, up := range cfg.Upstreams {
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
		for n := range cfg.Outputs {
			name = n
			break
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
		for n := range cfg.Upstreams {
			name = n
			break
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
	for name, o := range cfg.Outputs {
		d, err := e.Registry.Output(o.Type)
		if err != nil {
			out = append(out, driver.RuntimeStatus{Name: name, Type: o.Type, State: driver.StateBroken, Summary: err.Error()})
			continue
		}
		out = append(out, d.Status(name, o, ctx))
	}
	for name, up := range cfg.Upstreams {
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
