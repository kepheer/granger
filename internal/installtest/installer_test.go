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

func TestReleaseVersionSurvivesOSDetection(t *testing.T) {
	body, err := os.ReadFile("../../scripts/install-granger.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, release := range []string{
		"ID=debian\nVERSION_ID=13\nVERSION=\"13 (trixie)\"\n",
		"ID=ubuntu\nVERSION_ID=24.04\nVERSION=\"24.04 LTS (Noble Numbat)\"\n",
	} {
		dir := t.TempDir()
		osRelease := filepath.Join(dir, "os-release")
		if err := os.WriteFile(osRelease, []byte(release), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "dpkg"), []byte("#!/bin/sh\nprintf 'amd64\\n'\n"), 0700); err != nil {
			t.Fatal(err)
		}
		script := strings.ReplaceAll(string(body), "/etc/os-release", osRelease)
		script = strings.Replace(script, "\nmain \"$@\"", "\ndetect_os\nversion_tarball_url \"$RELEASE_VERSION\"", 1)
		cmd := exec.Command("/bin/sh", "-c", script)
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GRANGER_VERSION=v1.0.0-rc.1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, output)
		}
		want := "https://github.com/kepheer/granger/releases/download/v1.0.0-rc.1/granger_1.0.0-rc.1_linux_amd64.tar.gz"
		if strings.TrimSpace(string(output)) != want {
			t.Fatalf("unexpected release URL: %s", output)
		}
	}
}
