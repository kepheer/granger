package driver

import (
	"granger/internal/config"
	"granger/pkg/runner"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditDryRunDoesNotModifyWireGuardConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wg.conf")
	original := "[Interface]\nPrivateKey = test\nDNS = 1.1.1.1\n[Peer]\nAllowedIPs = 0.0.0.0/0\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	up := config.Upstream{Type: "wireguard", Interface: "audit0", Config: path}
	WireGuardUpstream{}.NormalizeConfig("audit", up, ApplyContext{Runner: runner.NewDryRun()})
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatalf("dry-run modified input config:\n%s", after)
	}
}

func TestDryRunDoesNotActivateConfig(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.conf")
	dst := filepath.Join(dir, "active", "target.conf")
	if err := os.WriteFile(src, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	result := activateConfig("preview", src, dst, ApplyContext{Runner: runner.NewDryRun()})
	if result.OK == false {
		t.Fatal(result)
	}
	if _, err := os.Lstat(filepath.Dir(dst)); os.IsNotExist(err) == false {
		t.Fatalf("dry-run created activation directory: %v", err)
	}
}
