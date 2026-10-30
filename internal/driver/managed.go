package driver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

func prepareManagedProxy(name string, out config.Output, ctx ApplyContext) []runner.Result {
	results := checkConfigFile(out.Type+" config "+name, out.Config)
	if out.Managed == false {
		return results
	}
	for _, result := range results {
		if result.OK == false {
			return results
		}
	}
	if ctx.Runner.DryRun {
		return append(results, runner.Result{Title: "Prepare managed service " + name, Command: out.Service, OK: true, Status: "dry-run", Output: "would write managed proxy service"})
	}
	changed, err := updateProxySNIPolicy(ctx.Config, name, out)
	if err != nil {
		return append(results, runner.Result{Title: "Render proxy policy", OK: false, Status: "error", Output: err.Error()})
	}
	binary, err := exec.LookPath(out.Type)
	if err != nil {
		return append(results, runner.Result{Title: "Locate " + out.Type, OK: false, Status: "error", Output: err.Error()})
	}
	if out.Service == "" || filepath.Base(out.Service) != out.Service || strings.HasPrefix(out.Service, "granger-") == false {
		return append(results, runner.Result{Title: "Prepare managed service " + name, OK: false, Status: "error", Output: "managed proxy requires a granger-owned service name"})
	}
	args := "run -config "
	if out.Type == "sing-box" {
		args = "run -c "
	}
	body := fmt.Sprintf("[Unit]\nDescription=Granger %s entrypoint %s\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nExecStart=%s %s%s\nRestart=on-failure\nRestartSec=2s\nUser=root\n\n[Install]\nWantedBy=multi-user.target\n", out.Type, safeName(name), netfilter.ShellQuote(binary), args, netfilter.ShellQuote(out.Config))
	path := filepath.Join("/etc/systemd/system", out.Service)
	current, _ := os.ReadFile(path)
	if string(current) != body {
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			return append(results, runner.Result{Title: "Write proxy service", OK: false, Status: "error", Output: err.Error()})
		}
		results = append(results, ctx.Runner.Run("Reload proxy services", 10*time.Second, nil, "systemctl", "daemon-reload"))
	}
	validation := []string{"run", "-test", "-config", out.Config}
	if out.Type == "sing-box" {
		validation = []string{"check", "-c", out.Config}
	}
	checked := ctx.Runner.Run("Validate "+out.Type+" entrypoint", 10*time.Second, nil, binary, validation...)
	results = append(results, checked)
	if checked.OK && changed {
		results = append(results, ctx.Runner.Run("Reload proxy policy "+name, 20*time.Second, nil, "systemctl", "try-restart", out.Service))
	}
	return results
}

func managedProxy(out config.Output) bool {
	return out.Managed && (out.Type == "xray" || out.Type == "sing-box")
}

func proxyFirewall(name string, up config.Upstream, ctx ApplyContext) []runner.Result {
	if up.Interface == "" {
		return nil
	}
	set := ipsetName(name)
	results := []runner.Result{ctx.Runner.Run("Create proxy ipset "+set, 10*time.Second, nil, "ipset", "create", set, "hash:ip", "family", "inet", "timeout", "86400", "-exist")}
	results = append(results, populateStaticHosts(name, ctx)...)
	for _, client := range ctx.Config.EnabledClients(ctx.Output) {
		mark := fmt.Sprintf("0x%x/0xffff0000", netfilter.UserMark(ctx.OutputName, client.Name))
		results = append(results, runShell(ctx, "Proxy domain route "+client.User+" via "+name, netfilter.EnsureAppend("mangle", netfilter.OutputChain, "-m", "mark", "--mark", mark, "-m", "set", "--match-set", set, "dst", "-m", "mark", "--mark", "0/0xffff", "-j", "MARK", "--set-xmark", fwmark(name)+"/0xffff")))
		results = append(results, runShell(ctx, "Proxy static hosts "+client.User+" via "+name, netfilter.EnsureAppend("mangle", netfilter.OutputChain, "-m", "mark", "--mark", mark, "-m", "set", "--match-set", staticHostSet(name), "dst", "-m", "mark", "--mark", "0/0xffff", "-j", "MARK", "--set-xmark", fwmark(name)+"/0xffff")))
		for _, rule := range ctx.Config.Rules {
			if rule.Via != name {
				continue
			}
			for _, cidr := range rule.CIDRs {
				if _, host := hostCIDR(cidr); host {
					continue
				}
				results = append(results, runShell(ctx, "Proxy CIDR "+cidr+" via "+name, netfilter.EnsureAppend("mangle", netfilter.OutputChain, "-m", "mark", "--mark", mark, "-d", cidr, "-m", "mark", "--mark", "0/0xffff", "-j", "MARK", "--set-xmark", fwmark(name)+"/0xffff")))
			}
		}
		results = append(results, runShell(ctx, "Proxy NAT "+client.User+" via "+name, netfilter.EnsureAppend("nat", netfilter.NATChain, "-o", up.Interface, "-m", "mark", "--mark", mark, "-j", "MASQUERADE")))
	}
	return results
}
