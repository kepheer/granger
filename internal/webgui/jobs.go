package webgui

import (
	"context"
	"errors"
	"granger/internal/config"
	"granger/internal/jobs"
	"granger/internal/protocols"
	"granger/pkg/runner"
	"net/http"
	"strings"
)

var protocolJobs = jobs.New()

func (a apiServer) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		id := strings.TrimPrefix(r.URL.Path, "/jobs/")
		if r.URL.Path == "/jobs" {
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: protocolJobs.List()})
			return
		}
		for _, job := range protocolJobs.List() {
			if job.ID == id {
				writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: job})
				return
			}
		}
		writeErrorText(w, http.StatusNotFound, "Unknown operation")
		return
	}
	if r.Method != http.MethodPost || strings.HasSuffix(r.URL.Path, "/cancel") == false {
		methodNotAllowed(w)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/jobs/"), "/cancel")
	if protocolJobs.Cancel(id) == false {
		writeErrorText(w, http.StatusConflict, "Operation is not running")
		return
	}
	writeJSON(w, http.StatusAccepted, apiResponse{OK: true})
}

func (a apiServer) startProtocolJob(w http.ResponseWriter, r *http.Request, action string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, exists := protocols.Find(body.Name); exists == false {
		writeErrorText(w, http.StatusBadRequest, "Unknown driver")
		return
	}
	if action == "uninstall" {
		cfg, err := loadConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if protocolInUse(cfg, body.Name) {
			writeErrorText(w, http.StatusConflict, "Driver is in use")
			return
		}
	}
	job, err := protocolJobs.Start("packages", action, body.Name, func(ctx context.Context) ([]runner.Result, error) {
		r := a.runner.WithContext(ctx)
		if action == "install" {
			status, results, err := protocols.New(r).Install(body.Name)
			if err == nil && status.Installed == false {
				err = errors.New(status.Summary)
			}
			return results, err
		}
		status, results, err := protocols.New(r).Uninstall(body.Name)
		if err == nil && status.Installed {
			err = errors.New("Driver is still installed")
		}
		return results, err
	})
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, apiResponse{OK: true, Data: job})
}

func validateModuleJobs(cfg config.Config) error {
	for _, up := range cfg.Upstreams {
		if protocolJobs.Removing(up.Type) {
			return errors.New("Driver is being removed")
		}
	}
	for _, out := range cfg.Outputs {
		if protocolJobs.Removing(out.Type) {
			return errors.New("Driver is being removed")
		}
	}
	return nil
}
