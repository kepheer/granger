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
	Outputs   map[string]Output   `json:"outputs" yaml:"outputs"`
	Upstreams map[string]Upstream `json:"upstreams" yaml:"upstreams"`
	Rules     []Rule              `json:"rules" yaml:"rules"`
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
	Name   string `json:"name" yaml:"name"`
	IP     string `json:"ip" yaml:"ip"`
	Config string `json:"config,omitempty" yaml:"config,omitempty"`
}

type Upstream struct {
	Type             string   `json:"type" yaml:"type"`
	Interface        string   `json:"interface" yaml:"interface"`
	Config           string   `json:"config,omitempty" yaml:"config,omitempty"`
	Service          string   `json:"service,omitempty" yaml:"service,omitempty"`
	DNS              []string `json:"dns,omitempty" yaml:"dns,omitempty"`
	Gateway          string   `json:"gateway,omitempty" yaml:"gateway,omitempty"`
	FallbackWhenDown string   `json:"fallback_when_down,omitempty" yaml:"fallback_when_down,omitempty"`
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

func Default(publicIP, uplinkIF, uplinkGW string) Config {
	return Config{
		Server: Server{
			PublicIP: publicIP,
			UplinkIF: uplinkIF,
			UplinkGW: uplinkGW,
		},
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
	}
	for name, up := range c.Upstreams {
		if up.Type == "" {
			return errors.New("upstream " + name + " has empty type")
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
