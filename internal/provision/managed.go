package provision

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"granger/internal/config"
	"granger/pkg/netfilter"
)

func newAWGOptions() (map[string]string, error) {
	options := map[string]string{"Jc": "4", "Jmin": "40", "Jmax": "70", "S1": "64", "S2": "32"}
	seen := map[string]bool{}
	for _, key := range []string{"H1", "H2", "H3", "H4"} {
		for {
			n, err := rand.Int(rand.Reader, big.NewInt(2147483642))
			if err != nil {
				return nil, err
			}
			value := fmt.Sprint(n.Int64() + 5)
			if seen[value] {
				continue
			}
			options[key] = value
			seen[value] = true
			break
		}
	}
	return options, nil
}

func renderAWGOptions(options map[string]string) string {
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var body strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&body, "%s = %s\n", key, options[key])
	}
	return body.String()
}

func managedSecrets(name string) string { return filepath.Join(config.SecretsDir, safeName(name)) }

func ensureManagedOutput(name string, out *config.Output, publicIP string) error {
	switch out.Type {
	case "openvpn", "xray", "sing-box":
	default:
		return errors.New("unsupported output protocol: " + out.Type)
	}
	if out.Config != "" && out.Managed == false {
		return nil
	}
	if out.ListenPort < 1 || out.ListenPort > 65535 {
		return errors.New("entrypoint port must be between 1 and 65535")
	}
	if publicIP == "" {
		return errors.New("server public IP or hostname is required to issue profiles")
	}
	out.Managed = true
	extension := ".json"
	if out.Type == "openvpn" {
		extension = ".conf"
	}
	if out.Config == "" {
		out.Config = filepath.Join(config.OutputsDir, safeName(name)+extension)
	}
	if out.Type == "openvpn" && out.ServerIP == "" {
		ip, err := firstUsableIP(out.Subnet)
		if err != nil {
			return err
		}
		out.ServerIP = ip.String()
	}
	if out.Type == "xray" || out.Type == "sing-box" {
		if out.Service == "" {
			out.Service = "granger-" + out.Type + "-" + safeName(name) + ".service"
		}
	}
	return renderOutput(name, *out, publicIP)
}

func issueManagedUser(cfg *config.Config, userID string) error {
	user := cfg.Users[userID]
	out := cfg.Outputs[user.Output]
	if out.Managed == false {
		return errors.New("automatic profiles require a Granger-managed entrypoint")
	}
	if err := EnsureOutput(user.Output, &out, cfg.Server.PublicIP); err != nil {
		return err
	}
	var clientIP string
	if out.Type == "openvpn" {
		ip, err := nextClientIP(out)
		if err != nil {
			return err
		}
		clientIP = ip.String() + "/32"
	}
	extension := ".json"
	if out.Type == "openvpn" {
		extension = ".ovpn"
	}
	path := filepath.Join(config.OutputsDir, safeName(user.Output)+"-clients", safeName(userID)+extension)
	client := config.Client{Name: userID, User: userID, IP: clientIP, Config: path}
	body, err := managedClientConfig(user.Output, out, client, cfg.Server)
	if err != nil {
		return err
	}
	if err := writeAtomic(path, body, 0600); err != nil {
		return err
	}
	out.Clients = append(out.Clients, client)
	if err := renderOutput(user.Output, out, cfg.Server.PublicIP); err != nil {
		return err
	}
	cfg.Outputs[user.Output] = out
	user.Client = userID
	cfg.Users[userID] = user
	return nil
}

func credential(dir, user string) (string, error) {
	path := filepath.Join(dir, "users", safeName(user)+".secret")
	body, err := os.ReadFile(path)
	if err == nil {
		return strings.TrimSpace(string(body)), nil
	}
	if os.IsNotExist(err) == false {
		return "", err
	}
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(value)
	return secret, writeAtomic(path, []byte(secret+"\n"), 0600)
}

func renderOutput(name string, out config.Output, publicIP string) error {
	if out.Type == "wireguard" || out.Type == "amneziawg" {
		return RenderWireGuardServer(name, out)
	}
	if out.Managed == false {
		return errors.New("cannot change users on an externally managed entrypoint")
	}
	dir := managedSecrets(name)
	if _, _, err := ensureCertificate(dir, "server", publicIP, false); err != nil {
		return err
	}
	if out.Type == "openvpn" {
		return renderOpenVPN(name, out, dir)
	}
	var users []map[string]any
	for _, client := range out.Clients {
		if client.Disabled {
			continue
		}
		password, err := credential(dir, client.Name)
		if err != nil {
			return err
		}
		if out.Type == "xray" {
			users = append(users, map[string]any{"email": client.Name, "password": password})
		}
		if out.Type == "sing-box" {
			users = append(users, map[string]any{"name": client.Name, "password": password})
		}
	}
	if len(users) == 0 {
		users = []map[string]any{}
	}
	var body any
	var outbounds []any
	var routing []any
	for _, client := range out.Clients {
		if client.Disabled {
			continue
		}
		tag := "user-" + client.Name
		mark := netfilter.UserMark(name, client.Name)
		if out.Type == "xray" {
			outbounds = append(outbounds, map[string]any{"tag": tag, "protocol": "freedom", "settings": map[string]any{"domainStrategy": "UseIPv4"}, "streamSettings": map[string]any{"sockopt": map[string]any{"mark": mark}}})
			routing = append(routing, map[string]any{"type": "field", "user": []string{client.Name}, "outboundTag": tag})
		}
		if out.Type == "sing-box" {
			outbounds = append(outbounds, map[string]any{"tag": tag, "type": "direct", "routing_mark": mark})
			routing = append(routing, map[string]any{"auth_user": []string{client.Name}, "outbound": tag})
		}
	}
	if len(outbounds) == 0 {
		if out.Type == "xray" {
			outbounds = []any{map[string]any{"protocol": "blackhole", "tag": "blocked"}}
		}
		if out.Type == "sing-box" {
			outbounds = []any{map[string]any{"type": "direct", "tag": "unused"}}
		}
	}
	if out.Type == "xray" {
		body = map[string]any{
			"log":       map[string]any{"loglevel": "warning"},
			"dns":       map[string]any{"servers": []string{"tcp+local://127.0.0.1"}, "queryStrategy": "UseIPv4", "disableCache": true},
			"inbounds":  []any{map[string]any{"listen": "0.0.0.0", "port": out.ListenPort, "protocol": "trojan", "settings": map[string]any{"clients": users}, "streamSettings": map[string]any{"network": "tcp", "security": "tls", "tlsSettings": map[string]any{"minVersion": "1.2", "certificates": []any{map[string]any{"certificateFile": filepath.Join(dir, "server.crt"), "keyFile": filepath.Join(dir, "server.key")}}}}}},
			"outbounds": outbounds,
			"routing":   map[string]any{"rules": routing},
		}
	}
	if out.Type == "sing-box" {
		body = map[string]any{
			"log":       map[string]any{"level": "warn"},
			"dns":       map[string]any{"servers": []any{map[string]any{"type": "udp", "tag": "granger-dns", "server": "127.0.0.1"}}, "strategy": "ipv4_only", "disable_cache": true},
			"inbounds":  []any{map[string]any{"type": "trojan", "tag": "clients", "listen": "0.0.0.0", "listen_port": out.ListenPort, "users": users, "tls": map[string]any{"enabled": true, "min_version": "1.2", "certificate_path": filepath.Join(dir, "server.crt"), "key_path": filepath.Join(dir, "server.key")}}},
			"outbounds": outbounds,
			"route":     map[string]any{"rules": routing, "default_domain_resolver": "granger-dns"},
		}
	}
	if body == nil {
		return errors.New("unsupported managed output: " + out.Type)
	}
	return writeJSONConfig(out.Config, body)
}

func managedClientConfig(name string, out config.Output, client config.Client, server config.Server) ([]byte, error) {
	dir := managedSecrets(name)
	ca, err := ensureAuthority(dir)
	if err != nil {
		return nil, err
	}
	if out.Type == "openvpn" {
		cert, key, err := ensureCertificate(dir, client.Name, "", true)
		if err != nil {
			return nil, err
		}
		body := fmt.Sprintf("client\ndev tun\ndev-type tun\nproto udp\nremote %s %d\nresolv-retry infinite\nnobind\npersist-key\npersist-tun\nremote-cert-tls server\ntls-version-min 1.2\ndata-ciphers AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305\nverb 3\n<ca>\n%s</ca>\n<cert>\n%s</cert>\n<key>\n%s</key>\n", server.PublicIP, out.ListenPort, ca.pem, cert, key)
		return []byte(body), nil
	}
	password, err := credential(dir, client.Name)
	if err != nil {
		return nil, err
	}
	var body any
	if out.Type == "xray" {
		body = map[string]any{
			"inbounds":  []any{map[string]any{"listen": "127.0.0.1", "port": 1080, "protocol": "socks", "settings": map[string]any{"udp": true}}},
			"outbounds": []any{map[string]any{"protocol": "trojan", "settings": map[string]any{"servers": []any{map[string]any{"address": server.PublicIP, "port": out.ListenPort, "password": password}}}, "streamSettings": map[string]any{"network": "tcp", "security": "tls", "tlsSettings": map[string]any{"serverName": server.PublicIP, "allowInsecure": false, "disableSystemRoot": true, "certificates": []any{map[string]any{"usage": "verify", "certificate": strings.Split(strings.TrimSpace(string(ca.pem)), "\n")}}}}}},
		}
	}
	if out.Type == "sing-box" {
		body = map[string]any{
			"inbounds":  []any{map[string]any{"type": "mixed", "listen": "127.0.0.1", "listen_port": 1080}},
			"outbounds": []any{map[string]any{"type": "trojan", "tag": "proxy", "server": server.PublicIP, "server_port": out.ListenPort, "password": password, "tls": map[string]any{"enabled": true, "server_name": server.PublicIP, "insecure": false, "certificate": strings.Split(strings.TrimSpace(string(ca.pem)), "\n")}}},
		}
	}
	if body == nil {
		return nil, errors.New("unsupported client protocol: " + out.Type)
	}
	return json.MarshalIndent(body, "", "  ")
}

func writeJSONConfig(path string, body any) error {
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'), 0600)
}

func renderOpenVPN(name string, out config.Output, dir string) error {
	_, subnet, err := net.ParseCIDR(out.Subnet)
	if err != nil {
		return err
	}
	ccd := filepath.Join(dir, "ccd")
	if err := os.MkdirAll(ccd, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(ccd)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.Remove(filepath.Join(ccd, entry.Name())); err != nil {
			return err
		}
	}
	for _, client := range out.Clients {
		if client.Disabled {
			continue
		}
		ip, _, err := net.ParseCIDR(client.IP)
		if err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(ccd, safeName(client.Name)), []byte(fmt.Sprintf("ifconfig-push %s %s\n", ip, net.IP(subnet.Mask))), 0600); err != nil {
			return err
		}
	}
	body := fmt.Sprintf("port %d\nproto udp\ndev-type tun\ndev %s\ntopology subnet\nserver %s %s\nca %s\ncert %s\nkey %s\ndh none\ntls-version-min 1.2\nverify-client-cert require\ndata-ciphers AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305\nclient-config-dir %s\nccd-exclusive\nkeepalive 10 120\npersist-key\npersist-tun\npush \"redirect-gateway def1\"\nverb 3\n", out.ListenPort, out.Interface, subnet.IP, net.IP(subnet.Mask), filepath.Join(dir, "ca.crt"), filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key"), ccd)
	body += fmt.Sprintf("push \"dhcp-option DNS %s\"\n", out.ServerIP)
	body += fmt.Sprintf("management %s unix\n", filepath.Join(dir, "management.sock"))
	return writeAtomic(out.Config, []byte(body), 0600)
}
