package config

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
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
	Server    Server              `json:"server"`
	Outputs   map[string]Output   `json:"outputs"`
	Upstreams map[string]Upstream `json:"upstreams"`
	Rules     []Rule              `json:"rules"`
}

type Server struct {
	PublicIP     string   `json:"public_ip,omitempty"`
	UplinkIF     string   `json:"uplink_if,omitempty"`
	UplinkGW     string   `json:"uplink_gw,omitempty"`
	DNSInterface string   `json:"dns_interface,omitempty"`
	DNSListen    string   `json:"dns_listen,omitempty"`
	DNSUpstreams []string `json:"dns_upstreams,omitempty"`
}

type Output struct {
	Type       string   `json:"type"`
	Interface  string   `json:"interface"`
	Subnet     string   `json:"subnet"`
	ServerIP   string   `json:"server_ip"`
	ListenPort int      `json:"listen_port"`
	Clients    []Client `json:"clients"`
}

type Client struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type Upstream struct {
	Type             string   `json:"type"`
	Interface        string   `json:"interface"`
	Config           string   `json:"config,omitempty"`
	DNS              []string `json:"dns,omitempty"`
	Gateway          string   `json:"gateway,omitempty"`
	FallbackWhenDown string   `json:"fallback_when_down,omitempty"`
}

type Rule struct {
	Name              string   `json:"name"`
	Domains           []string `json:"domains,omitempty"`
	CIDRs             []string `json:"cidrs,omitempty"`
	Via               string   `json:"via"`
	DomainFallbackVia string   `json:"domain_fallback_via,omitempty"`
	CIDRFallback      string   `json:"cidr_fallback,omitempty"`
	Default           bool     `json:"default,omitempty"`
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
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

func SaveAtomic(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
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
	for name, out := range c.Outputs {
		if out.Type == "" || out.Interface == "" {
			return errors.New("output " + name + " has empty type/interface")
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
