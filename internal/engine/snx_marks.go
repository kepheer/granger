package engine

import (
	"fmt"
	"time"

	"granger/internal/config"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

func (e Engine) applySNXConnectionMarks(cfg config.Config) []runner.Result {
	var results []runner.Result
	for _, name := range sortedUpstreamNames(cfg) {
		up := cfg.Upstreams[name]
		if up.Type != "snx-rs" || up.IsEnabled() == false || up.Interface == "" {
			continue
		}
		if e.Runner.Run("Check SNX-RS mark interface "+name, 3*time.Second, nil, "ip", "link", "show", "dev", up.Interface).OK == false {
			continue
		}
		mark := fmt.Sprintf("0x%x/0xffff", netfilter.RouteTableID(name))
		results = append(results,
			e.Runner.Run("Enable forwarding on SNX-RS interface "+name, 5*time.Second, nil, "sysctl", "-w", "net/ipv4/conf/"+up.Interface+"/forwarding=1"),
			e.Runner.Run("Validate SNX-RS source with route mark "+name, 5*time.Second, nil, "sysctl", "-w", "net/ipv4/conf/"+up.Interface+"/src_valid_mark=1"),
			e.Runner.Run("Restore SNX-RS connection route mark "+name, 5*time.Second, nil, "sh", "-c", netfilter.EnsureAppend("mangle", netfilter.MangleChain, "-i", up.Interface, "-m", "connmark", "--mark", mark, "-j", "CONNMARK", "--restore-mark", "--nfmask", "0xffff", "--ctmask", "0xffff")),
		)
		for _, outputName := range sortedOutputNames(cfg) {
			out := cfg.Outputs[outputName]
			if out.IsEnabled() == false || out.Interface == "" {
				continue
			}
			results = append(results, e.Runner.Run("Save SNX-RS connection route mark "+name+" for "+outputName, 5*time.Second, nil, "sh", "-c", netfilter.EnsureAppend("mangle", netfilter.MangleChain, "-i", out.Interface, "-m", "mark", "--mark", mark, "-j", "CONNMARK", "--save-mark", "--nfmask", "0xffff", "--ctmask", "0xffff")))
		}
	}
	return results
}
