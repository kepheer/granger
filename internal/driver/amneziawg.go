package driver

import (
	"os"
	"time"

	"granger/internal/config"
	"granger/pkg/runner"
)

type AmneziaWGOutput struct{}

func (AmneziaWGOutput) Type() string               { return "amneziawg" }
func (AmneziaWGOutput) DisplayName() string        { return "AmneziaWG" }
func (AmneziaWGOutput) Capabilities() []Capability { return []Capability{CapClientConfig} }

func (AmneziaWGOutput) GenerateServerConfig(name string, out config.Output, _ ApplyContext) []runner.Result {
	if out.Config == "" {
		return nil
	}
	if _, err := os.Stat(out.Config); err != nil {
		return []runner.Result{{Title: "AmneziaWG server config " + name, Command: out.Config, Output: err.Error(), OK: false, Status: "error"}}
	}
	return []runner.Result{{Title: "AmneziaWG server config " + name, Command: out.Config, Output: "config exists", OK: true, Status: "ok"}}
}

func (AmneziaWGOutput) GenerateClientConfig(_ string, out config.Output, ctx ApplyContext) (string, []runner.Result) {
	path := firstClientConfig(ctx, out)
	if path == "" {
		return "", []runner.Result{{Title: "AmneziaWG client config", Command: "config", Output: "client config path is empty", OK: false, Status: "error"}}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", []runner.Result{{Title: "AmneziaWG client config", Command: path, Output: err.Error(), OK: false, Status: "error"}}
	}
	return string(b), nil
}

func (AmneziaWGOutput) Start(_ string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Start AmneziaWG output "+out.Interface, 30*time.Second, nil, "systemctl", "start", amneziaWGService(out.Service, out.Interface))}
}

func (AmneziaWGOutput) Stop(_ string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Stop AmneziaWG output "+out.Interface, 30*time.Second, nil, "systemctl", "stop", amneziaWGService(out.Service, out.Interface))}
}

func (AmneziaWGOutput) Restart(_ string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Restart AmneziaWG output "+out.Interface, 30*time.Second, nil, "systemctl", "restart", amneziaWGService(out.Service, out.Interface))}
}

func (d AmneziaWGOutput) Status(name string, out config.Output, ctx ApplyContext) RuntimeStatus {
	return interfaceStatus(name, d.Type(), out.Interface, amneziaWGService(out.Service, out.Interface), d.Capabilities(), ctx)
}

func (AmneziaWGOutput) ApplyIngressFirewall(_ string, _ config.Output, ctx ApplyContext) []runner.Result {
	return outputReturnRoute(ctx)
}

type AmneziaWGUpstream struct{}

func (AmneziaWGUpstream) Type() string        { return "amneziawg" }
func (AmneziaWGUpstream) DisplayName() string { return "AmneziaWG" }
func (AmneziaWGUpstream) Capabilities() []Capability {
	return []Capability{CapDefaultRoute, CapDomainRoute, CapCIDRRoute}
}

func (AmneziaWGUpstream) NormalizeConfig(name string, up config.Upstream, _ ApplyContext) []runner.Result {
	if up.Config == "" {
		return nil
	}
	if _, err := os.Stat(up.Config); err != nil {
		return []runner.Result{{Title: "AmneziaWG upstream config " + name, Command: up.Config, Output: err.Error(), OK: false, Status: "error"}}
	}
	return []runner.Result{{Title: "AmneziaWG upstream config " + name, Command: up.Config, Output: "config exists", OK: true, Status: "ok"}}
}

func (AmneziaWGUpstream) Start(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Start AmneziaWG upstream "+name, 30*time.Second, nil, "systemctl", "start", amneziaWGService(up.Service, up.Interface))}
}

func (AmneziaWGUpstream) Stop(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Stop AmneziaWG upstream "+name, 30*time.Second, nil, "systemctl", "stop", amneziaWGService(up.Service, up.Interface))}
}

func (AmneziaWGUpstream) Restart(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{ctx.Runner.Run("Restart AmneziaWG upstream "+name, 30*time.Second, nil, "systemctl", "restart", amneziaWGService(up.Service, up.Interface))}
}

func (d AmneziaWGUpstream) Status(name string, up config.Upstream, ctx ApplyContext) RuntimeStatus {
	return interfaceStatus(name, d.Type(), up.Interface, amneziaWGService(up.Service, up.Interface), d.Capabilities(), ctx)
}

func (AmneziaWGUpstream) ApplyRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return applyUpstreamRoutes(name, up, ctx)
}

func (AmneziaWGUpstream) ApplyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return applyUpstreamFirewall(name, up, ctx)
}

func (AmneziaWGUpstream) RenderDNS(name string, up config.Upstream, rules []config.Rule, _ ApplyContext) []DNSRule {
	return renderDomainDNS(name, up, rules)
}

func amneziaWGService(explicit, iface string) string {
	return configuredService(explicit, "awg-quick@"+safeName(iface)+".service")
}
