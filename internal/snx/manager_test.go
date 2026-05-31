package snx

import (
	"strings"
	"testing"
)

func TestSanitizeOutputStripsANSIAndControlCharacters(t *testing.T) {
	got := SanitizeOutput("\x1b[31mPassword:\x1b[0m\r\nVerification\x00 Code:\r\n")
	if strings.Contains(got, "\x1b") || strings.Contains(got, "\x00") || strings.Contains(got, "\r") {
		t.Fatalf("sanitized output still contains control characters: %q", got)
	}
	if got != "Password:\nVerification Code:" {
		t.Fatalf("SanitizeOutput = %q", got)
	}
}

func TestDefaultAuthFlowIsPasswordThenSMS(t *testing.T) {
	flow := DefaultAuthFlow()
	if len(flow) != 2 {
		t.Fatalf("default flow len = %d, want 2", len(flow))
	}
	if flow[0].Type != "password" || flow[1].Type != "sms" {
		t.Fatalf("default flow = %#v", flow)
	}
}
