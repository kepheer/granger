package snx

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"granger/internal/config"
	"granger/pkg/runner"
)

func TestStartRejectsMissingConfigWithoutCreatingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-vpn.conf")
	manager := New(runner.NewDryRun())
	result := manager.Start(config.Upstream{Config: path}, map[string]string{"username": "test", "password": "test"})
	if result.OK || result.Output != "Add an SNX-RS config before signing in" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) == false {
		t.Fatalf("missing config was created: %v", err)
	}
}

func TestCheckConfigUsesProvidedPathAndRejectsEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom config.conf")
	if err := CheckConfig(dir); err == nil {
		t.Fatal("directory accepted as config")
	}
	if err := os.WriteFile(path, []byte(" \n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckConfig(path); err == nil {
		t.Fatal("empty config accepted")
	}
	if err := os.WriteFile(path, []byte("server-name = vpn.example.test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckConfig(path); err != nil {
		t.Fatal(err)
	}
}

func TestConnectCommandPassesSelectedConfigAsSingleArgument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom config.conf")
	cmd := connectCommand(context.Background(), path)
	want := []string{"snxctl", "--config-file", path, "connect"}
	if reflect.DeepEqual(cmd.Args, want) == false {
		t.Fatalf("args = %q, want %q", cmd.Args, want)
	}
}

func TestMissingDriverDoesNotRewriteConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	path := filepath.Join(dir, "provided.conf")
	body := "server-name = vpn.example.test\nuser-name = original\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	result := New(runner.NewDryRun()).Start(config.Upstream{Config: path}, map[string]string{"username": "replacement", "password": "test"})
	if result.OK || result.Output != "Install the SNX-RS driver before signing in" {
		t.Fatalf("unexpected result: %#v", result)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != body {
		t.Fatalf("config changed: %q, %v", got, err)
	}
}
