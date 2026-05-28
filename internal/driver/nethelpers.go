package driver

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_.@-]+`)

func safeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	return safeNameRe.ReplaceAllString(name, "_")
}

func routeTableID(name string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return 10000 + int(h.Sum32()%20000)
}

func fwmark(name string) string {
	return fmt.Sprintf("0x%x", routeTableID(name))
}

func ipsetName(name string) string {
	return "granger_" + safeName(name) + "4"
}

func runShell(ctx ApplyContext, title, script string) runner.Result {
	return ctx.Runner.RunSoft(title, 10*time.Second, nil, "sh", "-c", script)
}

func configuredService(explicit, fallback string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return fallback
}

func openVPNProfileName(name, cfgPath, iface string) string {
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

func openVPNService(explicit, kind, name, cfgPath, iface string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	profile := openVPNProfileName(name, cfgPath, iface)
	if kind == "server" {
		return "openvpn-server@" + profile + ".service"
	}
	return "openvpn-client@" + profile + ".service"
}

func openVPNFallbackService(name, cfgPath, iface string) string {
	return "openvpn@" + openVPNProfileName(name, cfgPath, iface) + ".service"
}

func systemctlWithFallback(ctx ApplyContext, title, action, primary, fallback string) runner.Result {
	if fallback == "" || fallback == primary {
		return ctx.Runner.Run(title, 30*time.Second, nil, "systemctl", action, primary)
	}
	return runShell(ctx, title, netfilter.ShellJoin("systemctl", action, primary)+" || "+netfilter.ShellJoin("systemctl", action, fallback))
}

func firstClientConfig(out config.Output) string {
	for _, client := range out.Clients {
		if client.Config != "" {
			return client.Config
		}
	}
	return ""
}

func interfaceStatus(name, typ, iface, service string, caps []Capability, ctx ApplyContext) RuntimeStatus {
	res := ctx.Runner.Run("Status "+typ+" "+name, 5*time.Second, nil, "ip", "-br", "addr", "show", iface)
	st := RuntimeStatus{Name: name, Type: typ, Interface: iface, Service: service, Capabilities: caps, Results: []runner.Result{res}}
	if res.OK {
		st.State = StateHealthy
		st.Summary = iface + " is up"
	} else {
		st.State = StatePending
		st.Summary = iface + " is not up"
	}
	return st
}

func serviceStatus(name, typ, service string, caps []Capability, ctx ApplyContext) RuntimeStatus {
	res := ctx.Runner.Run("Status "+typ+" "+name, 5*time.Second, nil, "systemctl", "is-active", service)
	st := RuntimeStatus{Name: name, Type: typ, Service: service, Capabilities: caps, Results: []runner.Result{res}}
	if res.OK {
		st.State = StateHealthy
		st.Summary = service + " is active"
	} else {
		st.State = StatePending
		st.Summary = service + " is not active"
	}
	return st
}

func outputReturnRoute(ctx ApplyContext) []runner.Result {
	if ctx.ClientCIDR == "" || ctx.Output.Interface == "" {
		return nil
	}
	return []runner.Result{
		ctx.Runner.RunSoft("Return route "+ctx.Output.Interface, 10*time.Second, nil, "ip", "route", "replace", ctx.ClientCIDR, "dev", ctx.Output.Interface),
	}
}

func applyUpstreamRoutes(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	if up.Interface == "" {
		return []runner.Result{{Title: "Route upstream " + name, Command: "config", Output: "upstream interface is empty", OK: false, Status: "error"}}
	}
	tableID := strconv.Itoa(routeTableID(name))
	mark := fwmark(name)
	var res []runner.Result
	res = append(res, ctx.Runner.RunSoft("Route default "+name, 10*time.Second, nil, "ip", "route", "replace", "default", "dev", up.Interface, "table", tableID))
	if ctx.ClientCIDR != "" && ctx.Output.Interface != "" {
		res = append(res, ctx.Runner.RunSoft("Route return "+name, 10*time.Second, nil, "ip", "route", "replace", ctx.ClientCIDR, "dev", ctx.Output.Interface, "table", tableID))
	}
	res = append(res, ensureIPRule(ctx, "fwmark "+mark, tableID, strconv.Itoa(10000+routeTableID(name)%20000)))
	for _, rule := range ctx.Config.Rules {
		if rule.Via != name {
			continue
		}
		if rule.Default && ctx.ClientCIDR != "" {
			res = append(res, ensureIPRule(ctx, "from "+ctx.ClientCIDR, tableID, "30000"))
		}
	}
	return res
}

func applyUpstreamFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	if up.Interface == "" || ctx.Output.Interface == "" || ctx.ClientCIDR == "" {
		return nil
	}
	setName := ipsetName(name)
	mark := fwmark(name)
	var res []runner.Result
	res = append(res, ctx.Runner.RunSoft("Create ipset "+setName, 10*time.Second, nil, "ipset", "create", setName, "hash:ip", "family", "inet", "timeout", "86400", "-exist"))
	res = append(res, runShell(ctx, "Mark domain set "+name, netfilter.EnsureAppend("mangle", "PREROUTING", "-i", ctx.Output.Interface, "-m", "set", "--match-set", setName, "dst", "-j", "MARK", "--set-mark", mark)))
	for _, rule := range ctx.Config.Rules {
		if rule.Via != name {
			continue
		}
		for _, cidr := range rule.CIDRs {
			res = append(res, runShell(ctx, "Mark CIDR "+cidr+" via "+name, netfilter.EnsureAppend("mangle", "PREROUTING", "-i", ctx.Output.Interface, "-d", cidr, "-j", "MARK", "--set-mark", mark)))
		}
	}
	res = append(res,
		runShell(ctx, "FORWARD output -> "+name, netfilter.EnsureInsert("", "FORWARD", 1, "-i", ctx.Output.Interface, "-o", up.Interface, "-j", "ACCEPT")),
		runShell(ctx, "FORWARD "+name+" -> output", netfilter.EnsureInsert("", "FORWARD", 2, "-i", up.Interface, "-o", ctx.Output.Interface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT")),
		runShell(ctx, "NAT "+name, netfilter.EnsureAppend("nat", "POSTROUTING", "-s", ctx.ClientCIDR, "-o", up.Interface, "-j", "MASQUERADE")),
	)
	return res
}

func renderDomainDNS(name string, up config.Upstream, rules []config.Rule) []DNSRule {
	var out []DNSRule
	for _, rule := range rules {
		if rule.Via != name {
			continue
		}
		for _, domain := range rule.Domains {
			if len(up.DNS) == 0 {
				out = append(out, DNSRule{Domain: domain, Set: ipsetName(name)})
				continue
			}
			for _, server := range up.DNS {
				out = append(out, DNSRule{Domain: domain, Server: server, Set: ipsetName(name)})
			}
		}
	}
	return out
}

func ensureIPRule(ctx ApplyContext, match, table, priority string) runner.Result {
	return runShell(ctx, "ip rule "+table, "ip rule show | grep -q "+netfilter.ShellQuote(match+".*lookup "+table)+" || ip rule add "+match+" table "+netfilter.ShellQuote(table)+" priority "+netfilter.ShellQuote(priority))
}
