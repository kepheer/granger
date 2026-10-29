package webgui

import (
	"context"
	"granger/internal/engine"
	"granger/pkg/runner"
	"log"
	"time"
)

var routingKey string

func refreshRoutingKey() {
	cfg, err := loadConfig()
	if err == nil {
		routingKey = engine.New(runner.New()).RoutingKey(cfg)
	}
}

func watchRouting(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mutationMu.Lock()
			cfg, err := loadConfig()
			if err != nil {
				mutationMu.Unlock()
				continue
			}
			e := engine.New(runner.New())
			key := e.RoutingKey(cfg)
			if routingKey != "" && routingKey != key {
				plan := e.ApplyConfig(cfg)
				if allResultsOK(plan.Results) == false {
					log.Printf("routing reconciliation failed after upstream state change")
					mutationMu.Unlock()
					continue
				}
				log.Printf("routing reconciled after upstream state change")
			}
			routingKey = key
			mutationMu.Unlock()
		}
	}
}
