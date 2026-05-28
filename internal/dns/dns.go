package dns

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"granger/internal/config"
	"granger/internal/driver"
	"granger/pkg/runner"
)

type Renderer struct {
	Runner runner.Runner
}

func New(r runner.Runner) Renderer { return Renderer{Runner: r} }

func (r Renderer) Apply(cfg config.Config, rules []driver.DNSRule) []runner.Result {
	body := r.Render(cfg, rules)
	var res []runner.Result
	res = append(res, r.writeConfig(config.DNSMasqPath, body))
	res = append(res, r.Runner.Run("Validate dnsmasq config", 10*time.Second, nil, "dnsmasq", "--test", "--conf-file=/etc/dnsmasq.conf"))
	if res[len(res)-1].OK {
		res = append(res, r.Runner.Run("Restart dnsmasq", 30*time.Second, nil, "systemctl", "restart", "dnsmasq.service"))
	}
	return res
}

func (r Renderer) Render(cfg config.Config, rules []driver.DNSRule) string {
	if cfg.Server.DNSListen == "" {
		cfg.Server.DNSListen = "127.0.0.1"
	}
	if cfg.Server.DNSInterface == "" {
		cfg.Server.DNSInterface = "lo"
	}
	if len(cfg.Server.DNSUpstreams) == 0 {
		cfg.Server.DNSUpstreams = []string{"8.8.8.8", "9.9.9.9"}
	}
	var b strings.Builder
	b.WriteString("interface=" + cfg.Server.DNSInterface + "\n")
	b.WriteString("bind-interfaces\n")
	b.WriteString("listen-address=" + cfg.Server.DNSListen + "\n")
	b.WriteString("port=53\nno-resolv\n")
	for _, upstream := range cfg.Server.DNSUpstreams {
		b.WriteString("server=" + upstream + "\n")
	}
	b.WriteString("cache-size=10000\n")
	b.WriteString("log-queries=extra\nlog-facility=/var/log/granger/dnsmasq.log\n")
	seenServer := map[string]bool{}
	seenSet := map[string]bool{}
	for _, rule := range rules {
		if rule.Domain == "" {
			continue
		}
		if rule.Server != "" {
			line := "server=/" + rule.Domain + "/" + rule.Server
			if !seenServer[line] {
				b.WriteString(line + "\n")
				seenServer[line] = true
			}
		}
		if rule.Set != "" {
			line := "ipset=/" + rule.Domain + "/" + rule.Set
			if !seenSet[line] {
				b.WriteString(line + "\n")
				seenSet[line] = true
			}
		}
	}
	return b.String()
}

func (r Renderer) writeConfig(path, body string) runner.Result {
	if r.Runner.DryRun {
		return runner.Result{Title: "Write dnsmasq config", Command: path, Output: body, OK: true, Status: "dry-run"}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return runner.Result{Title: "Write dnsmasq config", Command: path, Output: err.Error(), OK: false, Status: "error"}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".granger-dnsmasq-*")
	if err != nil {
		return runner.Result{Title: "Write dnsmasq config", Command: path, Output: err.Error(), OK: false, Status: "error"}
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return runner.Result{Title: "Write dnsmasq config", Command: path, Output: err.Error(), OK: false, Status: "error"}
	}
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		return runner.Result{Title: "Write dnsmasq config", Command: path, Output: err.Error(), OK: false, Status: "error"}
	}
	if err := tmp.Close(); err != nil {
		return runner.Result{Title: "Write dnsmasq config", Command: path, Output: err.Error(), OK: false, Status: "error"}
	}
	if err := os.Rename(name, path); err != nil {
		return runner.Result{Title: "Write dnsmasq config", Command: path, Output: err.Error(), OK: false, Status: "error"}
	}
	return runner.Result{Title: "Write dnsmasq config", Command: path, Output: "dnsmasq config written", OK: true, Status: "ok"}
}
