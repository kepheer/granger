package config

func PrivateIPv4Networks() []string {
	return []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
}

func (c Config) UserAllowsPrivate(id string) bool {
	for name, up := range c.Upstreams {
		if up.IsEnabled() && up.AllowPrivate && c.UserCanUseUpstream(id, name) {
			return true
		}
	}
	return false
}
