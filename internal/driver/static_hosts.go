package driver

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/runner"
)

func hostCIDR(cidr string) (string, bool) {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil || ip.To4() == nil {
		return "", false
	}
	prefix, bits := network.Mask.Size()
	return ip.String(), bits == 32 && prefix == 32
}

func staticHostSet(name string) string {
	return fmt.Sprintf("granger_c%d", RouteTableID(name))
}

func staticHosts(name string, cfg config.Config) []string {
	seen := map[string]bool{}
	for _, rule := range cfg.Rules {
		if rule.Via != name {
			continue
		}
		for _, cidr := range rule.CIDRs {
			if address, ok := hostCIDR(cidr); ok {
				seen[address] = true
			}
		}
	}
	values := make([]string, 0, len(seen))
	for address := range seen {
		values = append(values, address)
	}
	sort.Strings(values)
	return values
}

func populateStaticHosts(name string, ctx ApplyContext) []runner.Result {
	set := staticHostSet(name)
	temp := set + "_new"
	var body strings.Builder
	fmt.Fprintf(&body, "create %s hash:ip family inet -exist\nflush %s\n", temp, temp)
	for _, address := range staticHosts(name, ctx.Config) {
		fmt.Fprintf(&body, "add %s %s -exist\n", temp, address)
	}
	fmt.Fprintf(&body, "swap %s %s\ndestroy %s\n", temp, set, temp)
	created := ctx.Runner.Run("Create static host set "+name, 10*time.Second, nil, "ipset", "create", set, "hash:ip", "family", "inet", "-exist")
	result := ctx.Runner.Run("Populate static host set "+name, 15*time.Second, strings.NewReader(body.String()), "ipset", "restore")
	if ctx.Runner.DryRun {
		result.Output = body.String()
	}
	return []runner.Result{
		created,
		result,
	}
}
