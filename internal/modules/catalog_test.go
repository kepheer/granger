package modules_test

import (
	"granger/internal/driver"
	"granger/internal/modules"
	"granger/internal/protocols"
	"testing"
)

func TestCatalogMatchesRegisteredDriversAndInstallers(t *testing.T) {
	registry := driver.DefaultRegistry()
	for _, descriptor := range modules.All() {
		if descriptor.Upstream {
			if _, err := registry.Upstream(descriptor.ID); err != nil {
				t.Fatal(err)
			}
		}
		if descriptor.Entrypoint {
			if _, err := registry.Output(descriptor.ID); err != nil {
				t.Fatal(err)
			}
		}
		if descriptor.Installable {
			if _, found := protocols.Find(descriptor.ID); found == false {
				t.Fatalf("missing installer for %s", descriptor.ID)
			}
		}
	}
	for _, id := range append(registry.UpstreamTypes(), registry.OutputTypes()...) {
		if _, found := modules.Find(id); found == false {
			t.Fatalf("driver %s missing from module catalog", id)
		}
	}
}
