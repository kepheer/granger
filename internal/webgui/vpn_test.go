package webgui

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"granger/internal/config"
)

func TestVPNGUIStartsAndClosesWithEntrypoints(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	cfg := config.Config{Server: config.Server{GUIVPNPort: port}, Outputs: map[string]config.Output{
		"awg": {Interface: "awg0", ServerIP: "127.0.0.1"},
	}}
	v := vpnGUI{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "granger") }), servers: map[string]*http.Server{}}
	defer func() {
		for _, server := range v.servers {
			server.Close()
		}
	}()
	v.reconcile(cfg, "127.0.0.1:1984")
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	response, err := client.Get("http://" + address)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "granger" {
		t.Fatalf("unexpected GUI response %q", body)
	}
	v.reconcile(cfg, "127.0.0.1:1984")
	if len(v.servers) != 1 {
		t.Fatalf("duplicate listeners: %d", len(v.servers))
	}
	cfg.Server.GUIVPNPort = 0
	v.reconcile(cfg, "127.0.0.1:1984")
	if len(v.servers) != 0 {
		t.Fatal("disabled GUI retained listeners")
	}
	if response, err := client.Get("http://" + address); err == nil {
		response.Body.Close()
		t.Fatal("disabled interface still serves GUI")
	}
	cfg.Server.GUIVPNPort = 0
	v.reconcile(cfg, "")
	if len(v.servers) != 0 {
		t.Fatal("disabled GUI retained listeners")
	}
}
