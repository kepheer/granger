package engine

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"granger/internal/config"
	"granger/internal/driver"
)

type routingRuntimeState struct {
	Name      string
	Type      string
	State     driver.RuntimeState
	Interface string
	Service   string
}

type routingInterfaceState struct {
	Name      string
	Readable  bool
	Index     int
	Up        bool
	OperState string
	IPv4      []string
}

func (e Engine) RoutingKey(cfg config.Config) string {
	effective := e.effectiveConfig(cfg)
	var runtime []routingRuntimeState
	interfaces := map[string]bool{}
	for _, status := range e.Runtime(cfg) {
		runtime = append(runtime, routingRuntimeState{status.Name, status.Type, status.State, status.Interface, status.Service})
		if status.Interface != "" && status.Interface != "auto" {
			interfaces[status.Interface] = true
		}
	}
	interfaceNames := make([]string, 0, len(interfaces))
	for name := range interfaces {
		interfaceNames = append(interfaceNames, name)
	}
	sort.Strings(interfaceNames)
	var links []routingInterfaceState
	for _, name := range interfaceNames {
		links = append(links, e.routingInterface(name))
	}
	body, _ := json.Marshal(struct {
		Rules      []config.Rule
		Users      map[string]config.User
		Runtime    []routingRuntimeState
		Interfaces []routingInterfaceState
	}{effective.Rules, effective.Users, runtime, links})
	return string(body)
}

func (e Engine) routingInterface(name string) routingInterfaceState {
	state := routingInterfaceState{Name: name}
	result := e.Runner.Run("Routing interface state "+name, 5*time.Second, nil, "ip", "-j", "-4", "addr", "show", "dev", name)
	if result.OK == false {
		return state
	}
	var links []struct {
		Index     int      `json:"ifindex"`
		Flags     []string `json:"flags"`
		OperState string   `json:"operstate"`
		Addresses []struct {
			Family string `json:"family"`
			Local  string `json:"local"`
			Prefix int    `json:"prefixlen"`
		} `json:"addr_info"`
	}
	if json.Unmarshal([]byte(result.Output), &links) != nil || len(links) != 1 {
		return state
	}
	link := links[0]
	state.Readable = true
	state.Index = link.Index
	state.OperState = strings.ToUpper(link.OperState)
	for _, flag := range link.Flags {
		if flag == "UP" {
			state.Up = true
		}
	}
	addresses := map[string]bool{}
	for _, address := range link.Addresses {
		ip := net.ParseIP(address.Local)
		if address.Family == "inet" && ip != nil && ip.To4() != nil && address.Prefix >= 0 && address.Prefix <= 32 {
			addresses[fmt.Sprintf("%s/%d", ip.To4().String(), address.Prefix)] = true
		}
	}
	for address := range addresses {
		state.IPv4 = append(state.IPv4, address)
	}
	sort.Strings(state.IPv4)
	return state
}
