package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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
	cfg = e.effectiveConfig(cfg)
	sniConfig := cfg
	cfg = kernelDomainRules(cfg)
	ctx := driver.BaseContext(cfg, e.Runner)
	var res []runner.Result
	res = append(res, e.prepareSNI(sniConfig)...)
	if resultsOK(res) == false {
		return ApplyPlan{Results: res, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
	}
	res = append(res, e.beginRoutingUpdate(cfg)...)
	if resultsOK(res) == false {
		return ApplyPlan{Results: res, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
	}
	res = append(res, e.Runner.Run("Reset Granger firewall chains", 10*time.Second, nil, "sh", "-c", netfilter.ResetScript()))
	res = append(res, e.resetManagedRouteTables(cfg)...)
	res = append(res, e.applyUserAccess(cfg)...)
	if resultsOK(res) == false {
		return ApplyPlan{Results: res, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
	}
	res = append(res, e.Runner.Run("Enable IPv4 forwarding", 10*time.Second, nil, "sysctl", "-w", "net.ipv4.ip_forward=1"))
	outputNames := sortedOutputNames(cfg)
	upstreamNames := sortedUpstreamNames(cfg)
	for _, name := range outputNames {
		out := cfg.Outputs[name]
		if !out.IsEnabled() {
			res = append(res, runner.Result{Title: "Output driver " + name, Command: "config", Output: "output is disabled", OK: true, Status: "disabled"})
			continue
		}
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
			if !out.IsEnabled() {
				continue
			}
			outCtx := ctx
			outCtx.OutputName = outName
			outCtx.Output = out
			outCtx.ClientCIDR = out.Subnet
			res = append(res, d.ApplyFirewall(name, up, outCtx)...)
		}
	}
	res = append(res, e.applyPersonalFallback(cfg)...)
	res = append(res, e.applySNXConnectionMarks(cfg)...)
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
			if !out.IsEnabled() {
				continue
			}
			outCtx := ctx
			outCtx.OutputName = outName
			outCtx.Output = out
			outCtx.ClientCIDR = out.Subnet
			res = append(res, d.ApplyRoutes(name, up, outCtx)...)
		}
	}
	res = append(res, e.applyUserDefaultRoutes(cfg)...)
	res = append(res, e.applySNICapture(sniConfig)...)
	res = append(res, e.applyDNSCapture(cfg))
	dnsRules := e.DNSRules(cfg, ctx)
	res = append(res, dns.New(e.Runner).Apply(cfg, dnsRules)...)
	if resultsOK(res) {
		res = append(res, e.finishRoutingUpdate())
	}
	return ApplyPlan{DNSRules: dnsRules, Results: res, DryRun: e.Runner.DryRun, Firewall: netfilter.Backend}
}

func (e Engine) resetManagedRouteTables(cfg config.Config) []runner.Result {
	previous := readManagedRouteTables()
	current := make([]string, 0, len(cfg.Upstreams))
	for _, name := range sortedUpstreamNames(cfg) {
		current = append(current, strconv.Itoa(driver.RouteTableID(name)))
	}
	var results []runner.Result
	results = append(results, e.Runner.Run("Reset closed personal defaults", 10*time.Second, nil, "sh", "-c", "while ip rule del table "+routingGuardTable+" priority 30000 2>/dev/null; do :; done"))
	for _, table := range previous {
		script := "while ip rule del table " + netfilter.ShellQuote(table) + " 2>/dev/null; do :; done\n" +
			"ip route flush table " + netfilter.ShellQuote(table) + " 2>/dev/null || true"
		results = append(results, e.Runner.RunSoft("Reset route table "+table, 10*time.Second, nil, "sh", "-c", script))
	}
	for _, table := range current {
		results = append(results, e.Runner.Run("Protect managed route table "+table, 10*time.Second, nil, "ip", "route", "replace", "unreachable", "default", "metric", "32760", "table", table))
	}
	if !e.Runner.DryRun {
		if err := writeManagedRouteTables(current); err != nil {
			results = append(results, fail("Persist managed route tables", err))
		}
	}
	return results
}

func readManagedRouteTables() []string {
	body, err := os.ReadFile(filepath.Join(config.RuntimeDir, "route-tables"))
	if err != nil {
		return nil
	}
	var tables []string
	for _, line := range strings.Fields(string(body)) {
		if _, err := strconv.Atoi(line); err == nil {
			tables = append(tables, line)
		}
	}
	return tables
}

func (e Engine) effectiveConfig(cfg config.Config) config.Config {
	ctx := driver.BaseContext(cfg, e.Runner)
	available := map[string]bool{}
	for name, up := range cfg.Upstreams {
		if up.IsEnabled() == false {
			continue
		}
		d, err := e.Registry.Upstream(up.Type)
		if err == nil {
			available[name] = d.Status(name, up, ctx).State == driver.StateHealthy
		}
	}

	resolve := func(name, fallback string) string {
		seen := map[string]bool{}
		current := name
		for {
			up, ok := cfg.Upstreams[current]
			if ok == false || seen[current] {
				return name
			}
			seen[current] = true
			if available[current] || up.BlockFallback {
				return current
			}
			next := up.FallbackWhenDown
			if current == name && fallback != "" {
				next = fallback
			}
			if next == "" || next == "pending" {
				return current
			}
			current = next
		}
	}

	var rules []config.Rule
	for _, rule := range cfg.Rules {
		if len(rule.Domains) > 0 {
			domain := rule
			domain.CIDRs = nil
			domain.Default = false
			domain.Via = resolve(rule.Via, rule.DomainFallbackVia)
			rules = append(rules, domain)
		}
		if len(rule.CIDRs) > 0 || rule.Default {
			route := rule
			route.Domains = nil
			route.Via = resolve(rule.Via, rule.CIDRFallback)
			rules = append(rules, route)
		}
	}
	users := make(map[string]config.User, len(cfg.Users))
	for id, user := range cfg.Users {
		if user.DefaultUpstream != "" {
			user.DefaultUpstream = resolve(user.DefaultUpstream, "")
		}
		users[id] = user
	}
	cfg.Users = users
	cfg.Rules = rules
	return cfg
}

func (e Engine) SetUpstreamEnabled(name string, enabled bool, cfg config.Config) []runner.Result {
	up, ok := cfg.Upstreams[name]
	if !ok {
		return []runner.Result{fail("Set upstream state", fmt.Errorf("unknown upstream: %s", name))}
	}
	d, err := e.Registry.Upstream(up.Type)
	if err != nil {
		return []runner.Result{fail("Set upstream state", err)}
	}
	if enabled {
		ctx := driver.BaseContext(cfg, e.Runner)
		results := d.NormalizeConfig(name, up, ctx)
		results = append(results, d.Start(name, up, ctx)...)
		return append(results, e.ApplyConfig(cfg).Results...)
	}
	return append(d.Stop(name, up, driver.BaseContext(cfg, e.Runner)), e.ApplyConfig(cfg).Results...)
}

func (e Engine) SetOutputEnabled(name string, enabled bool, cfg config.Config) []runner.Result {
	out, ok := cfg.Outputs[name]
	if !ok {
		return []runner.Result{fail("Set output state", fmt.Errorf("unknown output: %s", name))}
	}
	d, err := e.Registry.Output(out.Type)
	if err != nil {
		return []runner.Result{fail("Set output state", err)}
	}
	if enabled {
		ctx := driver.BaseContext(cfg, e.Runner)
		results := d.GenerateServerConfig(name, out, ctx)
		results = append(results, d.Start(name, out, ctx)...)
		return append(results, e.ApplyConfig(cfg).Results...)
	}
	return append(d.Stop(name, out, driver.BaseContext(cfg, e.Runner)), e.ApplyConfig(cfg).Results...)
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
		if !o.IsEnabled() {
			out = append(out, driver.RuntimeStatus{Name: name, Type: o.Type, State: driver.StatePending, Summary: "output is disabled", Interface: o.Interface, Service: o.Service})
			continue
		}
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

func writeManagedRouteTables(tables []string) error {
	if err := os.MkdirAll(config.RuntimeDir, 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(config.RuntimeDir, ".route-tables-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(strings.Join(tables, "\n") + "\n"); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(config.RuntimeDir, "route-tables"))
}
