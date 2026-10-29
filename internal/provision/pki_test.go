package provision

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestIssuedCertificateVerifiesAgainstPrivateAuthority(t *testing.T) {
	dir := t.TempDir()
	ca, err := ensureAuthority(dir)
	if err != nil {
		t.Fatal(err)
	}
	cert, key, err := ensureCertificate(dir, "server", "192.0.2.4", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tls.X509KeyPair(cert, key); err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(cert)
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.certificate)
	if _, err := parsed.Verify(x509.VerifyOptions{Roots: roots, DNSName: "192.0.2.4"}); err != nil {
		t.Fatal(err)
	}
	again, _, err := ensureCertificate(dir, "server", "192.0.2.4", false)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(cert) {
		t.Fatal("certificate rotated on unchanged configuration")
	}
	stat, err := os.Stat(filepath.Join(dir, "ca.key"))
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0600 {
		t.Fatal("CA private key has insecure permissions")
	}
}

func TestClientCertificateCannotImpersonateServer(t *testing.T) {
	dir := t.TempDir()
	cert, _, err := ensureCertificate(dir, "alice", "", true)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(cert)
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := ensureAuthority(dir)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.certificate)
	if _, err := parsed.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err == nil {
		t.Fatal("client certificate accepted as server")
	}
}

func TestAWGParametersAreSharedAndUnique(t *testing.T) {
	options, err := newAWGOptions()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, key := range []string{"H1", "H2", "H3", "H4"} {
		if seen[options[key]] || options[key] == "" {
			t.Fatal("invalid AWG headers")
		}
		seen[options[key]] = true
	}
}

func TestQuickKeysDoNotRequireInstalledWireGuardTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	private, err := genKey()
	if err != nil {
		t.Fatal(err)
	}
	public, err := publicKeyFromPrivate(private)
	if err != nil {
		t.Fatal(err)
	}
	if len(private) != 44 || len(public) != 44 || private == public {
		t.Fatal("invalid WireGuard-compatible key pair")
	}
}
