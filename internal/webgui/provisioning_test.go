package webgui

import (
	"granger/internal/config"
	"testing"
)

func TestManagedEntrypointDefaultsAcceptAllSupportedProtocols(t *testing.T) {
	for _, protocol := range []string{"wireguard", "amneziawg", "openvpn", "xray", "sing-box"} {
		t.Run(protocol, func(t *testing.T) {
			out, err := prepareOutputConfig("test", outputPayload{Output: config.Output{Type: protocol, ListenPort: 51821}})
			if err != nil {
				t.Fatal(err)
			}
			if out.Type != protocol {
				t.Fatal("protocol changed")
			}
		})
	}
}
