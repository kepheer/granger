package engine

import (
	"fmt"

	"granger/internal/config"
	"granger/internal/driver"
)

func (e Engine) ValidateConfig(cfg config.Config) error {
	for name, up := range cfg.Upstreams {
		if _, err := e.Registry.Upstream(up.Type); err != nil {
			return fmt.Errorf("upstream %s: %w", name, err)
		}
		if up.IsEnabled() && (up.Type == "xray" || up.Type == "sing-box") && up.Interface == "" {
			return fmt.Errorf("upstream %s requires a kernel interface", name)
		}
	}
	for _, rule := range cfg.Rules {
		up, exists := cfg.Upstreams[rule.Via]
		if exists == false {
			return fmt.Errorf("rule %s references a missing upstream", rule.Name)
		}
		module, err := e.Registry.Upstream(up.Type)
		if err != nil {
			return err
		}
		capabilities := map[driver.Capability]bool{}
		for _, capability := range module.Capabilities() {
			capabilities[capability] = true
		}
		for required, used := range map[driver.Capability]bool{driver.CapDefaultRoute: rule.Default, driver.CapDomainRoute: len(rule.Domains) > 0, driver.CapCIDRRoute: len(rule.CIDRs) > 0} {
			if used && capabilities[required] == false {
				return fmt.Errorf("upstream %s does not support %s", rule.Via, required)
			}
		}
	}
	for name, out := range cfg.Outputs {
		if _, err := e.Registry.Output(out.Type); err != nil {
			return fmt.Errorf("entrypoint %s: %w", name, err)
		}
		if out.IsEnabled() && hasSNIRules(cfg) && routedSNIOutput(out) == false && (out.Managed == false || (out.Type != "xray" && out.Type != "sing-box")) {
			return fmt.Errorf("entrypoint %s has no SNI policy adapter; use a managed entrypoint or configure its native routing separately", name)
		}
	}
	return nil
}
