package webgui

import (
	"granger/internal/config"
	"granger/internal/engine"
	"granger/pkg/runner"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestConfigurationIsCommittedOnlyAfterSuccessfulApply(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(map[bool]string{true: "commit", false: "rollback"}[success], func(t *testing.T) {
			previousPath := configPath
			configPath = filepath.Join(t.TempDir(), "granger.yaml")
			defer func() { configPath = previousPath }()
			old := config.Default("192.0.2.1", "eth0", "")
			if err := config.SaveAtomic(configPath, old); err != nil {
				t.Fatal(err)
			}
			r := runner.NewDryRun()
			api := apiServer{runner: r, engine: engine.New(r)}
			handler := api.configurationTransaction(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next := old
				next.Server.PublicIP = "192.0.2.2"
				if err := saveConfig(next); err != nil {
					t.Fatal(err)
				}
				visible, err := loadConfig()
				if err != nil || visible.Server.PublicIP != old.Server.PublicIP {
					t.Fatal("uncommitted state became visible")
				}
				writeJSON(w, http.StatusOK, apiResponse{OK: success, Config: &next})
			}))
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/routing/graph", nil))
			applied, err := loadConfig()
			if err != nil {
				t.Fatal(err)
			}
			expected := old.Server.PublicIP
			if success {
				expected = "192.0.2.2"
			}
			if applied.Server.PublicIP != expected {
				t.Fatalf("applied %s; expected %s", applied.Server.PublicIP, expected)
			}
		})
	}
}

func TestReadingConfigurationNeverAppliesRuntimeChanges(t *testing.T) {
	oldPath := configPath
	configPath = filepath.Join(t.TempDir(), "granger.yaml")
	defer func() { configPath = oldPath }()
	if err := config.SaveAtomic(configPath, config.Default("192.0.2.1", "eth0", "")); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	(apiServer{}).handleConfig(response, httptest.NewRequest(http.MethodGet, "/config", nil))
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
}
