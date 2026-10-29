package engine

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"granger/internal/config"
	"granger/internal/driver"
	"granger/internal/provision"
	"granger/pkg/netfilter"
)

func (e Engine) ReconcileConfig(previous, next config.Config) ApplyPlan {
	ctx := driver.BaseContext(next, e.Runner)
	previousCtx := driver.BaseContext(previous, e.Runner)
	results := e.beginRoutingUpdate(previous)
	results = append(results, e.beginRoutingUpdate(next)...)
	results = append(results, e.Runner.Run("Protect access before reconciling tunnels", 10*time.Second, nil, "sh", "-c", netfilter.AccessHooks()))
	results = append(results, e.applyUserAccess(next)...)
	if resultsOK(results) == false {
		return ApplyPlan{Results: results, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
	}

	for _, name := range sortedOutputNames(previous) {
		out := previous.Outputs[name]
		replacement, exists := next.Outputs[name]
		if out.IsEnabled() == false || (exists && replacement.IsEnabled() && outputRuntimeEqual(out, replacement) && (out.Managed == false || previous.Server.PublicIP == next.Server.PublicIP)) {
			continue
		}
		d, err := e.Registry.Output(out.Type)
		if err != nil {
			results = append(results, fail("Stop replaced output "+name, err))
			continue
		}
		outCtx := previousCtx
		outCtx.OutputName, outCtx.Output, outCtx.ClientCIDR = name, out, out.Subnet
		results = append(results, d.Stop(name, out, outCtx)...)
	}
	for _, name := range sortedUpstreamNames(previous) {
		up := previous.Upstreams[name]
		replacement, exists := next.Upstreams[name]
		if up.IsEnabled() == false || (exists && replacement.IsEnabled() && upstreamRuntimeEqual(up, replacement)) {
			continue
		}
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			results = append(results, fail("Stop replaced upstream "+name, err))
			continue
		}
		results = append(results, d.Stop(name, up, previousCtx)...)
	}
	if resultsOK(results) == false {
		return ApplyPlan{Results: results, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
	}

	for _, name := range sortedOutputNames(next) {
		out := next.Outputs[name]
		old, exists := previous.Outputs[name]
		if out.IsEnabled() == false || (exists && old.IsEnabled() && outputRuntimeEqual(old, out) && (out.Managed == false || previous.Server.PublicIP == next.Server.PublicIP)) {
			continue
		}
		d, err := e.Registry.Output(out.Type)
		if err != nil {
			results = append(results, fail("Start replacement output "+name, err))
			continue
		}
		if out.Managed && e.Runner.DryRun == false {
			if err := provision.EnsureOutput(name, &out, next.Server.PublicIP); err != nil {
				results = append(results, fail("Regenerate managed output "+name, err))
				continue
			}
			next.Outputs[name] = out
		}
		if out.Managed == false || e.Runner.DryRun == false {
			if err := requireRuntimeConfig(out.Config); err != nil {
				results = append(results, fail("Prepare replacement output "+name, err))
				continue
			}
		}
		outCtx := ctx
		outCtx.OutputName, outCtx.Output, outCtx.ClientCIDR = name, out, out.Subnet
		prepared := d.GenerateServerConfig(name, out, outCtx)
		results = append(results, prepared...)
		if resultsOK(prepared) {
			results = append(results, d.Start(name, out, outCtx)...)
		}
	}
	ctx = driver.BaseContext(next, e.Runner)
	for _, name := range sortedUpstreamNames(next) {
		up := next.Upstreams[name]
		old, exists := previous.Upstreams[name]
		if up.IsEnabled() == false || (exists && old.IsEnabled() && upstreamRuntimeEqual(old, up)) {
			continue
		}
		d, err := e.Registry.Upstream(up.Type)
		if err != nil {
			results = append(results, fail("Start replacement upstream "+name, err))
			continue
		}
		if up.Type != "direct" && up.Type != "interface" {
			if err := requireRuntimeConfig(up.Config); err != nil {
				results = append(results, fail("Prepare replacement upstream "+name, err))
				continue
			}
		}
		prepared := d.NormalizeConfig(name, up, ctx)
		results = append(results, prepared...)
		if resultsOK(prepared) {
			results = append(results, d.Start(name, up, ctx)...)
		}
	}

	if resultsOK(results) == false {
		return ApplyPlan{Results: results, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
	}
	plan := e.ApplyConfig(next)
	plan.Results = append(results, plan.Results...)
	return plan
}

func outputRuntimeEqual(previous, next config.Output) bool {
	previous.Enabled, next.Enabled = nil, nil
	return reflect.DeepEqual(previous, next)
}

func upstreamRuntimeEqual(previous, next config.Upstream) bool {
	return previous.Type == next.Type && previous.Interface == next.Interface && previous.Config == next.Config && previous.Service == next.Service && reflect.DeepEqual(previous.AuthFlow, next.AuthFlow) && reflect.DeepEqual(previous.PromptPatterns, next.PromptPatterns)
}

func requireRuntimeConfig(path string) error {
	if path == "" {
		return fmt.Errorf("config path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() == false {
		return fmt.Errorf("config is not a regular file: %s", path)
	}
	return nil
}
