package driver

import "testing"

func TestDefaultRegistryIncludesVPNDrivers(t *testing.T) {
	reg := DefaultRegistry()

	for _, typ := range []string{"wireguard", "amneziawg", "openvpn", "sing-box", "xray"} {
		if _, err := reg.Output(typ); err != nil {
			t.Fatalf("output driver %q is not registered: %v", typ, err)
		}
		if _, err := reg.Upstream(typ); err != nil {
			t.Fatalf("upstream driver %q is not registered: %v", typ, err)
		}
	}
}

func TestWireGuardServiceName(t *testing.T) {
	if got, want := wireGuardService("", "wg0"), "wg-quick@wg0.service"; got != want {
		t.Fatalf("default WireGuard service = %q, want %q", got, want)
	}
	if got, want := wireGuardService("custom.service", "wg0"), "custom.service"; got != want {
		t.Fatalf("explicit WireGuard service = %q, want %q", got, want)
	}
}

func TestAmneziaWGServiceName(t *testing.T) {
	if got, want := amneziaWGService("", "awg0"), "awg-quick@awg0.service"; got != want {
		t.Fatalf("default AmneziaWG service = %q, want %q", got, want)
	}
	if got, want := amneziaWGService("custom.service", "awg0"), "custom.service"; got != want {
		t.Fatalf("explicit AmneziaWG service = %q, want %q", got, want)
	}
}

func TestOpenVPNServiceNames(t *testing.T) {
	cases := []struct {
		name     string
		explicit string
		kind     string
		driver   string
		config   string
		iface    string
		want     string
	}{
		{
			name:   "server profile from config",
			kind:   "server",
			driver: "home",
			config: "/etc/openvpn/server/home.conf",
			iface:  "tun0",
			want:   "openvpn-server@home.service",
		},
		{
			name:   "client profile from config",
			kind:   "client",
			driver: "exit",
			config: "/etc/openvpn/client/exit.ovpn",
			iface:  "tun1",
			want:   "openvpn-client@exit.service",
		},
		{
			name:   "client profile from interface",
			kind:   "client",
			driver: "remote",
			iface:  "tun-corp",
			want:   "openvpn-client@tun-corp.service",
		},
		{
			name:     "explicit service wins",
			explicit: "vpn-custom.service",
			kind:     "server",
			driver:   "home",
			config:   "/etc/openvpn/server/home.conf",
			iface:    "tun0",
			want:     "vpn-custom.service",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := openVPNService(tc.explicit, tc.kind, tc.driver, tc.config, tc.iface)
			if got != tc.want {
				t.Fatalf("OpenVPN service = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOpenVPNFallbackServiceName(t *testing.T) {
	if got, want := openVPNFallbackService("home", "/etc/openvpn/server/home.conf", "tun0"), "openvpn@home.service"; got != want {
		t.Fatalf("OpenVPN fallback service = %q, want %q", got, want)
	}
}

func TestXrayServiceNames(t *testing.T) {
	if got, want := xrayService("", "proxy", "/etc/granger/upstreams/proxy.json", ""), "xray@proxy.service"; got != want {
		t.Fatalf("Xray service from config = %q, want %q", got, want)
	}
	if got, want := xrayService("", "proxy", "", "tun-xray"), "xray@tun-xray.service"; got != want {
		t.Fatalf("Xray service from interface = %q, want %q", got, want)
	}
	if got, want := xrayService("custom.service", "proxy", "", ""), "custom.service"; got != want {
		t.Fatalf("explicit Xray service = %q, want %q", got, want)
	}
	if got, want := xrayFallbackService(), "xray.service"; got != want {
		t.Fatalf("Xray fallback service = %q, want %q", got, want)
	}
}

func TestSingBoxServiceNames(t *testing.T) {
	if got, want := singBoxService("", "proxy", "/etc/granger/upstreams/proxy.json", ""), "sing-box@proxy.service"; got != want {
		t.Fatalf("sing-box service from config = %q, want %q", got, want)
	}
	if got, want := singBoxService("", "proxy", "", "tun-sb"), "sing-box@tun-sb.service"; got != want {
		t.Fatalf("sing-box service from interface = %q, want %q", got, want)
	}
	if got, want := singBoxService("custom.service", "proxy", "", ""), "custom.service"; got != want {
		t.Fatalf("explicit sing-box service = %q, want %q", got, want)
	}
	if got, want := singBoxFallbackService(), "sing-box.service"; got != want {
		t.Fatalf("sing-box fallback service = %q, want %q", got, want)
	}
}
