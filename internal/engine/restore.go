package engine

import (
	"fmt"
	"granger/internal/config"
	"granger/internal/driver"
	"granger/internal/provision"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
	"time"
)

func (e Engine) RestoreConfig(cfg config.Config) ApplyPlan {
	if err := e.ValidateConfig(cfg); err != nil {
		return ApplyPlan{Results: []runner.Result{fail("Validate routing", err)}, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
	}
	ctx := driver.BaseContext(cfg, e.Runner)
	var results []runner.Result
	results = append(results, e.beginRoutingUpdate(cfg)...)
	results = append(results, e.Runner.Run("Protect access before starting tunnels", 10*time.Second, nil, "sh", "-c", netfilter.AccessHooks()))
	results = append(results, e.applyUserAccess(cfg)...)
	if resultsOK(results) == false {
		return ApplyPlan{Results: results, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
	}

	for _, name := range sortedOutputNames(cfg) {
		out := cfg.Outputs[name]
		d, err := e.Registry.Output(out.Type)
		if err != nil {
			results = append(results, fail("Restore output "+name, err))
			continue
		}

		outCtx := ctx
		outCtx.OutputName = name
		outCtx.Output = out
		outCtx.ClientCIDR = out.Subnet
		if out.IsEnabled() == false {
			results = append(results, d.Stop(name, out, outCtx)...)
			continue
		}

		if out.Managed && e.Runner.DryRun == false {
			originalClients := out.Clients
			out = provision.RuntimeOutput(cfg, out)
			if err := provision.EnsureOutput(name, &out, cfg.Server.PublicIP); err != nil {
				results = append(results, fail("Regenerate managed output "+name, err))
				continue
			}
			out.Clients = originalClients
			cfg.Outputs[name] = out
			outCtx.Output = out
		}
		prepared := d.GenerateServerConfig(name, out, outCtx)
		results = append(results, prepared...)
		if resultsOK(prepared) {
			if out.Managed {
				results = append(results, d.Restart(name, out, outCtx)...)
				continue
			}
			results = append(results, d.Start(name, out, outCtx)...)
		}
	}

	for _, name := range sortedUpstreamNames(cfg) {
		up := cfg.Upstreams[name]
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			results = append(results, fail("Restore upstream "+name, err))
			continue
		}

		if up.IsEnabled() == false {
			results = append(results, d.Stop(name, up, ctx)...)
			continue
		}

		prepared := d.NormalizeConfig(name, up, ctx)
		results = append(results, prepared...)
		if resultsOK(prepared) {
			results = append(results, d.Start(name, up, ctx)...)
		}
	}

	plan := e.ApplyConfig(cfg)
	plan.Results = append(results, plan.Results...)
	return plan
}

func resultsOK(results []runner.Result) bool {
	for _, result := range results {
		if result.OK == false {
			return false
		}
	}
	return true
}

func (e Engine) StopRemoved(previous, next config.Config) []runner.Result {
	ctx := driver.BaseContext(previous, e.Runner)
	var results []runner.Result
	for _, name := range sortedOutputNames(previous) {
		out := previous.Outputs[name]
		replacement, ok := next.Outputs[name]
		if ok && replacement.Type == out.Type && replacement.Interface == out.Interface && replacement.Service == out.Service && replacement.Config == out.Config {
			continue
		}
		d, err := e.Registry.Output(out.Type)
		if err != nil {
			results = append(results, fail("Stop removed output "+name, err))
			continue
		}
		results = append(results, d.Stop(name, out, ctx)...)
	}
	for _, name := range sortedUpstreamNames(previous) {
		up := previous.Upstreams[name]
		replacement, ok := next.Upstreams[name]
		if ok && replacement.Type == up.Type && replacement.Interface == up.Interface && replacement.Service == up.Service && replacement.Config == up.Config {
			continue
		}
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			results = append(results, fail("Stop removed upstream "+name, err))
			continue
		}
		results = append(results, d.Stop(name, up, ctx)...)
	}
	return results
}

func (e Engine) UpdateClients(name, userID string, cfg config.Config) []runner.Result {
	if name == "" {
		return nil
	}
	out, ok := cfg.Outputs[name]
	if ok == false {
		return []runner.Result{fail("Update output clients", fmt.Errorf("unknown output: %s", name))}
	}
	if out.IsEnabled() == false {
		return nil
	}
	ctx := driver.BaseContext(cfg, e.Runner)
	ctx.OutputName = name
	ctx.Output = out
	ctx.ClientCIDR = out.Subnet
	if out.Type == "wireguard" || out.Type == "amneziawg" {
		return driver.SyncQuickClients(name, out, ctx)
	}
	if out.Type == "openvpn" && out.Managed {
		user, exists := cfg.Users[userID]
		revoked := exists == false || user.Disabled
		found := false
		for _, client := range out.Clients {
			if client.User == userID {
				found = true
				revoked = revoked || cfg.ClientEnabled(client) == false
			}
		}
		return driver.UpdateOpenVPNClients(name, userID, revoked || found == false, ctx)
	}
	return e.RestartOutput(name, cfg)
}
