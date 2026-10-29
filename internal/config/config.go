package config

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
	"granger/pkg/netfilter"
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
	Admin            bool     `json:"admin,omitempty" yaml:"admin,omitempty"`
	AccessRestricted bool     `json:"access_restricted,omitempty" yaml:"access_restricted,omitempty"`
	AllowedUpstreams []string `json:"allowed_upstreams,omitempty" yaml:"allowed_upstreams,omitempty"`
	DefaultUpstream  string   `json:"default_upstream,omitempty" yaml:"default_upstream,omitempty"`
	DisplayName      string   `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Disabled         bool     `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	Output           string   `json:"output,omitempty" yaml:"output,omitempty"`
	Client           string   `json:"client,omitempty" yaml:"client,omitempty"`
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
	Managed         bool              `json:"managed,omitempty" yaml:"managed,omitempty"`
	ProtocolOptions map[string]string `json:"protocol_options,omitempty" yaml:"protocol_options,omitempty"`
	Type            string            `json:"type" yaml:"type"`
	Interface       string            `json:"interface" yaml:"interface"`
	Enabled         *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Config          string            `json:"config,omitempty" yaml:"config,omitempty"`
	Service         string            `json:"service,omitempty" yaml:"service,omitempty"`
	Subnet          string            `json:"subnet" yaml:"subnet"`
	ServerIP        string            `json:"server_ip" yaml:"server_ip"`
	ListenPort      int               `json:"listen_port" yaml:"listen_port"`
	Clients         []Client          `json:"clients" yaml:"clients"`
}

type Client struct {
	Name      string `json:"name" yaml:"name"`
	User      string `json:"user,omitempty" yaml:"user,omitempty"`
	IP        string `json:"ip" yaml:"ip"`
	Config    string `json:"config,omitempty" yaml:"config,omitempty"`
	PublicKey string `json:"public_key,omitempty" yaml:"public_key,omitempty"`
	Disabled  bool   `json:"disabled,omitempty" yaml:"disabled,omitempty"`
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
	Default          bool                `json:"default,omitempty" yaml:"default,omitempty"`
	BlockFallback    bool                `json:"block_fallback,omitempty" yaml:"block_fallback,omitempty"`
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
	return Parse(b)
}

func Parse(b []byte) (Config, error) {
	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, errors.New("config must contain exactly one YAML document")
		}
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
	if _, err := os.Stat(path); err == nil {
		backup := path + ".bak." + time.Now().Format("20060102-150405")
		if current, err := os.ReadFile(path); err == nil {
			if err := os.WriteFile(backup, current, 0600); err != nil {
				return err
			}
		}
	}
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
	userMarks := map[int]string{}
	services := map[string]string{}
	routeTables := map[int]string{}
	subnets := map[string]*net.IPNet{}
	for name := range c.Users {
		if name == "" {
			return errors.New("user has empty name")
		}
	}
	for name, user := range c.Users {
		for _, upstream := range user.AllowedUpstreams {
			if _, ok := c.Upstreams[upstream]; ok == false {
				return errors.New("user " + name + " references missing allowed upstream " + upstream)
			}
		}
		if user.DefaultUpstream != "" {
			if _, ok := c.Upstreams[user.DefaultUpstream]; ok == false {
				return errors.New("user " + name + " references missing default upstream")
			}
			allowedDefault := user.AccessRestricted == false
			for _, upstream := range user.AllowedUpstreams {
				if upstream == user.DefaultUpstream {
					allowedDefault = true
				}
			}
			if allowedDefault == false {
				return errors.New("user " + name + " default upstream is not allowed")
			}
		}
		if user.Output != "" {
			if _, ok := c.Outputs[user.Output]; !ok {
				return errors.New("user " + name + " references missing output " + user.Output)
			}
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
			_, subnet, err := net.ParseCIDR(out.Subnet)
			if err != nil {
				return err
			}
			if subnet.IP.To4() != nil {
				for previous, existing := range subnets {
					if subnet.Contains(existing.IP) || existing.Contains(subnet.IP) {
						return errors.New("output " + name + " subnet overlaps output " + previous)
					}
				}
				subnets[name] = subnet
			}
		}
		if out.Managed && (out.Type == "xray" || out.Type == "sing-box") {
			for _, client := range out.Clients {
				mark := netfilter.UserMark(name, client.Name)
				if previous, ok := userMarks[mark]; ok {
					return errors.New("proxy routing identity collision: " + previous + " and " + client.Name)
				}
				userMarks[mark] = name + ":" + client.Name
			}
		}
		if err := c.validateClients(name, out.Clients); err != nil {
			return err
		}
		if err := validateRoutedOutput(name, out); err != nil {
			return err
		}
	}
	for name, up := range c.Upstreams {
		table := netfilter.RouteTableID(name)
		if previous, ok := routeTables[table]; ok {
			return errors.New("upstream routing table collision: " + previous + " and " + name)
		}
		routeTables[table] = name
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
		if up.FallbackWhenDown != "" {
			if _, ok := c.Upstreams[up.FallbackWhenDown]; !ok {
				return errors.New("upstream " + name + " references missing fallback " + up.FallbackWhenDown)
			}
		}
	}
	defaultUpstream := ""
	for name, up := range c.Upstreams {
		if !up.Default {
			continue
		}
		if defaultUpstream != "" {
			return errors.New("upstream " + name + " conflicts with " + defaultUpstream + ": only one default upstream is allowed")
		}
		defaultUpstream = name
	}
	defaultRule := ""
	for _, rule := range c.Rules {
		if rule.Via == "" {
			return errors.New("rule " + rule.Name + " has empty via")
		}
		if _, ok := c.Upstreams[rule.Via]; !ok {
			return errors.New("rule " + rule.Name + " references missing upstream " + rule.Via)
		}
		if rule.DomainFallbackVia != "" {
			if _, ok := c.Upstreams[rule.DomainFallbackVia]; !ok {
				return errors.New("rule " + rule.Name + " references missing domain fallback " + rule.DomainFallbackVia)
			}
		}
		if rule.Default {
			if defaultRule != "" {
				return errors.New("rule " + rule.Name + " conflicts with " + defaultRule + ": only one default rule is allowed")
			}
			defaultRule = rule.Name
		}
		for _, cidr := range rule.CIDRs {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				return err
			}
		}
	}
	return nil
}

var managedQuickInterface = regexp.MustCompile(`^[a-zA-Z0-9_=+.-]{1,15}$`)

func validateRoutedOutput(name string, out Output) error {
	if out.Type != "wireguard" && out.Type != "amneziawg" && out.Type != "openvpn" {
		return nil
	}
	if out.Managed && (out.Type == "wireguard" || out.Type == "amneziawg") {
		if managedQuickInterface.MatchString(out.Interface) == false || out.Interface == "." || out.Interface == ".." || out.Interface == "lo" || out.Interface == "auto" {
			return errors.New("output " + name + " has invalid managed interface name")
		}
	}
	var subnet *net.IPNet
	if out.Subnet != "" {
		_, parsed, err := net.ParseCIDR(out.Subnet)
		if err != nil {
			return err
		}
		subnet = parsed
	}
	serverIP := net.ParseIP(out.ServerIP)
	if out.Managed {
		if subnet == nil || subnet.IP.To4() == nil {
			return errors.New("output " + name + " requires an IPv4 subnet")
		}
		if usableOutputIP(serverIP, subnet) == false {
			return errors.New("output " + name + " requires a usable server IPv4 address inside its subnet")
		}
	}
	seen := map[string]string{}
	for _, client := range out.Clients {
		if client.IP == "" && out.Managed == false {
			continue
		}
		addressCIDR := client.IP
		if out.Managed == false && net.ParseIP(addressCIDR).To4() != nil {
			addressCIDR += "/32"
		}
		ip, identity, err := net.ParseCIDR(addressCIDR)
		if err != nil {
			return errors.New("output " + name + " client " + client.Name + " requires an IPv4 /32 address")
		}
		prefix, bits := identity.Mask.Size()
		if ip.To4() == nil || bits != 32 || prefix != 32 {
			return errors.New("output " + name + " client " + client.Name + " requires an IPv4 /32 address")
		}
		if usableOutputIP(ip, subnet) == false || ip.Equal(serverIP) {
			return errors.New("output " + name + " client " + client.Name + " requires a usable address inside its subnet distinct from the server")
		}
		address := ip.String()
		if previous, ok := seen[address]; ok {
			return errors.New("output " + name + " clients " + previous + " and " + client.Name + " share an IPv4 address")
		}
		seen[address] = client.Name
	}
	return nil
}

func usableOutputIP(ip net.IP, subnet *net.IPNet) bool {
	if ip == nil || ip.To4() == nil || ip.IsGlobalUnicast() == false || ip.IsLoopback() {
		return false
	}
	if subnet == nil {
		return true
	}
	if subnet.IP.To4() == nil || len(subnet.Mask) != net.IPv4len || subnet.Contains(ip) == false || ip.Equal(subnet.IP) {
		return false
	}
	broadcast := append(net.IP(nil), subnet.IP.To4()...)
	for index, mask := range subnet.Mask {
		broadcast[index] |= ^mask
	}
	return ip.Equal(broadcast) == false
}

func (u Upstream) IsEnabled() bool {
	return u.Enabled == nil || *u.Enabled
}

func (o Output) IsEnabled() bool {
	return o.Enabled == nil || *o.Enabled
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

func (c Config) UserCanUseUpstream(id, upstream string) bool {
	if id == "" {
		return true
	}
	user, ok := c.Users[id]
	if ok == false || user.Disabled {
		return false
	}
	if user.AccessRestricted == false {
		return true
	}
	for _, name := range user.AllowedUpstreams {
		if name == upstream {
			return true
		}
	}
	return false
}
