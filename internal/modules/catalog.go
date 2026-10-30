package modules

import "sort"

type Descriptor struct {
	ID                string `json:"id"`
	Label             string `json:"label"`
	Upstream          bool   `json:"upstream"`
	Entrypoint        bool   `json:"entrypoint"`
	Installable       bool   `json:"installable"`
	KernelInterface   bool   `json:"kernel_interface"`
	Interactive       bool   `json:"interactive"`
	ManagedProtocol   string `json:"managed_protocol,omitempty"`
	MaxInstances      int    `json:"max_instances,omitempty"`
	Interface         string `json:"interface"`
	UpstreamInterface string `json:"upstream_interface"`
	Port              int    `json:"port"`
	Subnet            string `json:"subnet"`
}

var catalog = map[string]Descriptor{
	"wireguard": {ID: "wireguard", Label: "WireGuard", Upstream: true, Entrypoint: true, Installable: true, KernelInterface: true, Interface: "wg0", UpstreamInterface: "wg-up0", Port: 51820, Subnet: "10.66.0.0/24"},
	"amneziawg": {ID: "amneziawg", Label: "AmneziaWG", Upstream: true, Entrypoint: true, Installable: true, KernelInterface: true, Interface: "awg0", UpstreamInterface: "awg-up0", Port: 51821, Subnet: "10.88.0.0/24"},
	"openvpn":   {ID: "openvpn", Label: "OpenVPN", Upstream: true, Entrypoint: true, Installable: true, KernelInterface: true, Interface: "tun0", UpstreamInterface: "tun-up0", Port: 1194, Subnet: "10.77.0.0/24"},
	"snx-rs":    {ID: "snx-rs", Label: "SNX-RS", Upstream: true, Installable: true, KernelInterface: true, Interactive: true, MaxInstances: 1, UpstreamInterface: "snx0"},
	"xray":      {ID: "xray", Label: "Xray", Upstream: true, Entrypoint: true, Installable: true, ManagedProtocol: "trojan", UpstreamInterface: "tun-xray0", Port: 8443},
	"sing-box":  {ID: "sing-box", Label: "sing-box", Upstream: true, Entrypoint: true, Installable: true, ManagedProtocol: "trojan", UpstreamInterface: "tun-sb0", Port: 8444},
	"direct":    {ID: "direct", Label: "Direct uplink", Upstream: true, UpstreamInterface: "auto"},
	"interface": {ID: "interface", Label: "Existing interface", Upstream: true, KernelInterface: true, UpstreamInterface: "eth0"},
}

func Find(id string) (Descriptor, bool) {
	d, ok := catalog[id]
	return d, ok
}

func All() []Descriptor {
	out := make([]Descriptor, 0, len(catalog))
	for _, d := range catalog {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
