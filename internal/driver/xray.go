package driver

import (
	"os"
	"path/filepath"
	"strings"

	"granger/internal/config"
	"granger/pkg/runner"
)

type XrayOutput struct{}

func (XrayOutput) Type() string               { return "xray" }
func (XrayOutput) DisplayName() string        { return "Xray" }
func (XrayOutput) Capabilities() []Capability { return []Capability{CapClientConfig} }

func (XrayOutput) GenerateServerConfig(name string, out config.Output, _ ApplyContext) []runner.Result {
	return checkConfigFile("Xray output config "+name, out.Config)
}

func (XrayOutput) GenerateClientConfig(_ string, out config.Output, _ ApplyContext) (string, []runner.Result) {
	path := firstClientConfig(out)
	if path == "" {
		return "", []runner.Result{{Title: "Xray client config", Command: "config", Output: "client config path is empty", OK: false, Status: "error"}}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", []runner.Result{{Title: "Xray client config", Command: path, Output: err.Error(), OK: false, Status: "error"}}
	}
	return string(b), nil
}

func (XrayOutput) Start(name string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Start Xray output "+name, "start", xrayService(out.Service, name, out.Config, out.Interface), xrayFallbackService())}
}

func (XrayOutput) Stop(name string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Stop Xray output "+name, "stop", xrayService(out.Service, name, out.Config, out.Interface), xrayFallbackService())}
}

func (XrayOutput) Restart(name string, out config.Output, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Restart Xray output "+name, "restart", xrayService(out.Service, name, out.Config, out.Interface), xrayFallbackService())}
}

func (d XrayOutput) Status(name string, out config.Output, ctx ApplyContext) RuntimeStatus {
	service := xrayService(out.Service, name, out.Config, out.Interface)
	if out.Interface == "" {
		return serviceStatus(name, d.Type(), service, d.Capabilities(), ctx)
	}
	return interfaceStatus(name, d.Type(), out.Interface, service, d.Capabilities(), ctx)
}

func (XrayOutput) ApplyIngressFirewall(_ string, _ config.Output, ctx ApplyContext) []runner.Result {
	return outputReturnRoute(ctx)
}

type XrayUpstream struct{}

func (XrayUpstream) Type() string        { return "xray" }
func (XrayUpstream) DisplayName() string { return "Xray" }
func (XrayUpstream) Capabilities() []Capability {
	return []Capability{CapDefaultRoute, CapDomainRoute, CapCIDRRoute}
}

func (XrayUpstream) NormalizeConfig(name string, up config.Upstream, _ ApplyContext) []runner.Result {
	return checkConfigFile("Xray upstream config "+name, up.Config)
}

func (XrayUpstream) Start(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Start Xray upstream "+name, "start", xrayService(up.Service, name, up.Config, up.Interface), xrayFallbackService())}
}

func (XrayUpstream) Stop(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Stop Xray upstream "+name, "stop", xrayService(up.Service, name, up.Config, up.Interface), xrayFallbackService())}
}

func (XrayUpstream) Restart(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	return []runner.Result{systemctlWithFallback(ctx, "Restart Xray upstream "+name, "restart", xrayService(up.Service, name, up.Config, up.Interface), xrayFallbackService())}
}

func (d XrayUpstream) Status(name string, up config.Upstream, ctx ApplyContext) RuntimeStatus {
	service := xrayService(up.Service, name, up.Config, up.Interface)
	if up.Interface == "" {
		return serviceStatus(name, d.Type(), service, d.Capabilities(), ctx)
	}
	return interfaceStatus(name, d.Type(), up.Interface, service, d.Capabilities(), ctx)
}

func (XrayUpstream) ApplyRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	if up.Interface == "" {
		return nil
	}
	return applyUpstreamRoutes(name, up, ctx)
}

func (XrayUpstream) ApplyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	if up.Interface == "" {
		return nil
	}
	return applyUpstreamFirewall(name, up, ctx)
}

func (XrayUpstream) RenderDNS(name string, up config.Upstream, rules []config.Rule, _ ApplyContext) []DNSRule {
	return renderDomainDNS(name, up, rules)
}

func xrayService(explicit, name, cfgPath, iface string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return "xray@" + xrayProfileName(name, cfgPath, iface) + ".service"
}

func xrayFallbackService() string {
	return "xray.service"
}

func xrayProfileName(name, cfgPath, iface string) string {
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

func checkConfigFile(title, path string) []runner.Result {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return []runner.Result{{Title: title, Command: path, Output: err.Error(), OK: false, Status: "error"}}
	}
	return []runner.Result{{Title: title, Command: path, Output: "config exists", OK: true, Status: "ok"}}
}
