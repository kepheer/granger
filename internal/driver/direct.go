package driver

import (
	"strconv"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/runner"
)

type DirectUpstream struct{}

func (DirectUpstream) Type() string        { return "direct" }
func (DirectUpstream) DisplayName() string { return "Direct uplink" }
func (DirectUpstream) Capabilities() []Capability {
	return []Capability{CapDefaultRoute, CapDomainRoute, CapCIDRRoute}
}

func (DirectUpstream) NormalizeConfig(string, config.Upstream, ApplyContext) []runner.Result {
	return nil
}

func (DirectUpstream) Start(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{{Title: "Start direct upstream " + name, Command: "noop", Output: "direct upstream uses existing system uplink", OK: true, Status: "ok"}}
}

func (DirectUpstream) Stop(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{{Title: "Stop direct upstream " + name, Command: "noop", Output: "direct upstream cannot be stopped by Granger", OK: true, Status: "ok"}}
}

func (DirectUpstream) Restart(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{{Title: "Restart direct upstream " + name, Command: "noop", Output: "direct upstream uses existing system uplink", OK: true, Status: "ok"}}
}

func (d DirectUpstream) Status(name string, up config.Upstream, ctx ApplyContext) RuntimeStatus {
	iface := resolvedInterface(up, ctx)
	return interfaceStatus(name, d.Type(), iface, "", d.Capabilities(), ctx)
}

func (DirectUpstream) ApplyRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	iface := resolvedInterface(up, ctx)
	gateway := resolvedGateway(up, ctx)
	if iface == "" {
		return []runner.Result{{Title: "Route direct " + name, Command: "config", Output: "direct upstream interface is empty", OK: false, Status: "error"}}
	}
	if gateway == "" {
		return []runner.Result{{Title: "Route direct " + name, Command: "config", Output: "direct upstream gateway is empty", OK: false, Status: "error"}}
	}
	tableID := strconv.Itoa(routeTableID(name))
	mark := fwmark(name)
	res := []runner.Result{
		ctx.Runner.RunSoft("Route default "+name, 10*time.Second, nil, "ip", "route", "replace", "default", "via", gateway, "dev", iface, "table", tableID),
	}
	if ctx.ClientCIDR != "" && ctx.Output.Interface != "" {
		res = append(res, ctx.Runner.RunSoft("Route return "+name, 10*time.Second, nil, "ip", "route", "replace", ctx.ClientCIDR, "dev", ctx.Output.Interface, "table", tableID))
	}
	res = append(res, ensureIPRule(ctx, "fwmark "+mark, tableID, strconv.Itoa(10000+routeTableID(name)%20000)))
	for _, rule := range ctx.Config.Rules {
		if rule.Via == name && rule.Default && ctx.ClientCIDR != "" {
			res = append(res, ensureIPRule(ctx, "from "+ctx.ClientCIDR, tableID, "30000"))
		}
	}
	return res
}

func (DirectUpstream) ApplyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	up.Interface = resolvedInterface(up, ctx)
	return applyUpstreamFirewall(name, up, ctx)
}

func (DirectUpstream) RenderDNS(name string, up config.Upstream, rules []config.Rule, _ ApplyContext) []DNSRule {
	return renderDomainDNS(name, up, rules)
}

type InterfaceUpstream struct{}

func (InterfaceUpstream) Type() string        { return "interface" }
func (InterfaceUpstream) DisplayName() string { return "Existing interface" }
func (InterfaceUpstream) Capabilities() []Capability {
	return []Capability{CapDefaultRoute, CapDomainRoute, CapCIDRRoute}
}

func (InterfaceUpstream) NormalizeConfig(string, config.Upstream, ApplyContext) []runner.Result {
	return nil
}

func (InterfaceUpstream) Start(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{{Title: "Start interface upstream " + name, Command: "noop", Output: "existing interface is managed outside Granger", OK: true, Status: "ok"}}
}

func (InterfaceUpstream) Stop(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{{Title: "Stop interface upstream " + name, Command: "noop", Output: "existing interface is managed outside Granger", OK: true, Status: "ok"}}
}

func (InterfaceUpstream) Restart(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{{Title: "Restart interface upstream " + name, Command: "noop", Output: "existing interface is managed outside Granger", OK: true, Status: "ok"}}
}

func (d InterfaceUpstream) Status(name string, up config.Upstream, ctx ApplyContext) RuntimeStatus {
	return interfaceStatus(name, d.Type(), up.Interface, "", d.Capabilities(), ctx)
}

func (InterfaceUpstream) ApplyRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return applyUpstreamRoutes(name, up, ctx)
}

func (InterfaceUpstream) ApplyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return applyUpstreamFirewall(name, up, ctx)
}

func (InterfaceUpstream) RenderDNS(name string, up config.Upstream, rules []config.Rule, _ ApplyContext) []DNSRule {
	return renderDomainDNS(name, up, rules)
}

func resolvedInterface(up config.Upstream, ctx ApplyContext) string {
	if strings.TrimSpace(up.Interface) != "" && up.Interface != "auto" {
		return up.Interface
	}
	return ctx.UplinkIF
}

func resolvedGateway(up config.Upstream, ctx ApplyContext) string {
	if strings.TrimSpace(up.Gateway) != "" && up.Gateway != "auto" {
		return up.Gateway
	}
	return ctx.UplinkGW
}
