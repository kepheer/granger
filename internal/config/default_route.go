package config

func (cfg Config) PersonalDefault(userID string) string {
	user, ok := cfg.Users[userID]
	if userID != "" && (ok == false || user.Disabled) {
		return ""
	}
	target := user.DefaultUpstream
	if target == "" {
		for _, rule := range cfg.Rules {
			if rule.Default {
				target = rule.Via
				break
			}
		}
	}
	if _, exists := cfg.Upstreams[target]; exists == false || cfg.UserCanUseUpstream(userID, target) == false {
		return ""
	}
	return target
}
