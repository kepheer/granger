package webgui

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminAPIRequiresCredentialsForEverySource(t *testing.T) {
	token := strings.Repeat("a", 64)
	for _, host := range []string{"10.66.0.2:1234", "127.0.0.1:1234", "192.0.2.10:1234", "10.19.84.52:1234"} {
		req := httptest.NewRequest("GET", "/api/config", nil)
		req.RemoteAddr = host
		req.Header.Set("X-Forwarded-For", "10.19.84.52")
		if authorizedAdmin(req, token) {
			t.Fatalf("source address bypassed credentials: %s", host)
		}
	}
	req := httptest.NewRequest("GET", "/api/config", nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("b", 64))
	if authorizedAdmin(req, token) {
		t.Fatal("incorrect token accepted")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if authorizedAdmin(req, token) == false {
		t.Fatal("correct token denied")
	}
	req.Header.Set("Authorization", token)
	if authorizedAdmin(req, token) {
		t.Fatal("malformed authorization accepted")
	}
}
