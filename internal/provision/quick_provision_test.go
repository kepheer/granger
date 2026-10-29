package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"granger/internal/config"
)

func quickTestPaths(t *testing.T) quickPaths {
	t.Helper()
	root := t.TempDir()
	return quickPaths{outputs: filepath.Join(root, "outputs"), secrets: filepath.Join(root, "secrets"), wireguard: filepath.Join(root, "wireguard"), amnezia: filepath.Join(root, "amneziawg")}
}

func quickTestOutput(protocol string) config.Output {
	return config.Output{Type: protocol, Interface: "vpn0", Subnet: "10.66.0.0/24", ServerIP: "10.66.0.1", ListenPort: 51820}
}

func readQuickTestFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func writeQuickTestFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestUnmanagedQuickConfigurationIsPreserved(t *testing.T) {
	for _, protocol := range []string{"wireguard", "amneziawg"} {
		t.Run(protocol, func(t *testing.T) {
			paths := quickTestPaths(t)
			out := quickTestOutput(protocol)
			out.Config = filepath.Join(t.TempDir(), "external.conf")
			original := "[Interface]\nPrivateKey = externally-owned\nAddress = 10.66.0.1/24\n"
			writeQuickTestFile(t, out.Config, original, 0640)
			if err := ensureQuickOutput("external", &out, paths); err != nil {
				t.Fatal(err)
			}
			if readQuickTestFile(t, out.Config) != original || out.Managed {
				t.Fatal("unmanaged configuration was changed or adopted")
			}
			info, err := os.Stat(out.Config)
			if err != nil || info.Mode().Perm() != 0640 {
				t.Fatal("external configuration permissions changed")
			}
			target, err := os.Readlink(quickLink(out, paths))
			if err != nil || sameQuickConfig(target, out.Config) == false {
				t.Fatal("external configuration was not activated correctly")
			}
			if _, err := os.Stat(paths.secrets); os.IsNotExist(err) == false {
				t.Fatal("external configuration created provisioning secrets")
			}
			if err := renderQuickServer("external", out, paths); err == nil {
				t.Fatal("unmanaged configuration could be rendered automatically")
			}
			if readQuickTestFile(t, out.Config) != original {
				t.Fatal("unmanaged renderer overwrote the external configuration")
			}
		})
	}
}

func TestIssueUserRejectsUnmanagedExternalQuickConfiguration(t *testing.T) {
	for _, protocol := range []string{"wireguard", "amneziawg"} {
		t.Run(protocol, func(t *testing.T) {
			out := quickTestOutput(protocol)
			out.Config = filepath.Join(t.TempDir(), "external.conf")
			original := "[Interface]\nPrivateKey = externally-owned\n"
			writeQuickTestFile(t, out.Config, original, 0600)
			cfg := config.Default("192.0.2.1", "eth0", "192.0.2.254")
			cfg.Outputs["external"] = out
			cfg.Users["alice"] = config.User{Output: "external"}
			if err := IssueUser(&cfg, "alice"); err == nil {
				t.Fatal("unmanaged external quick output issued an automatic profile")
			}
			if readQuickTestFile(t, out.Config) != original || cfg.Outputs["external"].Managed || len(cfg.Outputs["external"].Clients) != 0 {
				t.Fatal("rejected provisioning changed the external configuration")
			}
		})
	}
}

func TestQuickProvisioningRejectsUnrelatedInterfaceConfigurationBeforeWrites(t *testing.T) {
	for _, protocol := range []string{"wireguard", "amneziawg"} {
		for _, symlink := range []bool{false, true} {
			t.Run(protocol+map[bool]string{false: "-file", true: "-symlink"}[symlink], func(t *testing.T) {
				paths := quickTestPaths(t)
				out := quickTestOutput(protocol)
				unrelated := filepath.Join(t.TempDir(), "unrelated.conf")
				original := "[Interface]\nPrivateKey = unrelated\n"
				writeQuickTestFile(t, unrelated, original, 0600)
				link := quickLink(out, paths)
				if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
					t.Fatal(err)
				}
				if symlink {
					if err := os.Symlink(unrelated, link); err != nil {
						t.Fatal(err)
					}
				}
				if symlink == false {
					writeQuickTestFile(t, link, original, 0600)
				}
				if err := ensureQuickOutput("new-output", &out, paths); err == nil {
					t.Fatal("unrelated interface configuration accepted")
				}
				if readQuickTestFile(t, unrelated) != original || readQuickTestFile(t, link) != original {
					t.Fatal("unrelated interface configuration was modified")
				}
				if symlink {
					target, err := os.Readlink(link)
					if err != nil || target != unrelated {
						t.Fatal("unrelated symlink was replaced")
					}
				}
				if _, err := os.Stat(paths.outputs); os.IsNotExist(err) == false {
					t.Fatal("provisioning wrote outputs before checking the interface")
				}
				if _, err := os.Stat(paths.secrets); os.IsNotExist(err) == false {
					t.Fatal("provisioning wrote secrets before checking the interface")
				}
				if out.Managed || out.Config != "" {
					t.Fatal("rejected provisioning changed output metadata")
				}
			})
		}
	}
}

func legacyQuickFixture(t *testing.T, protocol string) (quickPaths, config.Output, string) {
	t.Helper()
	paths := quickTestPaths(t)
	out := quickTestOutput(protocol)
	out.Config = filepath.Join(paths.outputs, "legacy.conf")
	private, err := genKey()
	if err != nil {
		t.Fatal(err)
	}
	body := "[Interface]\nPrivateKey = " + private + "\nAddress = 10.66.0.1/24\nListenPort = 51820\n"
	if protocol == "amneziawg" {
		body += "Jc = 5\nJmin = 30\nJmax = 80\nS1 = 50\nS2 = 60\nH1 = 101\nH2 = 102\nH3 = 103\nH4 = 104\n"
	}
	writeQuickTestFile(t, out.Config, body, 0600)
	writeQuickTestFile(t, filepath.Join(paths.secrets, "legacy-server.key"), private+"\n", 0600)
	if err := activateQuickLink(out, paths); err != nil {
		t.Fatal(err)
	}
	return paths, out, body
}

func TestLegacyQuickMigrationRequiresOwnedMatchingSecret(t *testing.T) {
	for _, protocol := range []string{"wireguard", "amneziawg"} {
		t.Run(protocol, func(t *testing.T) {
			paths, out, original := legacyQuickFixture(t, protocol)
			if err := adoptLegacyQuickOutput("legacy", &out, paths); err != nil {
				t.Fatal(err)
			}
			if out.Managed == false {
				t.Fatal("verified legacy configuration was not marked managed")
			}
			if readQuickTestFile(t, out.Config) != original {
				t.Fatal("migration modified the existing configuration before provisioning")
			}
			if err := ensureQuickOutput("legacy", &out, paths); err != nil {
				t.Fatal(err)
			}
			if protocol == "amneziawg" && (out.ProtocolOptions["H1"] != "101" || out.ProtocolOptions["Jc"] != "5") {
				t.Fatal("legacy AWG transport settings were rotated")
			}
			key := strings.TrimSpace(readQuickTestFile(t, filepath.Join(paths.secrets, "legacy-server.key")))
			if strings.Contains(readQuickTestFile(t, out.Config), "PrivateKey = "+key) == false {
				t.Fatal("legacy server identity changed")
			}
		})
	}
}

func TestLegacyQuickMigrationRejectsUnrelatedConfigAndSecret(t *testing.T) {
	for _, kind := range []string{"outside-outputs", "parent-symlink", "mismatched-secret", "unprotected-secret", "wrong-link"} {
		t.Run(kind, func(t *testing.T) {
			paths, out, original := legacyQuickFixture(t, "wireguard")
			keyPath := filepath.Join(paths.secrets, "legacy-server.key")
			if kind == "outside-outputs" {
				out.Config = filepath.Join(t.TempDir(), "external.conf")
				writeQuickTestFile(t, out.Config, original, 0600)
			}
			if kind == "parent-symlink" {
				external := t.TempDir()
				writeQuickTestFile(t, filepath.Join(external, "external.conf"), original, 0600)
				if err := os.Symlink(external, filepath.Join(paths.outputs, "outside")); err != nil {
					t.Fatal(err)
				}
				out.Config = filepath.Join(paths.outputs, "outside", "external.conf")
			}
			if kind == "mismatched-secret" {
				other, err := genKey()
				if err != nil {
					t.Fatal(err)
				}
				writeQuickTestFile(t, keyPath, other+"\n", 0600)
			}
			if kind == "unprotected-secret" {
				if err := os.Chmod(keyPath, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "wrong-link" {
				link := quickLink(out, paths)
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(t.TempDir(), "other.conf"), link); err != nil {
					t.Fatal(err)
				}
			}
			if err := adoptLegacyQuickOutput("legacy", &out, paths); err == nil {
				t.Fatal("unverified legacy quick configuration was adopted")
			}
			if out.Managed || readQuickTestFile(t, out.Config) != original {
				t.Fatal("rejected legacy migration modified the supplied configuration")
			}
		})
	}
}

func TestNewManagedQuickOutputsRemainUsableAndAWGOptionsStayStable(t *testing.T) {
	for _, protocol := range []string{"wireguard", "amneziawg"} {
		t.Run(protocol, func(t *testing.T) {
			paths := quickTestPaths(t)
			out := quickTestOutput(protocol)
			if err := ensureQuickOutput("managed", &out, paths); err != nil {
				t.Fatal(err)
			}
			if out.Managed == false || out.Config == "" {
				t.Fatal("new output was not provisioned")
			}
			first := readQuickTestFile(t, out.Config)
			if strings.Contains(first, "PrivateKey = ") == false {
				t.Fatal("missing server key")
			}
			target, err := os.Readlink(quickLink(out, paths))
			if err != nil || sameQuickConfig(target, out.Config) == false {
				t.Fatal("managed interface link is incorrect")
			}
			if protocol == "amneziawg" {
				if out.ProtocolOptions["H1"] == "" {
					t.Fatal("AWG options were not generated")
				}
				out.ProtocolOptions = nil
			}
			if err := ensureQuickOutput("managed", &out, paths); err != nil {
				t.Fatal(err)
			}
			if readQuickTestFile(t, out.Config) != first {
				t.Fatal("unchanged output rotated its key or AWG parameters")
			}
		})
	}
}
