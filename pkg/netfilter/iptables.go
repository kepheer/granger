package netfilter

import (
	"hash/fnv"
	"strconv"
	"strings"
)

const Backend = "iptables"

const (
	UserEgressChain  = "GRANGER_USER_EGRESS"
	UserForwardChain = "GRANGER_USER_FORWARD"
	UserInputChain   = "GRANGER_USER_INPUT"
	OutputChain      = "GRANGER_OUTPUT"
	UserOutputChain  = "GRANGER_USER_OUTPUT"
	MangleChain      = "GRANGER_PREROUTING"
	FilterChain      = "GRANGER_FORWARD"
	NATChain         = "GRANGER_POSTROUTING"
	DNSChain         = "GRANGER_DNS"
)

func ResetScript() string {
	return strings.Join([]string{
		ensureChain("mangle", MangleChain),
		ensureChain("mangle", OutputChain),
		ensureChain("", FilterChain),
		ensureChain("", UserOutputChain),
		ensureChain("nat", NATChain),
		ensureChain("nat", DNSChain),
		EnsureAppend("mangle", "PREROUTING", "-j", MangleChain),
		EnsureAppend("mangle", "OUTPUT", "-j", OutputChain),
		EnsureAppend("", "FORWARD", "-j", FilterChain),
		AccessHooks(),
		EnsureAppend("nat", "POSTROUTING", "-j", NATChain),
		EnsureInsert("nat", "PREROUTING", 1, "-j", DNSChain),
		ShellJoin(append(iptablesPrefix("mangle"), "-F", MangleChain)...),
		ShellJoin(append(iptablesPrefix("mangle"), "-F", OutputChain)...),
		ShellJoin(append(iptablesPrefix(""), "-F", FilterChain)...),
		ShellJoin(append(iptablesPrefix("nat"), "-F", NATChain)...),
	}, "\n")
}

func EnsureAppend(table, chain string, args ...string) string {
	check := append(iptablesPrefix(table), "-C", chain)
	check = append(check, args...)
	add := append(iptablesPrefix(table), "-A", chain)
	add = append(add, args...)
	return ShellJoin(check...) + " 2>/dev/null || " + ShellJoin(add...)
}

func EnsureInsert(table, chain string, position int, args ...string) string {
	check := append(iptablesPrefix(table), "-C", chain)
	check = append(check, args...)
	add := append(iptablesPrefix(table), "-I", chain, strconv.Itoa(position))
	add = append(add, args...)
	return ShellJoin(check...) + " 2>/dev/null || " + ShellJoin(add...)
}

func ShellJoin(parts ...string) string {
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, ShellQuote(part))
	}
	return strings.Join(quoted, " ")
}

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func iptablesPrefix(table string) []string {
	if table == "" || table == "filter" {
		return []string{"iptables"}
	}
	return []string{"iptables", "-t", table}
}

func ensureChain(table, chain string) string {
	return ShellJoin(append(iptablesPrefix(table), "-N", chain)...) + " 2>/dev/null || true"
}

func ProxyMark(name string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(name))
	return 0x1000000 + int(hash.Sum32()&0xffffff)
}

func UserMark(output, user string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(output + ":" + user))
	return int(0x40000000 | ((hash.Sum32()%16383)+1)<<16)
}

func EnsureFirst(table, chain, target string) string {
	list := ShellJoin(append(iptablesPrefix(table), "-S", chain)...)
	expected := "-A " + chain + " -j " + target
	insert := append(iptablesPrefix(table), "-I", chain, "1", "-j", target)
	return "[ \"$(" + list + " | sed -n '2p')\" = " + ShellQuote(expected) + " ] || " + ShellJoin(insert...)
}

func AccessHooks() string {
	return strings.Join([]string{ensureChain("mangle", UserEgressChain), EnsureFirst("mangle", "POSTROUTING", UserEgressChain), ensureChain("", UserForwardChain), ensureChain("", UserOutputChain), ensureChain("", UserInputChain), EnsureFirst("", "FORWARD", UserForwardChain), EnsureFirst("", "OUTPUT", UserOutputChain), EnsureFirst("", "INPUT", UserInputChain), IPv6ProxyGuard()}, "\n")
}

func IPv6ProxyGuard() string {
	create := ShellJoin("ip6tables", "-N", UserOutputChain) + " 2>/dev/null || true"
	check := ShellJoin("ip6tables", "-C", UserOutputChain, "-m", "mark", "--mark", "0x40000000/0x40000000", "-j", "REJECT")
	add := ShellJoin("ip6tables", "-A", UserOutputChain, "-m", "mark", "--mark", "0x40000000/0x40000000", "-j", "REJECT")
	first := strings.ReplaceAll(EnsureFirst("", "OUTPUT", UserOutputChain), "'iptables'", "'ip6tables'")
	return "if [ -r /proc/net/if_inet6 ] && [ -n \"$(head -n 1 /proc/net/if_inet6)\" ]; then\n" + create + "\n" + check + " 2>/dev/null || " + add + "\n" + first + "\nfi"
}

func RouteTableID(name string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(name))
	return 10000 + int(hash.Sum32()%20000)
}

func EnsureIPRule(match, table, priority string) string {
	selector := match + " table " + ShellQuote(table) + " priority " + ShellQuote(priority)
	exists := "ip rule show " + selector + " | grep -q ."
	return exists + " || ip rule add " + selector + " || " + exists
}
