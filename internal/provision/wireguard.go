package provision

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"granger/internal/config"
)

func EnsureOutput(name string, out *config.Output) error {
	if out.Type != "wireguard" {
		return nil
	}
	if out.Interface == "" || out.Subnet == "" || out.ListenPort == 0 {
		return errors.New("wireguard output requires interface, subnet, and listen_port")
	}
	if out.ServerIP == "" {
		ip, err := firstUsableIP(out.Subnet)
		if err != nil {
			return err
		}
		out.ServerIP = ip.String()
	}
	if out.Config == "" {
		out.Config = filepath.Join(config.OutputsDir, safeName(name)+".conf")
	}
	if err := RenderWireGuardServer(name, *out); err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/wireguard", 0700); err != nil {
		return err
	}
	link := filepath.Join("/etc/wireguard", safeName(out.Interface)+".conf")
	if _, err := os.Lstat(link); errors.Is(err, os.ErrNotExist) {
		return os.Symlink(out.Config, link)
	}
	return nil
}

func IssueUser(cfg *config.Config, userID string) error {
	user := cfg.Users[userID]
	if user.Output == "" {
		return errors.New("user output is required")
	}
	out, ok := cfg.Outputs[user.Output]
	if !ok {
		return errors.New("unknown user output: " + user.Output)
	}
	if out.Type != "wireguard" {
		return errors.New("automatic client provisioning is currently implemented for WireGuard outputs only")
	}
	if err := EnsureOutput(user.Output, &out); err != nil {
		return err
	}
	clientIP, err := nextClientIP(out)
	if err != nil {
		return err
	}
	serverPublic, err := publicKey(serverPrivateKeyPath(user.Output))
	if err != nil {
		return err
	}
	clientPrivate, err := genKey()
	if err != nil {
		return err
	}
	clientPublic, err := publicKeyFromPrivate(clientPrivate)
	if err != nil {
		return err
	}
	clientPath := filepath.Join(config.OutputsDir, safeName(user.Output)+"-clients", safeName(userID)+".conf")
	clientBody := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32
DNS = %s

[Peer]
PublicKey = %s
Endpoint = %s:%d
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
`, clientPrivate, clientIP, out.ServerIP, serverPublic, cfg.Server.PublicIP, out.ListenPort)
	if err := writeAtomic(clientPath, []byte(clientBody), 0600); err != nil {
		return err
	}
	out.Clients = append(out.Clients, config.Client{Name: userID, User: userID, IP: clientIP.String() + "/32", Config: clientPath, PublicKey: clientPublic})
	cfg.Outputs[user.Output] = out
	user.Client = userID
	cfg.Users[userID] = user
	return RenderWireGuardServer(user.Output, out)
}

func RevokeUser(cfg *config.Config, userID string) error {
	return SetUserDisabled(cfg, userID, true)
}

func SetUserDisabled(cfg *config.Config, userID string, disabled bool) error {
	user, ok := cfg.Users[userID]
	if !ok || user.Output == "" {
		return nil
	}
	out, ok := cfg.Outputs[user.Output]
	if !ok || out.Type != "wireguard" {
		return nil
	}
	for index := range out.Clients {
		if out.Clients[index].User == userID {
			out.Clients[index].Disabled = disabled
		}
	}
	cfg.Outputs[user.Output] = out
	return RenderWireGuardServer(user.Output, out)
}

func RenderWireGuardServer(name string, out config.Output) error {
	if out.Type != "wireguard" {
		return nil
	}
	privatePath := serverPrivateKeyPath(name)
	private, err := ensurePrivateKey(privatePath)
	if err != nil {
		return err
	}
	var body strings.Builder
	_, network, err := net.ParseCIDR(out.Subnet)
	if err != nil {
		return err
	}
	prefix, _ := network.Mask.Size()
	fmt.Fprintf(&body, "[Interface]\nPrivateKey = %s\nAddress = %s/%d\nListenPort = %d\n\n", private, out.ServerIP, prefix, out.ListenPort)
	for _, client := range out.Clients {
		if client.Disabled || client.PublicKey == "" {
			continue
		}
		fmt.Fprintf(&body, "[Peer]\nPublicKey = %s\nAllowedIPs = %s\n\n", client.PublicKey, client.IP)
	}
	return writeAtomic(out.Config, []byte(body.String()), 0600)
}

func serverPrivateKeyPath(outputName string) string {
	return filepath.Join(config.SecretsDir, safeName(outputName)+"-server.key")
}

func ensurePrivateKey(path string) (string, error) {
	if body, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(body)), nil
	}
	key, err := genKey()
	if err != nil {
		return "", err
	}
	return key, writeAtomic(path, []byte(key+"\n"), 0600)
}

func publicKey(path string) (string, error) {
	private, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return publicKeyFromPrivate(strings.TrimSpace(string(private)))
}

func publicKeyFromPrivate(private string) (string, error) {
	return commandWithInput(private+"\n", "wg", "pubkey")
}

func genKey() (string, error) {
	return commandWithInput("", "wg", "genkey")
}

func commandWithInput(stdin, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func nextClientIP(out config.Output) (net.IP, error) {
	_, network, err := net.ParseCIDR(out.Subnet)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{out.ServerIP: true}
	for _, client := range out.Clients {
		ip, _, err := net.ParseCIDR(client.IP)
		if err == nil {
			used[ip.String()] = true
		}
	}
	for ip := incrementIP(network.IP); network.Contains(ip); ip = incrementIP(ip) {
		if !used[ip.String()] {
			return ip, nil
		}
	}
	return nil, errors.New("wireguard output subnet has no available client addresses")
}

func firstUsableIP(subnet string) (net.IP, error) {
	_, network, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, err
	}
	return incrementIP(network.IP), nil
}

func incrementIP(ip net.IP) net.IP {
	next := append(net.IP(nil), ip...)
	for index := len(next) - 1; index >= 0; index-- {
		next[index]++
		if next[index] != 0 {
			break
		}
	}
	return next
}

func writeAtomic(path string, body []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".granger-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func safeName(name string) string {
	var out strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			out.WriteRune(r)
		default:
			out.WriteRune('_')
		}
	}
	return out.String()
}
