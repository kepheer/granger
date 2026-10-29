package driver

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"granger/internal/config"
	"granger/pkg/runner"
)

func SyncQuickClients(name string, out config.Output, ctx ApplyContext) []runner.Result {
	tool := "wg"
	quick := "wg-quick"
	if out.Type == "amneziawg" {
		tool = "awg"
		quick = "awg-quick"
	}
	if ctx.Runner.DryRun {
		return []runner.Result{ctx.Runner.Run("Synchronize clients "+name, 10*time.Second, nil, tool, "syncconf", out.Interface, "/dev/stdin")}
	}
	running := ctx.Runner.Run("Check client interface "+name, 5*time.Second, nil, "ip", "link", "show", "dev", out.Interface)
	if running.OK == false {
		service := wireGuardService(out.Service, out.Interface)
		if out.Type == "amneziawg" {
			service = amneziaWGService(out.Service, out.Interface)
		}
		return []runner.Result{ctx.Runner.Run("Start client interface "+name, 30*time.Second, nil, "systemctl", "start", service)}
	}
	commandCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	body, err := exec.CommandContext(commandCtx, quick, "strip", out.Interface).Output()
	if err != nil {
		return []runner.Result{{Title: "Prepare client synchronization " + name, Command: quick + " strip " + out.Interface, OK: false, Status: "error", Output: err.Error()}}
	}
	return []runner.Result{ctx.Runner.Run("Synchronize clients "+name, 10*time.Second, bytes.NewReader(body), tool, "syncconf", out.Interface, "/dev/stdin")}
}

func UpdateOpenVPNClients(name, userID string, revoked bool, ctx ApplyContext) []runner.Result {
	if revoked == false || ctx.Runner.DryRun {
		return []runner.Result{{Title: "Update OpenVPN client " + userID, OK: true, Status: "ok", Output: "client allowlist updated"}}
	}
	path := filepath.Join(config.SecretsDir, safeName(name), "management.sock")
	return disconnectOpenVPNClient(path, userID)
}

func disconnectOpenVPNClient(path, userID string) []runner.Result {
	connection, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		return []runner.Result{{Title: "Disconnect OpenVPN client " + userID, OK: false, Status: "error", Output: err.Error()}}
	}
	return disconnectOpenVPNSession(connection, userID)
}

func disconnectOpenVPNSession(connection net.Conn, userID string) []runner.Result {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.WriteString(connection, "kill "+safeName(userID)+"\n"); err != nil {
		return []runner.Result{{Title: "Disconnect OpenVPN client", OK: false, Status: "error", Output: err.Error()}}
	}
	scanner := bufio.NewScanner(io.LimitReader(connection, 8192))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "SUCCESS:") || (strings.HasPrefix(line, "ERROR:") && strings.Contains(line, "not found")) {
			_, _ = io.WriteString(connection, "quit\n")
			return []runner.Result{{Title: "Disconnect OpenVPN client " + userID, OK: true, Status: "ok", Output: "revoked client session disconnected"}}
		}
		if strings.HasPrefix(line, "ERROR:") {
			return []runner.Result{{Title: "Disconnect OpenVPN client " + userID, OK: false, Status: "error", Output: line}}
		}
	}
	err := scanner.Err()
	message := "OpenVPN did not confirm client disconnection"
	if err != nil {
		message = err.Error()
	}
	return []runner.Result{{Title: "Disconnect OpenVPN client " + userID, OK: false, Status: "error", Output: message}}
}
