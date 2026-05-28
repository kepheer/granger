package driver

import (
	"os"
	"path/filepath"
	"strings"

	"granger/internal/config"
	"granger/pkg/runner"
)

type SingBoxOutput struct{}

func (SingBoxOutput) Type() string               { return "sing-box" }
func (SingBoxOutput) DisplayName() string        { return "sing-box" }
func (SingBoxOutput) Capabilities() []Capability { return []Capability{CapClientConfig} }

func (SingBoxOutput) GenerateServerConfig(name string, out config.Output, _ ApplyContext) []runner.Result {
	return checkConfigFile("sing-box output config "+name, out.Config)
}

func (SingBoxOutput) GenerateClientConfig(_ string, out config.Output, ctx ApplyContext) (string, []runner.Result) {
	path := firstClientConfig(ctx, out)
	if path == "" {
		return "", []runner.Result{{Title: "sing-box client config", Command: "config", Output: "client config path is empty", OK: false, Status: "error"}}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", []runner.Result{{Title: "sing-box client config", Command: path, Output: err.Error(), OK: false, Status: "error"}}
	}
	return string(b), nil
}

func (SingBoxOutput) Start(name string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Start sing-box output "+name, "start", singBoxService(out.Service, name, out.Config, out.Interface), singBoxFallbackService())}
}

func (SingBoxOutput) Stop(name string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Stop sing-box output "+name, "stop", singBoxService(out.Service, name, out.Config, out.Interface), singBoxFallbackService())}
}

func (SingBoxOutput) Restart(name string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Restart sing-box output "+name, "restart", singBoxService(out.Service, name, out.Config, out.Interface), singBoxFallbackService())}
}

func (d SingBoxOutput) Status(name string, out config.Output, ctx ApplyContext) RuntimeStatus {
	service := singBoxService(out.Service, name, out.Config, out.Interface)
	if out.Interface == "" {
		return serviceStatus(name, d.Type(), service, d.Capabilities(), ctx)
	}
	return interfaceStatus(name, d.Type(), out.Interface, service, d.Capabilities(), ctx)
}

func (SingBoxOutput) ApplyIngressFirewall(_ string, _ config.Output, ctx ApplyContext) []runner.Result {
	return outputReturnRoute(ctx)
}

type SingBoxUpstream struct{}

func (SingBoxUpstream) Type() string        { return "sing-box" }
func (SingBoxUpstream) DisplayName() string { return "sing-box" }
func (SingBoxUpstream) Capabilities() []Capability {
	return []Capability{CapDefaultRoute, CapDomainRoute, CapCIDRRoute}
}

func (SingBoxUpstream) NormalizeConfig(name string, up config.Upstream, _ ApplyContext) []runner.Result {
	return checkConfigFile("sing-box upstream config "+name, up.Config)
}

func (SingBoxUpstream) Start(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Start sing-box upstream "+name, "start", singBoxService(up.Service, name, up.Config, up.Interface), singBoxFallbackService())}
}

func (SingBoxUpstream) Stop(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Stop sing-box upstream "+name, "stop", singBoxService(up.Service, name, up.Config, up.Interface), singBoxFallbackService())}
}

func (SingBoxUpstream) Restart(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Restart sing-box upstream "+name, "restart", singBoxService(up.Service, name, up.Config, up.Interface), singBoxFallbackService())}
}

func (d SingBoxUpstream) Status(name string, up config.Upstream, ctx ApplyContext) RuntimeStatus {
	service := singBoxService(up.Service, name, up.Config, up.Interface)
	if up.Interface == "" {
		return serviceStatus(name, d.Type(), service, d.Capabilities(), ctx)
	}
	return interfaceStatus(name, d.Type(), up.Interface, service, d.Capabilities(), ctx)
}

func (SingBoxUpstream) ApplyRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	if up.Interface == "" {
		return nil
	}
	return applyUpstreamRoutes(name, up, ctx)
}

func (SingBoxUpstream) ApplyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	if up.Interface == "" {
		return nil
	}
	return applyUpstreamFirewall(name, up, ctx)
}

func (SingBoxUpstream) RenderDNS(name string, up config.Upstream, rules []config.Rule, _ ApplyContext) []DNSRule {
	return renderDomainDNS(name, up, rules)
}

func singBoxService(explicit, name, cfgPath, iface string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return "sing-box@" + singBoxProfileName(name, cfgPath, iface) + ".service"
}

func singBoxFallbackService() string {
	return "sing-box.service"
}

func singBoxProfileName(name, cfgPath, iface string) string {
	if cfgPath != "" {
		base := filepath.Base(cfgPath)
		ext := filepath.Ext(base)
		if ext != "" {
			base = strings.TrimSuffix(base, ext)
		}
		if base != "" {
			return safeName(base)
		}
	}
	if iface != "" {
		return safeName(iface)
	}
	return safeName(name)
}
