package dns

import (
	"strings"

	"granger/internal/config"
	"granger/internal/driver"
	"granger/pkg/runner"
)

type Renderer struct {
	Runner runner.Runner
}

func New(r runner.Runner) Renderer { return Renderer{Runner: r} }

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
