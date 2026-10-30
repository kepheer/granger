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
			reconcileRoutingState(ctx)
		}
	}
}

func reconcileRoutingState(ctx context.Context) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	release, err := lockRouting(ctx)
	if err != nil {
		log.Printf("routing lock unavailable: %v", err)
		return
	}
	defer release()
	cfg, err := loadConfig()
	if err != nil {
		return
	}
	e := engine.New(runner.New())
	key := e.RoutingKey(cfg)
	if routingKey != "" && routingKey != key {
		plan := e.ApplyConfig(cfg)
		if allResultsOK(plan.Results) == false {
			log.Printf("routing reconciliation failed after upstream state change")
			return
		}
		log.Printf("routing reconciled after upstream state change")
	}
	routingKey = key
}
