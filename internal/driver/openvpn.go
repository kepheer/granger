package driver

import (
	"os"

	"granger/internal/config"
	"granger/pkg/runner"
)

type OpenVPNOutput struct{}

func (OpenVPNOutput) Type() string               { return "openvpn" }
func (OpenVPNOutput) DisplayName() string        { return "OpenVPN" }
func (OpenVPNOutput) Capabilities() []Capability { return []Capability{CapClientConfig} }

func (OpenVPNOutput) GenerateServerConfig(name string, out config.Output, _ ApplyContext) []runner.Result {
	if out.Config == "" {
		return nil
	}
	if _, err := os.Stat(out.Config); err != nil {
		return []runner.Result{{Title: "OpenVPN server config " + name, Command: out.Config, Output: err.Error(), OK: false, Status: "error"}}
	}
	return []runner.Result{{Title: "OpenVPN server config " + name, Command: out.Config, Output: "config exists", OK: true, Status: "ok"}}
}

func (OpenVPNOutput) GenerateClientConfig(_ string, out config.Output, ctx ApplyContext) (string, []runner.Result) {
	path := firstClientConfig(ctx, out)
	if path == "" {
		return "", []runner.Result{{Title: "OpenVPN client config", Command: "config", Output: "client config path is empty", OK: false, Status: "error"}}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", []runner.Result{{Title: "OpenVPN client config", Command: path, Output: err.Error(), OK: false, Status: "error"}}
	}
	return string(b), nil
}

func (OpenVPNOutput) Start(name string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Start OpenVPN output "+name, "start", openVPNService(out.Service, "server", name, out.Config, out.Interface), openVPNFallbackService(name, out.Config, out.Interface))}
}

func (OpenVPNOutput) Stop(name string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Stop OpenVPN output "+name, "stop", openVPNService(out.Service, "server", name, out.Config, out.Interface), openVPNFallbackService(name, out.Config, out.Interface))}
}

func (OpenVPNOutput) Restart(name string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Restart OpenVPN output "+name, "restart", openVPNService(out.Service, "server", name, out.Config, out.Interface), openVPNFallbackService(name, out.Config, out.Interface))}
}

func (d OpenVPNOutput) Status(name string, out config.Output, ctx ApplyContext) RuntimeStatus {
	return interfaceStatus(name, d.Type(), out.Interface, openVPNService(out.Service, "server", name, out.Config, out.Interface), d.Capabilities(), ctx)
}

func (OpenVPNOutput) ApplyIngressFirewall(_ string, _ config.Output, ctx ApplyContext) []runner.Result {
	return outputReturnRoute(ctx)
}

type OpenVPNUpstream struct{}

func (OpenVPNUpstream) Type() string        { return "openvpn" }
func (OpenVPNUpstream) DisplayName() string { return "OpenVPN" }
func (OpenVPNUpstream) Capabilities() []Capability {
	return []Capability{CapDefaultRoute, CapDomainRoute, CapCIDRRoute}
}

func (OpenVPNUpstream) NormalizeConfig(name string, up config.Upstream, _ ApplyContext) []runner.Result {
	if up.Config == "" {
		return nil
	}
	if _, err := os.Stat(up.Config); err != nil {
		return []runner.Result{{Title: "OpenVPN upstream config " + name, Command: up.Config, Output: err.Error(), OK: false, Status: "error"}}
	}
	return []runner.Result{{Title: "OpenVPN upstream config " + name, Command: up.Config, Output: "config exists", OK: true, Status: "ok"}}
}

func (OpenVPNUpstream) Start(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Start OpenVPN upstream "+name, "start", openVPNService(up.Service, "client", name, up.Config, up.Interface), openVPNFallbackService(name, up.Config, up.Interface))}
}

func (OpenVPNUpstream) Stop(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Stop OpenVPN upstream "+name, "stop", openVPNService(up.Service, "client", name, up.Config, up.Interface), openVPNFallbackService(name, up.Config, up.Interface))}
}

func (OpenVPNUpstream) Restart(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Restart OpenVPN upstream "+name, "restart", openVPNService(up.Service, "client", name, up.Config, up.Interface), openVPNFallbackService(name, up.Config, up.Interface))}
}

func (d OpenVPNUpstream) Status(name string, up config.Upstream, ctx ApplyContext) RuntimeStatus {
	return interfaceStatus(name, d.Type(), up.Interface, openVPNService(up.Service, "client", name, up.Config, up.Interface), d.Capabilities(), ctx)
}

func (OpenVPNUpstream) ApplyRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return applyUpstreamRoutes(name, up, ctx)
}

func (OpenVPNUpstream) ApplyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return applyUpstreamFirewall(name, up, ctx)
}

func (OpenVPNUpstream) RenderDNS(name string, up config.Upstream, rules []config.Rule, _ ApplyContext) []DNSRule {
	return renderDomainDNS(name, up, rules)
}
