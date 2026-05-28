package driver

import (
	"os"
	"time"

	"granger/internal/config"
	"granger/pkg/runner"
)

type WireGuardOutput struct{}

func (WireGuardOutput) Type() string               { return "wireguard" }
func (WireGuardOutput) DisplayName() string        { return "WireGuard" }
func (WireGuardOutput) Capabilities() []Capability { return []Capability{CapClientConfig} }

func (WireGuardOutput) GenerateServerConfig(name string, out config.Output, ctx ApplyContext) []runner.Result {
	if out.Config == "" {
		return nil
	}
	if _, err := os.Stat(out.Config); err != nil {
		return []runner.Result{{Title: "WireGuard server config " + name, Command: out.Config, Output: err.Error(), OK: false, Status: "error"}}
	}
	return []runner.Result{{Title: "WireGuard server config " + name, Command: out.Config, Output: "config exists", OK: true, Status: "ok"}}
}

func (WireGuardOutput) GenerateClientConfig(_ string, out config.Output, ctx ApplyContext) (string, []runner.Result) {
	path := firstClientConfig(ctx, out)
	if path == "" {
		return "", []runner.Result{{Title: "WireGuard client config", Command: "config", Output: "client config path is empty", OK: false, Status: "error"}}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", []runner.Result{{Title: "WireGuard client config", Command: path, Output: err.Error(), OK: false, Status: "error"}}
	}
	return string(b), nil
}

func (WireGuardOutput) Start(_ string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Start WireGuard output "+out.Interface, 30*time.Second, nil, "systemctl", "start", wireGuardService(out.Service, out.Interface))}
}

func (WireGuardOutput) Stop(_ string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Stop WireGuard output "+out.Interface, 30*time.Second, nil, "systemctl", "stop", wireGuardService(out.Service, out.Interface))}
}

func (WireGuardOutput) Restart(_ string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Restart WireGuard output "+out.Interface, 30*time.Second, nil, "systemctl", "restart", wireGuardService(out.Service, out.Interface))}
}

func (d WireGuardOutput) Status(name string, out config.Output, ctx ApplyContext) RuntimeStatus {
	return interfaceStatus(name, d.Type(), out.Interface, wireGuardService(out.Service, out.Interface), d.Capabilities(), ctx)
}

func (WireGuardOutput) ApplyIngressFirewall(_ string, _ config.Output, ctx ApplyContext) []runner.Result {
	return outputReturnRoute(ctx)
}

type WireGuardUpstream struct{}

func (WireGuardUpstream) Type() string        { return "wireguard" }
func (WireGuardUpstream) DisplayName() string { return "WireGuard" }
func (WireGuardUpstream) Capabilities() []Capability {
	return []Capability{CapDefaultRoute, CapDomainRoute, CapCIDRRoute}
}

func (WireGuardUpstream) NormalizeConfig(name string, up config.Upstream, _ ApplyContext) []runner.Result {
	if up.Config == "" {
		return nil
	}
	if _, err := os.Stat(up.Config); err != nil {
		return []runner.Result{{Title: "WireGuard upstream config " + name, Command: up.Config, Output: err.Error(), OK: false, Status: "error"}}
	}
	return []runner.Result{{Title: "WireGuard upstream config " + name, Command: up.Config, Output: "config exists", OK: true, Status: "ok"}}
}

func (WireGuardUpstream) Start(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Start WireGuard upstream "+name, 30*time.Second, nil, "systemctl", "start", wireGuardService(up.Service, up.Interface))}
}

func (WireGuardUpstream) Stop(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Stop WireGuard upstream "+name, 30*time.Second, nil, "systemctl", "stop", wireGuardService(up.Service, up.Interface))}
}

func (WireGuardUpstream) Restart(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Restart WireGuard upstream "+name, 30*time.Second, nil, "systemctl", "restart", wireGuardService(up.Service, up.Interface))}
}

func (d WireGuardUpstream) Status(name string, up config.Upstream, ctx ApplyContext) RuntimeStatus {
	return interfaceStatus(name, d.Type(), up.Interface, wireGuardService(up.Service, up.Interface), d.Capabilities(), ctx)
}

func (WireGuardUpstream) ApplyRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return applyUpstreamRoutes(name, up, ctx)
}

func (WireGuardUpstream) ApplyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return applyUpstreamFirewall(name, up, ctx)
}

func (WireGuardUpstream) RenderDNS(name string, up config.Upstream, rules []config.Rule, _ ApplyContext) []DNSRule {
	return renderDomainDNS(name, up, rules)
}

func wireGuardService(explicit, iface string) string {
	return configuredService(explicit, "wg-quick@"+safeName(iface)+".service")
}
