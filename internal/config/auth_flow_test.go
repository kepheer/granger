package config

import (
	"strings"
	"testing"
)

func TestValidateAcceptsSNXPromptFlow(t *testing.T) {
	cfg := Config{
		Outputs: map[string]Output{},
		Upstreams: map[string]Upstream{
			"corp": {
				Type:      "snx-rs",
				Interface: "snx0",
				AuthFlow: []AuthStep{
					{Type: "password", Secret: true},
					{Type: "email", Label: "Email code"},
				},
			},
		},
		Rules: []Rule{{Name: "default", Via: "corp", Default: true}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate rejected SNX prompt flow: %v", err)
	}
}

func TestValidateRejectsUnknownAuthStep(t *testing.T) {
	cfg := Config{
		Outputs: map[string]Output{},
		Upstreams: map[string]Upstream{
			"corp": {
				Type:      "snx-rs",
				Interface: "snx0",
				AuthFlow:  []AuthStep{{Type: "magic"}},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate accepted unknown auth step")
	}
	if !strings.Contains(err.Error(), "unsupported step type") {
		t.Fatalf("unexpected error: %v", err)
	}
}
