package webgui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"granger/internal/classifier"
	"granger/internal/config"
	"granger/internal/engine"
	"granger/internal/modules"
	"granger/internal/protocols"
	"granger/internal/provision"
	grruntime "granger/internal/runtime"
	"granger/internal/snx"
	"granger/pkg/netfilter"
	"granger/pkg/runner"
)

const (
	DefaultListen = "127.0.0.1:1984"
	DefaultDir    = "dist/gui"
)

type Server struct {
	Listen string
	Dir    string
}

func (s Server) ListenAndServe() error {
	if err := ensureAdminToken(); err != nil {
		return err
	}
	listen := strings.TrimSpace(s.Listen)
	if listen == "" {
		listen = DefaultListen
	}
	dir := strings.TrimSpace(s.Dir)
	if dir == "" {
		dir = DefaultDir
	}
	if err := ensureReadableIndex(dir); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mutationMu.Lock()
	refreshRoutingKey()
	mutationMu.Unlock()
	go watchRouting(ctx)
	handler := securityHeaders(s.handler(dir))
	go watchVPNGUI(ctx, listen, handler)
	server := &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("serving Granger GUI from %s on http://%s", dir, listen)
	return server.ListenAndServe()
}

func (s Server) handler(dir string) http.Handler {
	r := runner.New()
	api := apiServer{runner: r, engine: engine.New(r), snxSessions: snx.NewRegistry(r)}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", requireAdmin(api.routes())))
	mux.Handle("/", spaHandler(dir))
	return mux
}

func ConfigFromEnv() Server {
	listen := os.Getenv("GRANGER_GUI_LISTEN")
	if strings.TrimSpace(listen) == "" {
		listen = os.Getenv("GRANGER_LISTEN")
	}
	return Server{
		Listen: listen,
		Dir:    os.Getenv("GRANGER_GUI_DIR"),
	}
}

func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if staticFileExists(dir, r.URL.Path) {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

func staticFileExists(root, requestPath string) bool {
	clean := path.Clean("/" + requestPath)
	name := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
	info, err := os.Stat(name)
	if err != nil || info.IsDir() {
		return false
	}
	return true
}

func ensureReadableIndex(dir string) error {
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("GUI build not found: run `npm run build` in ./gui first")
		}
		return err
	}
	if info.IsDir() {
		return errors.New("GUI index path is a directory")
	}
	return nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

type apiServer struct {
	runner      runner.Runner
	engine      engine.Engine
	snxSessions *snx.Registry
}

type apiResponse struct {
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Config  *config.Config  `json:"config,omitempty"`
	Results []runner.Result `json:"results,omitempty"`
	Data    any             `json:"data,omitempty"`
}

type upstreamPayload struct {
	Name         string          `json:"name"`
	Upstream     config.Upstream `json:"upstream"`
	InlineConfig string          `json:"inline_config,omitempty"`
	ConfigName   string          `json:"config_name,omitempty"`
}

type outputPayload struct {
	Name         string        `json:"name"`
	Output       config.Output `json:"output"`
	InlineConfig string        `json:"inline_config,omitempty"`
	ConfigName   string        `json:"config_name,omitempty"`
}

type userPayload struct {
	AllowedUpstreams []string `json:"allowed_upstreams"`
	AccessRestricted *bool    `json:"access_restricted,omitempty"`
	DefaultUpstream  string   `json:"default_upstream,omitempty"`
	DisplayName      string   `json:"display_name"`
	Output           string   `json:"output"`
}

type snxPayload struct {
	SessionID string            `json:"session_id,omitempty"`
	Inputs    map[string]string `json:"inputs"`
}

type graphNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Label    string         `json:"label"`
	Position map[string]int `json:"position,omitempty"`
	Data     any            `json:"data,omitempty"`
}

type graphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

type routingGraph struct {
	Version string        `json:"version"`
	Nodes   []graphNode   `json:"nodes"`
	Edges   []graphEdge   `json:"edges"`
	Config  config.Config `json:"config"`
}

func (a apiServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/modules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: modules.All()})
	})
	mux.HandleFunc("/config", a.handleConfig)
	mux.HandleFunc("/config/export", a.handleConfigExport)
	mux.HandleFunc("/config/import", a.handleConfigImport)
	mux.HandleFunc("/dashboard", a.handleDashboard)
	mux.HandleFunc("/runtime", a.handleRuntime)
	mux.HandleFunc("/upstreams", a.handleUpstreams)
	mux.HandleFunc("/upstreams/", a.handleUpstream)
	mux.HandleFunc("/outputs", a.handleOutputs)
	mux.HandleFunc("/outputs/", a.handleOutput)
	mux.HandleFunc("/profiles/", a.handleProfile)
	mux.HandleFunc("/users", a.handleUsers)
	mux.HandleFunc("/users/", a.handleUser)
	mux.HandleFunc("/snx/", a.handleSNX)
	mux.HandleFunc("/routing/graph", a.handleRoutingGraph)
	mux.HandleFunc("/routing/dry-run", a.handleRoutingDryRun)
	mux.HandleFunc("/jobs", a.handleJobs)
	mux.HandleFunc("/jobs/", a.handleJobs)
	mux.HandleFunc("/protocols", a.handleProtocols)
	mux.HandleFunc("/protocols/install", a.handleProtocolInstall)
	mux.HandleFunc("/protocols/uninstall", a.handleProtocolUninstall)
	return requireMutationHeader(a.configurationTransaction(mux))
}

var mutationMu sync.Mutex
var configPath = config.Path

func requireMutationHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-Granger-Request") != "1" {
			writeErrorText(w, http.StatusForbidden, "missing CSRF request header")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/jobs/") {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/snx/") && (strings.HasSuffix(r.URL.Path, "/start") || strings.HasSuffix(r.URL.Path, "/submit") || strings.HasSuffix(r.URL.Path, "/cancel")) {
			next.ServeHTTP(w, r)
			return
		}

		mutationMu.Lock()
		defer mutationMu.Unlock()
		next.ServeHTTP(w, r)
		refreshRoutingKey()
	})
}

func (a apiServer) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		a.changeProfile(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/profiles/"), "/"), "/")
	if len(parts) != 2 {
		writeErrorText(w, http.StatusNotFound, "expected /api/profiles/{output}/{client}")
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out, ok := cfg.Outputs[parts[0]]
	if !ok {
		writeErrorText(w, http.StatusNotFound, "unknown output")
		return
	}
	for _, client := range out.Clients {
		if client.Name != parts[1] {
			continue
		}
		if !cfg.ClientEnabled(client) {
			writeErrorText(w, http.StatusForbidden, "client profile is revoked")
			return
		}
		body, err := os.ReadFile(client.Config)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		extension := filepath.Ext(client.Config)
		if extension != ".json" && extension != ".ovpn" {
			extension = ".conf"
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+safeConfigName(client.Name)+extension+`"`)
		_, _ = w.Write(body)
		return
	}
	writeErrorText(w, http.StatusNotFound, "unknown client profile")
}

func (a apiServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: grruntime.Collect(cfg, a.runner, a.engine)})
}

func (a apiServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg})
}

func (a apiServer) handleConfigExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	body, err := os.ReadFile(config.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="granger.yaml"`)
	_, _ = w.Write(body)
}

func (a apiServer) handleConfigImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	body, err := readImportedConfig(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := config.Parse(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := saveConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	plan := a.engine.RestoreConfig(cfg)
	writeJSON(w, http.StatusOK, apiResponse{OK: allResultsOK(plan.Results), Config: &cfg, Results: plan.Results, Data: map[string]string{"firewall": plan.Firewall}})
}

func (a apiServer) handleRuntime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: a.engine.Runtime(cfg), Results: []runner.Result{a.runner.Run("Config path", time.Second, nil, "sh", "-c", "test -f "+shellQuote(config.Path))}})
}

func (a apiServer) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	cfg, err := loadConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if cfg.Upstreams == nil {
		cfg.Upstreams = map[string]config.Upstream{}
	}
	var payload upstreamPayload
	if err := readJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name := safeConfigName(payload.Name)
	if name == "" {
		writeErrorText(w, http.StatusBadRequest, "upstream name is required")
		return
	}
	if _, ok := cfg.Upstreams[name]; ok {
		writeErrorText(w, http.StatusConflict, "upstream already exists")
		return
	}
	up, err := a.prepareUpstreamConfig(name, payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	results, err := a.ensureProtocol(up.Type)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: err.Error(), Results: results})
		return
	}
	cfg.Upstreams[name] = up
	if err := saveConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if up.IsEnabled() {
		results = append(results, a.engine.SetUpstreamEnabled(name, true, cfg)...)
	}
	writeJSON(w, http.StatusCreated, apiResponse{OK: allResultsOK(results), Config: &cfg, Results: results})
}

func (a apiServer) handleUpstream(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/upstreams/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		methodNotAllowed(w)
		return
	}
	name := parts[0]
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	up, ok := cfg.Upstreams[name]
	if !ok {
		writeErrorText(w, http.StatusNotFound, "unknown upstream")
		return
	}
	if len(parts) == 2 && parts[1] == "enable" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if body.Enabled && up.Type == "snx-rs" {
			if err := snx.CheckConfig(up.Config); err != nil {
				writeError(w, http.StatusConflict, err)
				return
			}
			if _, err := a.ensureProtocol(up.Type); err != nil {
				writeError(w, http.StatusConflict, err)
				return
			}
		}
		a.cancelSNX(name, up)
		up.Enabled = &body.Enabled
		cfg.Upstreams[name] = up
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeApplyResponse(w, cfg, a.engine.SetUpstreamEnabled(name, body.Enabled, cfg))
		return
	}
	if len(parts) == 2 && parts[1] == "config" {
		a.handleUpstreamConfig(w, r, cfg, name, up)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: up})
	case http.MethodPut:
		var payload upstreamPayload
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		a.cancelSNX(name, up)
		next, err := a.prepareUpstreamConfig(name, payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		cfg.Upstreams[name] = next
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeApplyResponse(w, cfg, a.applyPending(cfg))
	case http.MethodDelete:
		if err := canDeleteUpstream(cfg, name); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		disabled := false
		up.Enabled = &disabled
		cfg.Upstreams[name] = up
		results := a.engine.SetUpstreamEnabled(name, false, cfg)
		delete(cfg.Upstreams, name)
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		results = append(results, a.uninstallProtocolIfUnused(r, cfg, up.Type)...)
		writeJSON(w, http.StatusOK, apiResponse{OK: allResultsOK(results), Config: &cfg, Results: results})
	default:
		methodNotAllowed(w)
	}
}

func (a apiServer) handleUpstreamConfig(w http.ResponseWriter, r *http.Request, cfg config.Config, name string, up config.Upstream) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var body struct {
		InlineConfig string `json:"inline_config"`
		ConfigName   string `json:"config_name"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		defer file.Close()
		b, err := ioReadAllLimit(file, 4<<20)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		body.InlineConfig = string(b)
		body.ConfigName = header.Filename
	} else if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	a.cancelSNX(name, up)
	path, err := writeUpstreamInlineConfig(name, body.ConfigName, body.InlineConfig)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	up.Config = path
	cfg.Upstreams[name] = up
	if err := saveConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeApplyResponse(w, cfg, a.applyPending(cfg))
}

func (a apiServer) handleOutputs(w http.ResponseWriter, r *http.Request) {
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: cfg.Outputs})
	case http.MethodPost:
		var payload outputPayload
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		name := safeConfigName(payload.Name)
		if name == "" {
			writeErrorText(w, http.StatusBadRequest, "output name is required")
			return
		}
		if _, ok := cfg.Outputs[name]; ok {
			writeErrorText(w, http.StatusConflict, "output already exists")
			return
		}
		results, err := a.ensureProtocol(payload.Output.Type)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: err.Error(), Results: results})
			return
		}
		out, err := prepareOutputConfig(name, payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if cfg.Outputs == nil {
			cfg.Outputs = map[string]config.Output{}
		}
		if err := provision.EnsureOutput(name, &out, cfg.Server.PublicIP); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		cfg.Outputs[name] = out
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if out.IsEnabled() {
			results = append(results, a.engine.SetOutputEnabled(name, true, cfg)...)
		}
		writeJSON(w, http.StatusCreated, apiResponse{OK: allResultsOK(results), Config: &cfg, Results: results})
	default:
		methodNotAllowed(w)
	}
}

func (a apiServer) handleOutput(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/outputs/"), "/")
	parts := strings.Split(rest, "/")
	name := safeConfigName(parts[0])
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out, ok := cfg.Outputs[name]
	if !ok {
		writeErrorText(w, http.StatusNotFound, "unknown output")
		return
	}
	if len(parts) == 2 && parts[1] == "enable" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		out.Enabled = &body.Enabled
		cfg.Outputs[name] = out
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeApplyResponse(w, cfg, a.engine.SetOutputEnabled(name, body.Enabled, cfg))
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: out})
	case http.MethodPut:
		var payload outputPayload
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		next, err := prepareOutputConfig(name, payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		cfg.Outputs[name] = next
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeApplyResponse(w, cfg, a.applyPending(cfg))
	case http.MethodDelete:
		if err := canDeleteOutput(cfg, name); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		disabled := false
		out.Enabled = &disabled
		cfg.Outputs[name] = out
		results := a.engine.SetOutputEnabled(name, false, cfg)
		delete(cfg.Outputs, name)
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		results = append(results, a.uninstallProtocolIfUnused(r, cfg, out.Type)...)
		writeJSON(w, http.StatusOK, apiResponse{OK: allResultsOK(results), Config: &cfg, Results: results})
	default:
		methodNotAllowed(w)
	}
}

func (a apiServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: cfg.Users})
	case http.MethodPost:
		var payload userPayload
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(payload.DisplayName) == "" {
			writeErrorText(w, http.StatusBadRequest, "user display name is required")
			return
		}
		if payload.Output != "" {
			if _, ok := cfg.Outputs[payload.Output]; !ok {
				writeErrorText(w, http.StatusBadRequest, "unknown user output")
				return
			}
		}
		id := ""
		for attempt := 0; attempt < 64; attempt++ {
			candidate := "usr_" + randomID(6)
			collision := false
			mark := netfilter.UserMark(payload.Output, candidate)
			for outputName, output := range cfg.Outputs {
				for _, client := range output.Clients {
					if netfilter.UserMark(outputName, client.Name) == mark {
						collision = true
					}
				}
			}
			if collision == false {
				id = candidate
				break
			}
		}
		if id == "" {
			writeErrorText(w, http.StatusConflict, "no unique routing identity available")
			return
		}
		if cfg.Users == nil {
			cfg.Users = map[string]config.User{}
		}
		restricted := true
		if payload.AccessRestricted != nil {
			restricted = *payload.AccessRestricted
		}
		allowed := payload.AllowedUpstreams
		if allowed == nil {
			if _, ok := cfg.Upstreams["direct"]; ok {
				allowed = []string{"direct"}
			}
		}
		cfg.Users[id] = config.User{DisplayName: strings.TrimSpace(payload.DisplayName), Output: payload.Output, AccessRestricted: restricted, AllowedUpstreams: allowed, DefaultUpstream: payload.DefaultUpstream}
		if err := cfg.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if payload.Output != "" {
			if err := provision.IssueUser(&cfg, id); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		results := []runner.Result{}
		if payload.Output != "" {
			results = append(results, a.engine.ApplyConfig(cfg).Results...)
			results = append(results, a.engine.UpdateClients(payload.Output, id, cfg)...)
		}
		writeJSON(w, http.StatusCreated, apiResponse{OK: allResultsOK(results), Config: &cfg, Data: map[string]string{"id": id}, Results: results})
	default:
		methodNotAllowed(w)
	}
}

func (a apiServer) handleUser(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/users/"), "/"), "/")
	if len(parts) == 2 && parts[1] == "profiles" {
		cfg, err := loadConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		a.handleUserProfiles(w, r, cfg, parts[0])
		return
	}
	id := safeConfigName(strings.Trim(strings.TrimPrefix(r.URL.Path, "/users/"), "/"))
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	user, ok := cfg.Users[id]
	if !ok {
		writeErrorText(w, http.StatusNotFound, "unknown user")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: user})
	case http.MethodPut:
		var payload struct {
			DisplayName      string    `json:"display_name"`
			Output           string    `json:"output"`
			Disabled         *bool     `json:"disabled"`
			AllowedUpstreams *[]string `json:"allowed_upstreams,omitempty"`
			AccessRestricted *bool     `json:"access_restricted,omitempty"`
			DefaultUpstream  *string   `json:"default_upstream,omitempty"`
		}
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(payload.DisplayName) != "" {
			user.DisplayName = strings.TrimSpace(payload.DisplayName)
		}
		if payload.Output != "" && payload.Output != user.Output {
			writeErrorText(w, http.StatusBadRequest, "moving an issued profile between outputs is not supported; revoke it and issue a new profile")
			return
		}
		if payload.Disabled != nil {
			user.Disabled = *payload.Disabled
		}
		if payload.AllowedUpstreams != nil {
			user.AllowedUpstreams = *payload.AllowedUpstreams
			user.AccessRestricted = true
		}
		if payload.AccessRestricted != nil {
			user.AccessRestricted = *payload.AccessRestricted
		}
		if payload.DefaultUpstream != nil {
			user.DefaultUpstream = *payload.DefaultUpstream
		}
		cfg.Users[id] = user
		if err := cfg.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := updateUserCredentials(&cfg, id, payload.Disabled); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		results := a.engine.ApplyConfig(cfg).Results
		for name, out := range cfg.Outputs {
			for _, client := range out.Clients {
				if client.User == id {
					results = append(results, a.engine.UpdateClients(name, id, cfg)...)
					break
				}
			}
		}
		writeApplyResponse(w, cfg, results)
	case http.MethodDelete:
		if err := provision.RevokeUser(&cfg, id); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		affected := []string{}
		for name, out := range cfg.Outputs {
			for _, client := range out.Clients {
				if client.User == id {
					affected = append(affected, name)
					break
				}
			}
		}
		delete(cfg.Users, id)
		for outputName, out := range cfg.Outputs {
			clients := out.Clients[:0]
			for _, client := range out.Clients {
				if client.User != id {
					clients = append(clients, client)
				}
			}
			out.Clients = clients
			cfg.Outputs[outputName] = out
		}
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		results := a.engine.ApplyConfig(cfg).Results
		for _, name := range affected {
			results = append(results, a.engine.UpdateClients(name, id, cfg)...)
		}
		writeApplyResponse(w, cfg, results)
	default:
		methodNotAllowed(w)
	}
}

func (a apiServer) handleSNX(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/snx/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		writeErrorText(w, http.StatusNotFound, "expected /api/snx/{upstream}/{action}")
		return
	}
	name, action := parts[0], parts[1]
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	up, ok := cfg.Upstreams[name]
	if !ok || up.Type != "snx-rs" {
		writeErrorText(w, http.StatusNotFound, "unknown SNX-RS upstream")
		return
	}
	manager := a.snxSessions.Connection(name)
	if up.IsEnabled() == false && action != "status" && action != "cancel" {
		writeErrorText(w, http.StatusConflict, "Connection is disabled")
		return
	}

	switch action {
	case "status":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		configErr := snx.CheckConfig(up.Config)
		configMessage := ""
		if configErr != nil {
			configMessage = configErr.Error()
		}
		_, driverErr := exec.LookPath("snxctl")
		data := struct {
			snx.PendingStatus
			ConfigReady     bool            `json:"config_ready"`
			ConfigError     string          `json:"config_error,omitempty"`
			DriverInstalled bool            `json:"driver_installed"`
			InitialStep     config.AuthStep `json:"initial_step"`
		}{manager.PendingStatus(), configErr == nil, configMessage, driverErr == nil, snx.InitialStep(up)}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: data})
	case "start":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var body snxPayload
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if manager.StartAsync(up, body.Inputs, a.reconcileSNX) == false {
			writeErrorText(w, http.StatusConflict, "Sign-in is already in progress")
			return
		}
		writeJSON(w, http.StatusAccepted, apiResponse{OK: true, Data: manager.PendingStatus()})
	case "submit":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var body snxPayload
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if manager.SubmitSessionAsync(body.SessionID, body.Inputs, a.reconcileSNX) == false {
			writeErrorText(w, http.StatusConflict, "Sign-in is already in progress")
			return
		}
		writeJSON(w, http.StatusAccepted, apiResponse{OK: true, Data: manager.PendingStatus()})
	case "cancel":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var body snxPayload
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if manager.CancelSession(body.SessionID) == false {
			writeErrorText(w, http.StatusConflict, "Sign-in session changed; refresh the connection")
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: manager.PendingStatus()})
	case "disconnect":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		results := []runner.Result{manager.Disconnect(up.Config)}
		results = append(results, a.engine.ApplyConfig(cfg).Results...)
		writeJSON(w, http.StatusOK, apiResponse{OK: allResultsOK(results), Results: results, Data: manager.PendingStatus()})
	default:
		writeErrorText(w, http.StatusNotFound, "unknown SNX-RS action")
	}
}

func (a apiServer) reconcileSNX() []runner.Result {
	mutationMu.Lock()
	defer mutationMu.Unlock()

	cfg, err := loadConfig()
	if err != nil {
		return []runner.Result{{Title: "Restore SNX routes", Output: err.Error(), OK: false, Status: "error"}}
	}

	results := a.engine.ApplyConfig(cfg).Results
	refreshRoutingKey()
	return results
}

func (a apiServer) handleRoutingGraph(w http.ResponseWriter, r *http.Request) {
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: buildRoutingGraph(cfg)})
	case http.MethodPost:
		var graph routingGraph
		if err := readJSON(r, &graph); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if graph.Version != configVersion(cfg) {
			writeErrorText(w, http.StatusConflict, "Configuration changed. Reload the routing graph before saving to preserve current users and permissions.")
			return
		}
		next, err := configFromGraph(graph)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := next.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := saveConfig(next); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		results := a.applyReplacement(cfg, next)
		writeJSON(w, http.StatusOK, apiResponse{OK: allResultsOK(results), Config: &next, Results: results, Data: buildRoutingGraph(next)})
	default:
		methodNotAllowed(w)
	}
}

func (a apiServer) handleRoutingDryRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if r.ContentLength != 0 {
		var graph routingGraph
		if err := readJSON(r, &graph); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		next, err := configFromGraph(graph)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := next.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		cfg = next
	}
	plan := a.engine.PreviewConfig(cfg)
	writeJSON(w, http.StatusOK, apiResponse{OK: allResultsOK(plan.Results), Results: plan.Results, Data: map[string]string{"firewall": plan.Firewall, "preview": "current connection state"}})
}

func (a apiServer) handleProtocolInstall(w http.ResponseWriter, r *http.Request) {
	a.startProtocolJob(w, r, "install")
}

func (a apiServer) handleProtocolUninstall(w http.ResponseWriter, r *http.Request) {
	a.startProtocolJob(w, r, "uninstall")
}

func (a apiServer) handleProtocols(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	manager := protocols.New(a.runner)
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: manager.StatusAll()})
}

func (a apiServer) prepareUpstreamConfig(name string, payload upstreamPayload) (config.Upstream, error) {
	up := payload.Upstream
	if up.Config == "" && strings.TrimSpace(payload.InlineConfig) != "" {
		path, err := writeUpstreamInlineConfig(name, payload.ConfigName, payload.InlineConfig)
		if err != nil {
			return up, err
		}
		up.Config = path
	}
	if up.Type == "" {
		return up, errors.New("upstream type is required")
	}
	return up, nil
}

func (a apiServer) ensureProtocol(name string) ([]runner.Result, error) {
	switch name {
	case "", "direct", "interface":
		return nil, nil
	}
	installer, ok := protocols.Find(name)
	if !ok {
		return nil, errors.New("unknown protocol installer: " + name)
	}
	if installer.Check(a.runner).Installed {
		return nil, nil
	}
	return nil, errors.New("protocol " + name + " is not installed; install it explicitly using the protocol button on the Upstreams page first")
}

func (a apiServer) uninstallProtocolIfUnused(r *http.Request, cfg config.Config, name string) []runner.Result {
	if r.URL.Query().Get("uninstall_protocol") != "true" || protocolInUse(cfg, name) {
		return nil
	}
	_, results, _ := protocols.New(a.runner).Uninstall(name)
	return results
}

func prepareOutputConfig(name string, payload outputPayload) (config.Output, error) {
	out := payload.Output
	if out.Config == "" && strings.TrimSpace(payload.InlineConfig) != "" {
		path, err := writeInlineConfig(config.OutputsDir, name, payload.ConfigName, payload.InlineConfig)
		if err != nil {
			return out, err
		}
		out.Config = path
	}
	if out.Type == "" {
		return out, errors.New("output type is required")
	}

	return out, nil
}

func protocolInUse(cfg config.Config, name string) bool {
	if classifier.NeedsTransparent(cfg) {
		backend, err := classifier.DefaultRegistry(runner.New()).Find(cfg.Server.DomainRouterSettings().Backend)
		if err == nil && backend.Dependency() == name {
			return true
		}
	}

	for _, out := range cfg.Outputs {
		if out.Type == name {
			return true
		}
	}
	for _, up := range cfg.Upstreams {
		if up.Type == name {
			return true
		}
	}
	return false
}

func canDeleteUpstream(cfg config.Config, name string) error {
	for id, user := range cfg.Users {
		if user.DefaultUpstream == name {
			return errors.New("upstream is a default route for user " + id)
		}
		for _, allowed := range user.AllowedUpstreams {
			if allowed == name {
				return errors.New("upstream is referenced by user access policy " + id)
			}
		}
	}

	for _, rule := range cfg.Rules {
		if rule.Via == name || rule.DomainFallbackVia == name {
			return errors.New("upstream is still referenced by routing rule " + rule.Name)
		}
	}
	for upstreamName, up := range cfg.Upstreams {
		if up.FallbackWhenDown == name {
			return errors.New("upstream is still referenced as fallback by " + upstreamName)
		}
	}
	return nil
}

func canDeleteOutput(cfg config.Config, name string) error {
	if len(cfg.Outputs[name].Clients) > 0 {
		return errors.New("entrypoint still has issued profiles")
	}

	for userID, user := range cfg.Users {
		if user.Output == name {
			return errors.New("output still has issued profile " + userID)
		}
	}
	return nil
}

func allResultsOK(results []runner.Result) bool {
	for _, result := range results {
		if !result.OK {
			return false
		}
	}
	return true
}

func buildRoutingGraph(cfg config.Config) routingGraph {
	var nodes []graphNode
	var edges []graphEdge
	x := 0
	for name, out := range cfg.Outputs {
		nodes = append(nodes, graphNode{ID: "output:" + name, Type: "entrypoint", Label: name, Position: map[string]int{"x": x, "y": 40}, Data: out})
		x += 220
	}
	x = 0
	for name, user := range cfg.Users {
		nodes = append(nodes, graphNode{ID: "user:" + name, Type: "user", Label: user.DisplayName, Position: map[string]int{"x": x, "y": 180}, Data: user})
		x += 220
	}
	for i, rule := range cfg.Rules {
		id := "rule:" + rule.Name
		nodes = append(nodes, graphNode{ID: id, Type: "rule", Label: rule.Name, Position: map[string]int{"x": 120 + i*220, "y": 320}, Data: rule})
		if rule.Via != "" {
			edges = append(edges, graphEdge{ID: id + "->upstream:" + rule.Via, Source: id, Target: "upstream:" + rule.Via, Label: "via"})
		}
		if rule.DomainFallbackVia != "" {
			edges = append(edges, graphEdge{ID: id + "->fallback:" + rule.DomainFallbackVia, Source: id, Target: "upstream:" + rule.DomainFallbackVia, Label: "fallback"})
		}
	}
	x = 0
	for name, up := range cfg.Upstreams {
		nodes = append(nodes, graphNode{ID: "upstream:" + name, Type: "upstream", Label: name, Position: map[string]int{"x": x, "y": 520}, Data: up})
		if len(up.DNS) > 0 {
			dnsID := "dns:" + name
			nodes = append(nodes, graphNode{ID: dnsID, Type: "dns", Label: name + " DNS", Position: map[string]int{"x": x, "y": 700}, Data: up.DNS})
			edges = append(edges, graphEdge{ID: dnsID + "->upstream:" + name, Source: dnsID, Target: "upstream:" + name, Label: "resolver"})
		}
		x += 240
	}
	return routingGraph{Version: configVersion(cfg), Nodes: nodes, Edges: edges, Config: cfg}
}

func configFromGraph(graph routingGraph) (config.Config, error) {
	cfg := config.Config{
		Server:    graph.Config.Server,
		Users:     map[string]config.User{},
		Outputs:   map[string]config.Output{},
		Upstreams: map[string]config.Upstream{},
		Rules:     []config.Rule{},
	}
	seen := map[string]bool{}
	for _, node := range graph.Nodes {
		if seen[node.ID] {
			return cfg, errors.New("duplicate graph node: " + node.ID)
		}
		seen[node.ID] = true
		name := graphName(node)
		if name == "" {
			return cfg, errors.New("graph node requires a name")
		}
		switch node.Type {
		case "entrypoint":
			var out config.Output
			if err := decodeNodeData(node.Data, &out); err != nil {
				return cfg, err
			}
			cfg.Outputs[name] = out
		case "user":
			var user config.User
			if err := decodeNodeData(node.Data, &user); err != nil {
				return cfg, err
			}
			user.DisplayName = node.Label
			cfg.Users[name] = user
		case "upstream", "fallback":
			var up config.Upstream
			if err := decodeNodeData(node.Data, &up); err != nil {
				return cfg, err
			}
			cfg.Upstreams[name] = up
		case "rule":
			var rule config.Rule
			if err := decodeNodeData(node.Data, &rule); err != nil {
				return cfg, err
			}
			rule.Name = name
			rule.Via = ""
			rule.DomainFallbackVia = ""
			for _, edge := range graph.Edges {
				if edge.Source != node.ID {
					continue
				}
				target, ok := strings.CutPrefix(edge.Target, "upstream:")
				if ok == false {
					return cfg, errors.New("routing edge must target an upstream")
				}
				if edge.Label == "fallback" {
					if rule.DomainFallbackVia != "" {
						return cfg, errors.New("rule has multiple fallback connections: " + name)
					}
					rule.DomainFallbackVia = target
					continue
				}
				if rule.Via != "" {
					return cfg, errors.New("rule has multiple primary connections: " + name)
				}
				rule.Via = target
			}
			cfg.Rules = append(cfg.Rules, rule)
		}
	}
	return cfg, nil
}

func decodeNodeData(data any, dst any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func graphName(node graphNode) string {
	parts := strings.SplitN(node.ID, ":", 2)
	if len(parts) == 2 {
		return safeConfigName(parts[1])
	}
	return safeConfigName(node.ID)
}

func loadConfig() (config.Config, error) {
	cfg, err := config.Load(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return config.Default("", "auto", "auto"), nil
	}
	return cfg, err
}

func saveConfig(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := engine.New(runner.New()).ValidateConfig(cfg); err != nil {
		return err
	}
	if err := validateModuleJobs(cfg); err != nil {
		return err
	}
	return config.SaveAtomic(configPath+".pending", cfg)
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func readImportedConfig(r *http.Request) ([]byte, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			return nil, err
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return ioReadAllLimit(file, 4<<20)
	}
	var body struct {
		Config string `json:"config"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Config) == "" {
		return nil, errors.New("config is empty")
	}
	return []byte(body.Config), nil
}

func writeJSON(w http.ResponseWriter, status int, body apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeErrorText(w, status, err.Error())
}

func writeErrorText(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{OK: false, Error: msg})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeErrorText(w, http.StatusMethodNotAllowed, "method not allowed")
}

func safeConfigName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func writeUpstreamInlineConfig(upstreamName, requestedName, body string) (string, error) {
	return writeInlineConfig(config.UpstreamsDir, upstreamName, requestedName, body)
}

func writeInlineConfig(root, profileName, requestedName, body string) (string, error) {
	if strings.TrimSpace(body) == "" {
		return "", errors.New("inline config is empty")
	}
	name := safeConfigName(requestedName)
	if name == "" {
		name = safeConfigName(profileName) + ".conf"
	}
	dst := filepath.Join(root, name)
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	cleanDst, err := filepath.Abs(dst)
	if err != nil {
		return "", err
	}
	if cleanDst != cleanRoot && !strings.HasPrefix(cleanDst, cleanRoot+string(os.PathSeparator)) {
		return "", errors.New("config path escapes upstream directory")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(root, ".upload-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, cleanDst); err != nil {
		return "", err
	}
	return cleanDst, nil
}

func randomID(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}

func ioReadAllLimit(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("uploaded config is too large")
	}
	return b, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func updateUserCredentials(cfg *config.Config, id string, disabled *bool) error {
	if disabled == nil {
		return nil
	}
	return provision.SetUserDisabled(cfg, id, *disabled)
}

func configVersion(cfg config.Config) string {
	body, _ := json.Marshal(cfg)
	return fmt.Sprintf("%x", sha256.Sum256(body))
}

func (a apiServer) cancelSNX(name string, up config.Upstream) {
	if up.Type == "snx-rs" && a.snxSessions != nil {
		a.snxSessions.Connection(name).Cancel()
	}
}
