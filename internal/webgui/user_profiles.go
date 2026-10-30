package webgui

import (
	"granger/internal/config"
	"granger/internal/provision"
	"net/http"
	"strings"
)

func (a apiServer) handleUserProfiles(w http.ResponseWriter, r *http.Request, cfg config.Config, id string) {
	if _, exists := cfg.Users[id]; exists == false {
		writeErrorText(w, http.StatusNotFound, "Unknown user")
		return
	}
	if r.Method == http.MethodGet {
		profiles := map[string][]config.Client{}
		for name, out := range cfg.Outputs {
			for _, client := range out.Clients {
				if client.User == id {
					profiles[name] = append(profiles[name], client)
				}
			}
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: profiles})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var payload struct {
		Output string `json:"output"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := provision.IssueProfile(&cfg, id, payload.Output); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := saveConfig(cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	results := a.engine.ApplyConfig(cfg).Results
	results = append(results, a.engine.UpdateClients(payload.Output, id, cfg)...)
	writeJSON(w, http.StatusCreated, apiResponse{OK: allResultsOK(results), Config: &cfg, Results: results})
}

func (a apiServer) changeProfile(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/profiles/"), "/"), "/")
	if len(parts) != 2 {
		writeErrorText(w, http.StatusNotFound, "Unknown profile")
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var payload struct {
		Disabled bool `json:"disabled"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := provision.SetProfileDisabled(&cfg, parts[0], parts[1], payload.Disabled); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := saveConfig(cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	results := a.engine.ApplyConfig(cfg).Results
	userID := ""
	for _, client := range cfg.Outputs[parts[0]].Clients {
		if client.Name == parts[1] {
			userID = client.User
		}
	}
	results = append(results, a.engine.UpdateClients(parts[0], userID, cfg)...)
	writeApplyResponse(w, cfg, results)
}
