package webgui

import (
	"granger/pkg/runner"
	"os"
	"path/filepath"
	"testing"
)

func TestCreatingEntrypointCannotInstallMissingProtocol(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "apt-called")
	if err := os.Symlink("/bin/sh", filepath.Join(dir, "sh")); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf called > '" + marker + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "apt-get"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	_, err := (apiServer{runner: runner.New()}).ensureProtocol("openvpn")
	if err == nil {
		t.Fatal("missing protocol accepted")
	}
	if _, err := os.Stat(marker); os.IsNotExist(err) == false {
		t.Fatal("entrypoint creation invoked package manager")
	}
}
