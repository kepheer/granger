package webgui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"granger/internal/config"
	"granger/pkg/runner"
)

type bufferedResponse struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *bufferedResponse) Header() http.Header { return w.header }
func (w *bufferedResponse) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}
func (w *bufferedResponse) Write(body []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.body.Write(body)
}

type savedFile struct {
	path string
	body []byte
	mode os.FileMode
}

func snapshotConfigFiles(cfg config.Config) []savedFile {
	paths := map[string]bool{}
	for _, up := range cfg.Upstreams {
		paths[up.Config] = true
	}
	for _, out := range cfg.Outputs {
		paths[out.Config] = true
		for _, client := range out.Clients {
			paths[client.Config] = true
		}
	}
	var files []savedFile
	for path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.Mode().IsRegular() == false {
			continue
		}
		body, err := os.ReadFile(path)
		if err == nil {
			files = append(files, savedFile{path, body, info.Mode().Perm()})
		}
	}
	return files
}

func (a apiServer) configurationTransaction(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || strings.HasPrefix(r.URL.Path, "/snx/") || strings.HasPrefix(r.URL.Path, "/protocols/") || strings.HasPrefix(r.URL.Path, "/jobs") || r.URL.Path == "/routing/dry-run" {
			next.ServeHTTP(w, r)
			return
		}
		previous, err := loadConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		files := snapshotConfigFiles(previous)
		pending := configPath + ".pending"
		if err := os.Remove(pending); err != nil && os.IsNotExist(err) == false {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		buffer := &bufferedResponse{header: make(http.Header)}
		next.ServeHTTP(buffer, r)
		var response apiResponse
		parsed := json.Unmarshal(buffer.body.Bytes(), &response) == nil
		candidate, candidateErr := config.Load(pending)
		success := parsed && response.OK && buffer.code < 400
		if success && candidateErr == nil {
			if err := config.SaveAtomic(configPath, candidate); err != nil {
				success = false
				response.OK = false
				response.Error = "Could not save applied configuration: " + err.Error()
			}
		}
		if success == false {
			for _, file := range files {
				if err := os.WriteFile(file.path, file.body, file.mode); err != nil {
					response.Results = append(response.Results, runner.Result{Title: "Restore configuration file", OK: false, Status: "error", Output: fmt.Sprintf("%s: %v", file.path, err)})
				}
			}
			if candidateErr == nil {
				rollback := a.engine.ReconcileConfig(candidate, previous)
				if allResultsOK(rollback.Results) {
					rollback.Results = append(rollback.Results, a.engine.RestoreConfig(previous).Results...)
				}
				response.Results = append(response.Results, rollback.Results...)
				response.Config = &previous
				response.Data = nil
				if allResultsOK(rollback.Results) == false {
					response.Error = "Configuration was not saved; runtime recovery failed and traffic protection remains enabled"
				}
			}
		}
		if err := os.Remove(pending); err != nil && os.IsNotExist(err) == false {
			response.OK = false
			response.Error = "Could not remove pending configuration"
		}
		for key, values := range buffer.header {
			w.Header()[key] = values
		}
		if parsed && (candidateErr == nil || response.Error != "") {
			writeJSON(w, buffer.code, response)
			return
		}
		w.WriteHeader(buffer.code)
		_, _ = w.Write(buffer.body.Bytes())
	})
}
