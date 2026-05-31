package protocols

import (
	"testing"

	"granger/pkg/runner"
)

func TestSupportedInstallers(t *testing.T) {
	want := map[string]bool{
		"amneziawg": false,
		"openvpn":   false,
		"sing-box":  false,
		"snx-rs":    false,
		"wireguard": false,
		"xray":      false,
	}
	for _, installer := range Supported() {
		if _, ok := want[installer.Name()]; !ok {
			t.Fatalf("unexpected protocol installer %q", installer.Name())
		}
		want[installer.Name()] = true
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("missing protocol installer %q", name)
		}
	}
}

func TestUnknownInstallDoesNotRunCommand(t *testing.T) {
	status, results, err := New(runner.NewDryRun()).Install("rm -rf /")
	if err == nil {
		t.Fatal("expected unknown installer error")
	}
	if status.State != "unknown" {
		t.Fatalf("state = %q, want unknown", status.State)
	}
	if len(results) != 1 || results[0].Command != "protocol:rm -rf /" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestManagedExternalInstallersAreWhitelisted(t *testing.T) {
	for _, name := range []string{"amneziawg", "sing-box", "snx-rs", "xray"} {
		status, results, err := New(runner.NewDryRun()).Install(name)
		if err != nil {
			t.Fatalf("Install(%q) returned error: %v", name, err)
		}
		if !status.Installable {
			t.Fatalf("%q should be installable through a managed installer", name)
		}
		if len(results) == 0 {
			t.Fatalf("%q returned no installation steps", name)
		}
	}
}

func TestSingBoxInstallerUsesOfficialRepository(t *testing.T) {
	_, results, err := New(runner.NewDryRun()).Install("sing-box")
	if err != nil {
		t.Fatalf("Install(sing-box) returned error: %v", err)
	}
	if !hasCommand(results, "curl -fsSL https://sing-box.app/gpg.key -o /etc/apt/keyrings/sagernet.asc") {
		t.Fatalf("sing-box installer did not fetch official signing key: %#v", results)
	}
	if !hasCommand(results, "tee /etc/apt/sources.list.d/sagernet.sources") {
		t.Fatalf("sing-box installer did not write apt source: %#v", results)
	}
}

func TestSNXRSInstallerSimulatesBeforeInstall(t *testing.T) {
	_, results, err := New(runner.NewDryRun()).Install("snx-rs")
	if err != nil {
		t.Fatalf("Install(snx-rs) returned error: %v", err)
	}
	if !hasCommand(results, "env DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -s install snx-rs") {
		t.Fatalf("SNX-RS installer did not simulate apt install: %#v", results)
	}
}

func TestXrayInstallerUsesOfficialScript(t *testing.T) {
	_, results, err := New(runner.NewDryRun()).Install("xray")
	if err != nil {
		t.Fatalf("Install(xray) returned error: %v", err)
	}
	if !hasCommand(results, "curl -fsSL https://github.com/XTLS/Xray-install/raw/main/install-release.sh -o /var/lib/granger/installers/xray-install-release.sh") {
		t.Fatalf("Xray installer did not download official installer: %#v", results)
	}
	if !hasCommand(results, "bash /var/lib/granger/installers/xray-install-release.sh install -u root") {
		t.Fatalf("Xray installer did not execute cached installer: %#v", results)
	}
}

func TestAptInstallerUsesAptOnly(t *testing.T) {
	_, results, err := New(runner.NewDryRun()).Install("wireguard")
	if err != nil {
		t.Fatalf("Install(wireguard) returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Command != "env DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get update" {
		t.Fatalf("first command = %q", results[0].Command)
	}
	if results[1].Command != "env DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get install -y --no-install-recommends wireguard-tools" {
		t.Fatalf("install command = %q", results[1].Command)
	}
}

func hasCommand(results []runner.Result, command string) bool {
	for _, result := range results {
		if result.Command == command {
			return true
		}
	}
	return false
}
