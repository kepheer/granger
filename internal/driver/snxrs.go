package driver

import (
	"os"
	"strconv"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

type SNXRSUpstream struct{}

func (SNXRSUpstream) Type() string        { return "snx-rs" }
func (SNXRSUpstream) DisplayName() string { return "SNX-RS" }
func (SNXRSUpstream) Capabilities() []Capability {
	return []Capability{CapInteractive, CapDomainRoute, CapCIDRRoute}
}

func (SNXRSUpstream) NormalizeConfig(name string, up config.Upstream, _ ApplyContext) []runner.Result {
	if !up.IsEnabled() {
		return []runner.Result{{Title: "SNX-RS upstream " + name, Command: "config", Output: "upstream is disabled", OK: true, Status: "disabled"}}
	}
	if up.Config == "" {
		return []runner.Result{{Title: "SNX-RS upstream config " + name, Command: "config", Output: "config path is empty", OK: false, Status: "error"}}
	}
	if _, err := os.Stat(up.Config); err != nil {
		return []runner.Result{{Title: "SNX-RS upstream config " + name, Command: up.Config, Output: err.Error(), OK: false, Status: "error"}}
	}
	return []runner.Result{{Title: "SNX-RS upstream config " + name, Command: up.Config, Output: "config exists", OK: true, Status: "ok"}}
}

func (SNXRSUpstream) Start(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Start SNX-RS upstream "+name, 30*time.Second, nil, "systemctl", "start", snxRSService(up.Service))}
}

func (SNXRSUpstream) Stop(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Stop SNX-RS upstream "+name, 30*time.Second, nil, "systemctl", "stop", snxRSService(up.Service))}
}

func (SNXRSUpstream) Restart(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Restart SNX-RS upstream "+name, 30*time.Second, nil, "systemctl", "restart", snxRSService(up.Service))}
}

func (d SNXRSUpstream) Status(name string, up config.Upstream, ctx ApplyContext) RuntimeStatus {
	if !up.IsEnabled() {
		return RuntimeStatus{Name: name, Type: d.Type(), State: StatePending, Summary: "SNX-RS upstream is disabled", Interface: up.Interface, Service: snxRSService(up.Service), Capabilities: d.Capabilities()}
	}
	res := ctx.Runner.Run("Status SNX-RS "+name, 5*time.Second, nil, "ip", "-br", "addr", "show", up.Interface)
	st := RuntimeStatus{Name: name, Type: d.Type(), Interface: up.Interface, Service: snxRSService(up.Service), Capabilities: d.Capabilities(), Results: []runner.Result{res}}
	if res.OK {
		st.State = StateHealthy
		st.Summary = up.Interface + " is up"
	} else {
		st.State = StatePending
		st.Summary = up.Interface + " is not connected; domain rules may use configured fallback and CIDR rules stay pending"
	}
	return st
}

func (SNXRSUpstream) ApplyRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	if !up.IsEnabled() {
		return []runner.Result{{Title: "SNX-RS routes " + name, Command: "config", Output: "upstream is disabled", OK: true, Status: "disabled"}}
	}
	tableID := strconv.Itoa(routeTableID(name))
	mark := fwmark(name)
	res := []runner.Result{
		ensureIPRule(ctx, "fwmark "+mark, tableID, strconv.Itoa(10000+routeTableID(name)%20000)),
	}
	if !snxInterfaceUp(up, ctx) {
		for _, rule := range ctx.Config.Rules {
			if rule.Via != name {
				continue
			}
			for _, cidr := range rule.CIDRs {
				res = append(res, ctx.Runner.RunSoft("SNX-RS pending CIDR "+cidr, 10*time.Second, nil, "ip", "route", "replace", "unreachable", cidr, "table", tableID))
			}
		}
		res = append(res, runner.Result{Title: "SNX-RS routes " + name, Command: "ip link show " + up.Interface, Output: "SNX-RS is not connected; routes are pending until " + up.Interface + " exists", OK: true, Status: "pending"})
		return res
	}
	snxIP := snxIPv4(up, ctx)
	for _, dnsIP := range up.DNS {
		args := []string{"route", "replace", dnsIP + "/32", "dev", up.Interface}
		if snxIP != "" {
			args = append(args, "src", snxIP)
		}
		res = append(res, ctx.Runner.RunSoft("SNX-RS DNS route "+dnsIP, 10*time.Second, nil, "ip", args...))
	}
	for _, rule := range ctx.Config.Rules {
		if rule.Via != name {
			continue
		}
		for _, cidr := range rule.CIDRs {
			args := []string{"route", "replace", cidr, "dev", up.Interface}
			if snxIP != "" {
				args = append(args, "src", snxIP)
			}
			args = append(args, "table", tableID)
			res = append(res, ctx.Runner.RunSoft("SNX-RS route "+cidr, 10*time.Second, nil, "ip", args...))
		}
	}
	if ctx.ClientCIDR != "" && ctx.Output.Interface != "" {
		res = append(res, ctx.Runner.RunSoft("SNX-RS return route "+name, 10*time.Second, nil, "ip", "route", "replace", ctx.ClientCIDR, "dev", ctx.Output.Interface, "table", tableID))
	}
	return res
}

func (SNXRSUpstream) ApplyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	if !up.IsEnabled() || ctx.Output.Interface == "" || ctx.ClientCIDR == "" {
		return nil
	}
	setName := ipsetName(name)
	mark := fwmark(name)
	res := []runner.Result{
		ctx.Runner.RunSoft("Create ipset "+setName, 10*time.Second, nil, "ipset", "create", setName, "hash:ip", "family", "inet", "timeout", "86400", "-exist"),
		runShell(ctx, "Mark SNX-RS domain set "+name, netfilter.EnsureAppend("mangle", "PREROUTING", "-i", ctx.Output.Interface, "-m", "set", "--match-set", setName, "dst", "-j", "MARK", "--set-mark", mark)),
	}
	for _, rule := range ctx.Config.Rules {
		if rule.Via != name {
			continue
		}
		for _, cidr := range rule.CIDRs {
			res = append(res, runShell(ctx, "Mark SNX-RS CIDR "+cidr, netfilter.EnsureAppend("mangle", "PREROUTING", "-i", ctx.Output.Interface, "-d", cidr, "-j", "MARK", "--set-mark", mark)))
		}
	}
	if snxInterfaceUp(up, ctx) {
		res = append(res,
			runShell(ctx, "FORWARD output -> "+name, netfilter.EnsureInsert("", "FORWARD", 1, "-i", ctx.Output.Interface, "-o", up.Interface, "-j", "ACCEPT")),
			runShell(ctx, "FORWARD "+name+" -> output", netfilter.EnsureInsert("", "FORWARD", 2, "-i", up.Interface, "-o", ctx.Output.Interface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT")),
			runShell(ctx, "NAT "+name, netfilter.EnsureAppend("nat", "POSTROUTING", "-s", ctx.ClientCIDR, "-o", up.Interface, "-j", "MASQUERADE")),
		)
	}
	return res
}

func (SNXRSUpstream) RenderDNS(name string, up config.Upstream, rules []config.Rule, ctx ApplyContext) []DNSRule {
	if !up.IsEnabled() {
		return nil
	}
	connected := snxInterfaceUp(up, ctx)
	var out []DNSRule
	for _, rule := range rules {
		if rule.Via != name {
			continue
		}
		fallback := firstNonEmpty(rule.DomainFallbackVia, up.FallbackWhenDown)
		for _, domain := range rule.Domains {
			if connected && len(up.DNS) > 0 {
				for _, server := range up.DNS {
					out = append(out, DNSRule{Domain: domain, Server: server, Set: ipsetName(name)})
				}
				continue
			}
			if fallback != "" && fallback != "pending" {
				out = append(out, DNSRule{Domain: domain, Set: ipsetName(fallback)})
			}
		}
	}
	return out
}

func snxRSService(explicit string) string {
	return configuredService(explicit, "snx-rs.service")
}

func snxInterfaceUp(up config.Upstream, ctx ApplyContext) bool {
	if strings.TrimSpace(up.Interface) == "" {
		return false
	}
	return ctx.Runner.Run("Status SNX-RS interface "+up.Interface, 3*time.Second, nil, "ip", "link", "show", up.Interface).OK
}

func snxIPv4(up config.Upstream, ctx ApplyContext) string {
	res := ctx.Runner.Run("SNX-RS IPv4 "+up.Interface, 3*time.Second, nil, "sh", "-c", "ip -4 -o addr show dev "+netfilter.ShellQuote(up.Interface)+" | awk '{split($4,a,\"/\"); print a[1]; exit}'")
	if !res.OK {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(res.Output, "(no output)", ""))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
