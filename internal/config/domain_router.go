package config

import (
	"errors"
	"strconv"
	"strings"
)

type DomainRouter struct {
	Backend  string `json:"backend,omitempty" yaml:"backend,omitempty"`
	Port     int    `json:"port,omitempty" yaml:"port,omitempty"`
	Table    int    `json:"table,omitempty" yaml:"table,omitempty"`
	TCPPorts []int  `json:"tcp_ports,omitempty" yaml:"tcp_ports,omitempty"`
	UDPPorts []int  `json:"udp_ports,omitempty" yaml:"udp_ports,omitempty"`
}

func (s Server) DomainRouterSettings() DomainRouter {
	settings := s.DomainRouter
	if settings.Backend == "" {
		settings.Backend = "xray"
	}
	if settings.Port == 0 {
		settings.Port = 19845
	}
	if settings.Table == 0 {
		settings.Table = 19845
	}
	if settings.TCPPorts == nil {
		settings.TCPPorts = []int{80, 443}
	}
	if settings.UDPPorts == nil {
		settings.UDPPorts = []int{443}
	}
	return settings
}

func PortList(ports []int, separator string) string {
	values := make([]string, len(ports))
	for i, port := range ports {
		values[i] = strconv.Itoa(port)
	}
	return strings.Join(values, separator)
}

func (s Server) validateDomainRouter() error {
	settings := s.DomainRouterSettings()
	if settings.Backend != "xray" {
		return errors.New("unsupported domain router backend")
	}
	if settings.Port < 1 || settings.Port > 65535 {
		return errors.New("domain router port must be between 1 and 65535")
	}
	if settings.Table < 1 || settings.Table >= 2147483000 || settings.Table == 253 || settings.Table == 254 || settings.Table == 255 {
		return errors.New("domain router table is reserved or outside its range")
	}
	if settings.Port == s.GUIVPNPort {
		return errors.New("domain router port conflicts with the panel")
	}
	for _, ports := range [][]int{settings.TCPPorts, settings.UDPPorts} {
		if len(ports) > 15 {
			return errors.New("domain router supports up to 15 ports per transport")
		}
		seen := map[int]bool{}
		for _, port := range ports {
			if port < 1 || port > 65535 || seen[port] {
				return errors.New("domain router ports must be unique and between 1 and 65535")
			}
			seen[port] = true
		}
	}
	return nil
}
