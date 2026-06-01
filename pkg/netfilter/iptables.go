package netfilter

import (
	"strconv"
	"strings"
)

const Backend = "iptables"

const (
	MangleChain = "GRANGER_PREROUTING"
	FilterChain = "GRANGER_FORWARD"
	NATChain    = "GRANGER_POSTROUTING"
)

func ResetScript() string {
	return strings.Join([]string{
		ensureChain("mangle", MangleChain),
		ensureChain("", FilterChain),
		ensureChain("nat", NATChain),
		EnsureAppend("mangle", "PREROUTING", "-j", MangleChain),
		EnsureAppend("", "FORWARD", "-j", FilterChain),
		EnsureAppend("nat", "POSTROUTING", "-j", NATChain),
		ShellJoin(append(iptablesPrefix("mangle"), "-F", MangleChain)...),
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
