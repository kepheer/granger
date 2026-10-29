package installtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseInstallationNeverInstallsProtocolDrivers(t *testing.T) {
	body, err := os.ReadFile("../../scripts/install-granger.sh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "packages")
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "apt-get"), []byte(stub), 0700); err != nil {
		t.Fatal(err)
	}
	script := strings.Replace(string(body), "\nmain \"$@\"", "\ninstall_packages", 1)
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	packages, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"wireguard", "amneziawg", "openvpn", "xray", "sing-box", "snx-rs", "linux-headers", "dkms"} {
		if strings.Contains(string(packages), forbidden) {
			t.Fatalf("base installer installed optional driver: %s", forbidden)
		}
	}
}
