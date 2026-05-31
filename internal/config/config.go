package config

import (
	"errors"
	"net"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	Path         = "/etc/granger/granger.yaml"
	DNSMasqPath  = "/etc/dnsmasq.d/granger.conf"
	RuntimeDir   = "/var/lib/granger"
	OptDir       = "/opt/granger"
	SecretsDir   = "/etc/granger/secrets"
	UpstreamsDir = "/etc/granger/upstreams"
	OutputsDir   = "/etc/granger/outputs"
)

type Config struct {
	Server    Server              `json:"server" yaml:"server"`
	Users     map[string]User     `json:"users,omitempty" yaml:"users,omitempty"`
	Outputs   map[string]Output   `json:"outputs" yaml:"outputs"`
	Upstreams map[string]Upstream `json:"upstreams" yaml:"upstreams"`
	Rules     []Rule              `json:"rules" yaml:"rules"`
}

type User struct {
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Disabled    bool   `json:"disabled,omitempty" yaml:"disabled,omitempty"`
}

type Server struct {
	PublicIP     string   `json:"public_ip,omitempty" yaml:"public_ip,omitempty"`
	UplinkIF     string   `json:"uplink_if,omitempty" yaml:"uplink_if,omitempty"`
	UplinkGW     string   `json:"uplink_gw,omitempty" yaml:"uplink_gw,omitempty"`
	DNSInterface string   `json:"dns_interface,omitempty" yaml:"dns_interface,omitempty"`
	DNSListen    string   `json:"dns_listen,omitempty" yaml:"dns_listen,omitempty"`
	DNSUpstreams []string `json:"dns_upstreams,omitempty" yaml:"dns_upstreams,omitempty"`
}

type Output struct {
	Type       string   `json:"type" yaml:"type"`
	Interface  string   `json:"interface" yaml:"interface"`
	Config     string   `json:"config,omitempty" yaml:"config,omitempty"`
	Service    string   `json:"service,omitempty" yaml:"service,omitempty"`
	Subnet     string   `json:"subnet" yaml:"subnet"`
	ServerIP   string   `json:"server_ip" yaml:"server_ip"`
	ListenPort int      `json:"listen_port" yaml:"listen_port"`
	Clients    []Client `json:"clients" yaml:"clients"`
}

type Client struct {
	Name     string `json:"name" yaml:"name"`
	User     string `json:"user,omitempty" yaml:"user,omitempty"`
	IP       string `json:"ip" yaml:"ip"`
	Config   string `json:"config,omitempty" yaml:"config,omitempty"`
	Disabled bool   `json:"disabled,omitempty" yaml:"disabled,omitempty"`
}

type Upstream struct {
	Type             string              `json:"type" yaml:"type"`
	Interface        string              `json:"interface" yaml:"interface"`
	Config           string              `json:"config,omitempty" yaml:"config,omitempty"`
	Service          string              `json:"service,omitempty" yaml:"service,omitempty"`
	Enabled          *bool               `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	DNS              []string            `json:"dns,omitempty" yaml:"dns,omitempty"`
	Gateway          string              `json:"gateway,omitempty" yaml:"gateway,omitempty"`
	FallbackWhenDown string              `json:"fallback_when_down,omitempty" yaml:"fallback_when_down,omitempty"`
	RouteMode        string              `json:"route_mode,omitempty" yaml:"route_mode,omitempty"`
	AuthFlow         []AuthStep          `json:"auth_flow,omitempty" yaml:"auth_flow,omitempty"`
	PromptPatterns   map[string][]string `json:"prompt_patterns,omitempty" yaml:"prompt_patterns,omitempty"`
	Install          *InstallSpec        `json:"install,omitempty" yaml:"install,omitempty"`
}

type Rule struct {
	Name              string   `json:"name" yaml:"name"`
	Domains           []string `json:"domains,omitempty" yaml:"domains,omitempty"`
	CIDRs             []string `json:"cidrs,omitempty" yaml:"cidrs,omitempty"`
	Via               string   `json:"via" yaml:"via"`
	DomainFallbackVia string   `json:"domain_fallback_via,omitempty" yaml:"domain_fallback_via,omitempty"`
	CIDRFallback      string   `json:"cidr_fallback,omitempty" yaml:"cidr_fallback,omitempty"`
	Default           bool     `json:"default,omitempty" yaml:"default,omitempty"`
}

type AuthStep struct {
	Name      string   `json:"name,omitempty" yaml:"name,omitempty"`
	Type      string   `json:"type" yaml:"type"`
	Label     string   `json:"label,omitempty" yaml:"label,omitempty"`
	Secret    bool     `json:"secret,omitempty" yaml:"secret,omitempty"`
	Prompts   []string `json:"prompts,omitempty" yaml:"prompts,omitempty"`
	TimeoutMS int      `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
}

type InstallSpec struct {
	Package string   `json:"package,omitempty" yaml:"package,omitempty"`
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
	Managed bool     `json:"managed,omitempty" yaml:"managed,omitempty"`
}

func Default(publicIP, uplinkIF, uplinkGW string) Config {
	return Config{
		Server: Server{
			PublicIP: publicIP,
			UplinkIF: uplinkIF,
			UplinkGW: uplinkGW,
		},
		Users:     map[string]User{},
		Outputs:   map[string]Output{},
		Upstreams: map[string]Upstream{},
		Rules:     nil,
	}
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

func SaveAtomic(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".granger-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (c Config) Validate() error {
	interfaces := map[string]string{}
	services := map[string]string{}
	for name := range c.Users {
		if name == "" {
			return errors.New("user has empty name")
		}
	}
	for name, out := range c.Outputs {
		if out.Type == "" {
			return errors.New("output " + name + " has empty type")
		}
		if out.Interface == "" && outputRequiresInterface(out.Type) {
			return errors.New("output " + name + " has empty interface")
		}
		if err := rememberUnique(interfaces, out.Interface, "output "+name); err != nil {
			return err
		}
		if err := rememberUnique(services, out.Service, "output "+name); err != nil {
			return err
		}
		if out.Subnet != "" {
			if _, _, err := net.ParseCIDR(out.Subnet); err != nil {
				return err
			}
		}
		if err := c.validateClients(name, out.Clients); err != nil {
			return err
		}
	}
	for name, up := range c.Upstreams {
		if up.Type == "" {
			return errors.New("upstream " + name + " has empty type")
		}
		if err := validateAuthFlow(name, up.AuthFlow); err != nil {
			return err
		}
		if err := rememberUnique(interfaces, up.Interface, "upstream "+name); err != nil {
			return err
		}
		if err := rememberUnique(services, up.Service, "upstream "+name); err != nil {
			return err
		}
	}
	for _, rule := range c.Rules {
		if rule.Via == "" {
			return errors.New("rule " + rule.Name + " has empty via")
		}
		if _, ok := c.Upstreams[rule.Via]; !ok {
			return errors.New("rule " + rule.Name + " references missing upstream " + rule.Via)
		}
		for _, cidr := range rule.CIDRs {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (u Upstream) IsEnabled() bool {
	return u.Enabled == nil || *u.Enabled
}

func (c Config) ClientEnabled(client Client) bool {
	if client.Disabled {
		return false
	}
	if client.User == "" {
		return true
	}
	user, ok := c.Users[client.User]
	return ok && !user.Disabled
}

func (c Config) EnabledClients(out Output) []Client {
	clients := make([]Client, 0, len(out.Clients))
	for _, client := range out.Clients {
		if c.ClientEnabled(client) {
			clients = append(clients, client)
		}
	}
	return clients
}

func (c Config) validateClients(outputName string, clients []Client) error {
	seen := map[string]bool{}
	for _, client := range clients {
		if client.Name == "" {
			return errors.New("output " + outputName + " has client with empty name")
		}
		if seen[client.Name] {
			return errors.New("output " + outputName + " has duplicate client " + client.Name)
		}
		seen[client.Name] = true
		if client.User != "" {
			if _, ok := c.Users[client.User]; !ok {
				return errors.New("output " + outputName + " client " + client.Name + " references missing user " + client.User)
			}
		}
	}
	return nil
}

func outputRequiresInterface(typ string) bool {
	switch typ {
	case "sing-box", "xray":
		return false
	default:
		return true
	}
}

func rememberUnique(seen map[string]string, value, owner string) error {
	if value == "" || value == "auto" {
		return nil
	}
	if prev, ok := seen[value]; ok {
		return errors.New(owner + " conflicts with " + prev + " on " + value)
	}
	seen[value] = owner
	return nil
}

func validateAuthFlow(upstreamName string, flow []AuthStep) error {
	seen := map[string]bool{}
	for _, step := range flow {
		if step.Type == "" {
			return errors.New("upstream " + upstreamName + " auth flow has empty step type")
		}
		switch step.Type {
		case "username", "password", "otp", "sms", "email", "custom":
		default:
			return errors.New("upstream " + upstreamName + " auth flow has unsupported step type " + step.Type)
		}
		key := step.Name
		if key == "" {
			key = step.Type
		}
		if seen[key] {
			return errors.New("upstream " + upstreamName + " auth flow has duplicate step " + key)
		}
		seen[key] = true
	}
	return nil
}
