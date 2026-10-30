package provision

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"granger/internal/config"
)

type quickPaths struct {
	outputs   string
	secrets   string
	wireguard string
	amnezia   string
}

func defaultQuickPaths() quickPaths {
	return quickPaths{outputs: config.OutputsDir, secrets: config.SecretsDir, wireguard: "/etc/wireguard", amnezia: "/etc/amnezia/amneziawg"}
}

func EnsureOutput(name string, out *config.Output, publicIP string) error {
	if out.Type != "wireguard" && out.Type != "amneziawg" {
		return ensureManagedOutput(name, out, publicIP)
	}
	return ensureQuickOutput(name, out, defaultQuickPaths())
}

func quickLink(out config.Output, paths quickPaths) string {
	directory := paths.wireguard
	if out.Type == "amneziawg" {
		directory = paths.amnezia
	}
	return filepath.Join(directory, out.Interface+".conf")
}

func validQuickInterface(iface string) bool {
	return iface != "" && iface != "." && iface != ".." && len(iface) <= 15 && safeName(iface) == iface
}

func sameQuickConfig(first, second string) bool {
	firstAbs, firstErr := filepath.Abs(first)
	secondAbs, secondErr := filepath.Abs(second)
	if firstErr != nil || secondErr != nil {
		return false
	}
	if filepath.Clean(firstAbs) == filepath.Clean(secondAbs) {
		return true
	}
	firstReal, firstErr := filepath.EvalSymlinks(firstAbs)
	secondReal, secondErr := filepath.EvalSymlinks(secondAbs)
	return firstErr == nil && secondErr == nil && firstReal == secondReal
}

func verifyQuickLink(out config.Output, paths quickPaths) error {
	if validQuickInterface(out.Interface) == false {
		return errors.New("quick output requires a valid Linux interface name")
	}
	link := quickLink(out, paths)
	info, err := os.Lstat(link)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(link)
		if err != nil {
			return err
		}
		if filepath.IsAbs(target) == false {
			target = filepath.Join(filepath.Dir(link), target)
		}
		if sameQuickConfig(target, out.Config) {
			return nil
		}
		return errors.New("interface config points to an unrelated file: " + link)
	}
	if info.Mode().IsRegular() && sameQuickConfig(link, out.Config) {
		return nil
	}
	return errors.New("interface already has an unrelated configuration: " + link)
}

func activateQuickLink(out config.Output, paths quickPaths) error {
	if err := verifyQuickLink(out, paths); err != nil {
		return err
	}
	link := quickLink(out, paths)
	if _, err := os.Lstat(link); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
		return err
	}
	absolute, err := filepath.Abs(out.Config)
	if err != nil {
		return err
	}
	return os.Symlink(absolute, link)
}

func readQuickOptions(path string) (map[string]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{}
	inInterface := false
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inInterface = strings.EqualFold(trimmed, "[Interface]")
			continue
		}
		if inInterface == false {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if ok == false {
			continue
		}
		value, _, _ = strings.Cut(value, "#")
		fields[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return fields, nil
}

func awgOptionsFromFields(fields map[string]string) map[string]string {
	result := map[string]string{}
	for _, key := range []string{"Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4", "H1", "H2", "H3", "H4", "I1", "I2", "I3", "I4", "I5", "HeaderProtectionKey", "ContentPaddingAddition", "RekeyAfterTime", "RekeyTimeout", "RejectAfterTime", "KeepaliveTimeout", "MaxHandshakeAttempts", "RandomTrailers", "DisableCookies"} {
		if value, ok := fields[strings.ToLower(key)]; ok {
			result[key] = value
		}
	}
	return result
}

func ensureQuickOutput(name string, out *config.Output, paths quickPaths) error {
	next := *out
	if next.Config == "" {
		next.Managed = true
		next.Config = filepath.Join(paths.outputs, safeName(name)+".conf")
	}
	if err := verifyQuickLink(next, paths); err != nil {
		return err
	}
	if next.Managed == false {
		info, err := os.Stat(next.Config)
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() == false {
			return errors.New("external quick configuration is not a regular file")
		}
		return activateQuickLink(next, paths)
	}
	if next.Subnet == "" || next.ListenPort < 1 || next.ListenPort > 65535 {
		return errors.New("managed quick output requires subnet and a valid listen_port")
	}
	if next.ServerIP == "" {
		ip, err := firstUsableIP(next.Subnet)
		if err != nil {
			return err
		}
		next.ServerIP = ip.String()
	}
	if next.Type == "amneziawg" && len(next.ProtocolOptions) == 0 {
		fields, err := readQuickOptions(next.Config)
		if err == nil {
			next.ProtocolOptions = awgOptionsFromFields(fields)
		}
		if errors.Is(err, os.ErrNotExist) {
			options, err := newAWGOptions()
			if err != nil {
				return err
			}
			next.ProtocolOptions = options
		}
		if err != nil && errors.Is(err, os.ErrNotExist) == false {
			return err
		}
	}
	if err := renderQuickServer(name, next, paths); err != nil {
		return err
	}
	if err := activateQuickLink(next, paths); err != nil {
		return err
	}
	*out = next
	return nil
}

func adoptLegacyQuickOutput(name string, out *config.Output, paths quickPaths) error {
	if out.Managed {
		return verifyQuickLink(*out, paths)
	}
	if out.Config == "" {
		return errors.New("automatic profiles require a Granger-managed quick output")
	}
	base, err := filepath.EvalSymlinks(paths.outputs)
	if err != nil {
		return errors.New("automatic profiles require a Granger-managed quick output")
	}
	info, err := os.Lstat(out.Config)
	if err != nil || info.Mode().IsRegular() == false {
		return errors.New("automatic profiles require a Granger-owned quick configuration")
	}
	candidate, err := filepath.EvalSymlinks(out.Config)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(base, candidate)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return errors.New("automatic profiles require a configuration inside the Granger outputs directory")
	}
	keyPath := filepath.Join(paths.secrets, safeName(name)+"-server.key")
	keyInfo, err := os.Lstat(keyPath)
	if err != nil || keyInfo.Mode().IsRegular() == false || keyInfo.Mode().Perm()&0077 != 0 {
		return errors.New("legacy quick output has no protected Granger-owned server secret")
	}
	private, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	if _, err := publicKeyFromPrivate(string(private)); err != nil {
		return errors.New("legacy quick output has an invalid server secret")
	}
	fields, err := readQuickOptions(out.Config)
	if err != nil {
		return err
	}
	if fields["privatekey"] != strings.TrimSpace(string(private)) {
		return errors.New("legacy quick configuration does not match its Granger server secret")
	}
	if err := verifyQuickLink(*out, paths); err != nil {
		return err
	}
	if out.Type == "amneziawg" && len(out.ProtocolOptions) == 0 {
		out.ProtocolOptions = awgOptionsFromFields(fields)
	}
	out.Managed = true
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
	if out.Type != "wireguard" && out.Type != "amneziawg" {
		return issueManagedUser(cfg, userID)
	}
	if err := adoptLegacyQuickOutput(user.Output, &out, defaultQuickPaths()); err != nil {
		return err
	}
	if err := EnsureOutput(user.Output, &out, cfg.Server.PublicIP); err != nil {
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
	clientDNS := out.ServerIP
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
`, clientPrivate, clientIP, clientDNS, serverPublic, cfg.Server.PublicIP, out.ListenPort)
	if out.Type == "amneziawg" {
		clientBody = strings.Replace(clientBody, "[Interface]\n", "[Interface]\n"+renderAWGOptions(out.ProtocolOptions), 1)
	}
	if err := writeAtomic(clientPath, []byte(clientBody), 0600); err != nil {
		return err
	}
	out.Clients = append(out.Clients, config.Client{Name: userID, User: userID, IP: clientIP.String() + "/32", Config: clientPath, PublicKey: clientPublic})
	cfg.Outputs[user.Output] = out
	user.Client = userID
	cfg.Users[userID] = user
	return renderOutput(user.Output, RuntimeOutput(*cfg, out), cfg.Server.PublicIP)
}

func RevokeUser(cfg *config.Config, userID string) error {
	return SetUserDisabled(cfg, userID, true)
}

func SetUserDisabled(cfg *config.Config, userID string, disabled bool) error {
	user, exists := cfg.Users[userID]
	if exists == false {
		return nil
	}
	user.Disabled = disabled
	cfg.Users[userID] = user
	for name, out := range cfg.Outputs {
		for _, client := range out.Clients {
			if client.User == userID {
				if err := RenderProfiles(*cfg, name); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

func RenderWireGuardServer(name string, out config.Output) error {
	return renderQuickServer(name, out, defaultQuickPaths())
}

func renderQuickServer(name string, out config.Output, paths quickPaths) error {
	if out.Type != "wireguard" && out.Type != "amneziawg" {
		return nil
	}
	if out.Managed == false {
		return errors.New("automatic profiles require a Granger-managed quick output")
	}
	if err := verifyQuickLink(out, paths); err != nil {
		return err
	}
	privatePath := filepath.Join(paths.secrets, safeName(name)+"-server.key")
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
	if out.Type == "amneziawg" {
		bodyString := body.String()
		body.Reset()
		body.WriteString(strings.Replace(bodyString, "[Interface]\n", "[Interface]\n"+renderAWGOptions(out.ProtocolOptions), 1))
	}
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
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(private))
	if err != nil {
		return "", err
	}
	key, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()), nil
}

func genKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[0] &= 248
	raw[31] = (raw[31] & 127) | 64
	return base64.StdEncoding.EncodeToString(raw), nil
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
		if network.Contains(incrementIP(ip)) == false {
			break
		}
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
