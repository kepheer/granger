package webgui

import (
	"net/http"

	"granger/internal/config"
	"granger/pkg/runner"
)

func writeApplyResponse(w http.ResponseWriter, cfg config.Config, results []runner.Result) {
	writeJSON(w, http.StatusOK, apiResponse{OK: allResultsOK(results), Config: &cfg, Results: results})
}

func (a apiServer) applyReplacement(previous, next config.Config) []runner.Result {
	plan := a.engine.ReconcileConfig(previous, next)
	return plan.Results
}
