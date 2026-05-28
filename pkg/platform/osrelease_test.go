package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadOSRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte("ID=debian\nVERSION_ID=\"12\"\nPRETTY_NAME=\"Debian GNU/Linux 12 (bookworm)\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadOSRelease(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "debian" {
		t.Fatalf("ID = %q, want debian", got.ID)
	}
	if got.VersionID != "12" {
		t.Fatalf("VersionID = %q, want 12", got.VersionID)
	}
	if got.PrettyName != "Debian GNU/Linux 12 (bookworm)" {
		t.Fatalf("PrettyName = %q", got.PrettyName)
	}
	if got.Arch == "" {
		t.Fatal("Arch is empty")
	}
}

func TestReadOSReleaseIgnoresCommentsAndMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte("# comment\nBROKEN\nID=ubuntu\nVERSION_CODENAME='noble'\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadOSRelease(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "ubuntu" {
		t.Fatalf("ID = %q, want ubuntu", got.ID)
	}
	if _, ok := got.Values["BROKEN"]; ok {
		t.Fatal("malformed line was parsed")
	}
	if got.Values["VERSION_CODENAME"] != "noble" {
		t.Fatalf("VERSION_CODENAME = %q, want noble", got.Values["VERSION_CODENAME"])
	}
}
