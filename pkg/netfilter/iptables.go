package netfilter

import (
	"strconv"
	"strings"
)

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
