package webgui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"granger/internal/config"
	"granger/internal/engine"
	"granger/internal/protocols"
	"granger/internal/provision"
	grruntime "granger/internal/runtime"
	"granger/internal/snx"
	"granger/pkg/runner"
)

const (
	DefaultListen = "10.19.84.51:1984"
	DefaultDir    = "dist/gui"
)

type Server struct {
	Listen string
	Dir    string
}

func (s Server) ListenAndServe() error {
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

	server := &http.Server{
		Addr:              listen,
		Handler:           securityHeaders(s.handler(dir)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("serving Granger GUI from %s on http://%s", dir, listen)
	return server.ListenAndServe()
}

func (s Server) handler(dir string) http.Handler {
	r := runner.New()
	api := apiServer{runner: r, engine: engine.New(r), snx: snx.New(r)}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api.routes()))
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
	runner runner.Runner
	engine engine.Engine
	snx    *snx.Manager
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
	DisplayName string `json:"display_name"`
	Output      string `json:"output"`
}

type snxPayload struct {
	Inputs map[string]string `json:"inputs"`
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
	Nodes  []graphNode   `json:"nodes"`
	Edges  []graphEdge   `json:"edges"`
	Config config.Config `json:"config"`
}

func (a apiServer) routes() http.Handler {
	mux := http.NewServeMux()
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
	mux.HandleFunc("/protocols", a.handleProtocols)
	mux.HandleFunc("/protocols/install", a.handleProtocolInstall)
	mux.HandleFunc("/protocols/uninstall", a.handleProtocolUninstall)
	return requireMutationHeader(mux)
}

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
		next.ServeHTTP(w, r)
	})
}

func (a apiServer) handleProfile(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Disposition", `attachment; filename="`+safeConfigName(client.Name)+`.conf"`)
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
	if err := config.SaveAtomic(config.Path, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	plan := a.engine.ApplyConfig(cfg)
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
	writeJSON(w, http.StatusCreated, apiResponse{OK: true, Config: &cfg, Results: results})
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
		up.Enabled = &body.Enabled
		cfg.Upstreams[name] = up
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg, Results: a.engine.SetUpstreamEnabled(name, body.Enabled, cfg)})
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
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg})
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
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg, Results: results})
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
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg})
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
		if err := provision.EnsureOutput(name, &out); err != nil {
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
		writeJSON(w, http.StatusCreated, apiResponse{OK: true, Config: &cfg, Results: results})
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
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg, Results: a.engine.SetOutputEnabled(name, body.Enabled, cfg)})
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
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg})
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
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg, Results: results})
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
		id := "usr_" + randomID(6)
		if cfg.Users == nil {
			cfg.Users = map[string]config.User{}
		}
		cfg.Users[id] = config.User{DisplayName: strings.TrimSpace(payload.DisplayName), Output: payload.Output}
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
			results = append(results, a.engine.RestartOutput(payload.Output, cfg)...)
		}
		writeJSON(w, http.StatusCreated, apiResponse{OK: true, Config: &cfg, Data: map[string]string{"id": id}, Results: results})
	default:
		methodNotAllowed(w)
	}
}

func (a apiServer) handleUser(w http.ResponseWriter, r *http.Request) {
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
			DisplayName string `json:"display_name"`
			Output      string `json:"output"`
			Disabled    bool   `json:"disabled"`
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
		user.Disabled = payload.Disabled
		cfg.Users[id] = user
		if err := provision.SetUserDisabled(&cfg, id, payload.Disabled); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := saveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg, Results: a.engine.RestartOutput(user.Output, cfg)})
	case http.MethodDelete:
		if err := provision.RevokeUser(&cfg, id); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
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
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg, Results: a.engine.RestartOutput(user.Output, cfg)})
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
	switch action {
	case "status":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: a.snx.PendingStatus()})
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
		results := []runner.Result{a.snx.Start(up, body.Inputs)}
		pending := a.snx.PendingStatus()
		if results[0].OK && !pending.Pending {
			results = append(results, a.engine.ApplyConfig(cfg).Results...)
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Results: results, Data: pending})
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
		results := []runner.Result{a.snx.Submit(body.Inputs)}
		if results[0].OK && !strings.HasPrefix(results[0].Status, "pending:") {
			results = append(results, a.engine.ApplyConfig(cfg).Results...)
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Results: results, Data: a.snx.PendingStatus()})
	case "cancel":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		a.snx.Cancel()
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: a.snx.PendingStatus()})
	case "disconnect":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		results := []runner.Result{a.snx.Disconnect()}
		results = append(results, a.engine.ApplyConfig(cfg).Results...)
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Results: results, Data: a.snx.PendingStatus()})
	default:
		writeErrorText(w, http.StatusNotFound, "unknown SNX-RS action")
	}
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
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &next})
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
	e := engine.New(runner.NewDryRun())
	plan := e.ApplyConfig(cfg)
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Results: plan.Results, Data: map[string]string{"firewall": plan.Firewall}})
}

func (a apiServer) handleProtocolInstall(w http.ResponseWriter, r *http.Request) {
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
	manager := protocols.New(a.runner)
	status, results, err := manager.Install(body.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: err.Error(), Results: results, Data: status})
		return
	}
	ok := true
	for _, result := range results {
		if !result.OK {
			ok = false
			break
		}
	}
	httpStatus := http.StatusOK
	if !ok {
		httpStatus = http.StatusAccepted
	}
	writeJSON(w, httpStatus, apiResponse{OK: ok, Results: results, Data: status})
}

func (a apiServer) handleProtocolUninstall(w http.ResponseWriter, r *http.Request) {
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
	cfg, err := loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if protocolInUse(cfg, body.Name) {
		writeErrorText(w, http.StatusConflict, "protocol is still referenced by configured upstreams or outputs")
		return
	}
	status, results, err := protocols.New(a.runner).Uninstall(body.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{OK: allResultsOK(results), Results: results, Data: status})
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
	_, results, err := protocols.New(a.runner).Install(name)
	if err != nil {
		return results, err
	}
	if !allResultsOK(results) {
		return results, errors.New("protocol installation failed: " + name)
	}
	return results, nil
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
	return routingGraph{Nodes: nodes, Edges: edges, Config: cfg}
}

func configFromGraph(graph routingGraph) (config.Config, error) {
	cfg := graph.Config
	if cfg.Users == nil {
		cfg.Users = map[string]config.User{}
	}
	if cfg.Outputs == nil {
		cfg.Outputs = map[string]config.Output{}
	}
	if cfg.Upstreams == nil {
		cfg.Upstreams = map[string]config.Upstream{}
	}
	var rules []config.Rule
	for _, node := range graph.Nodes {
		name := graphName(node)
		if name == "" {
			continue
		}
		switch node.Type {
		case "entrypoint":
			var out config.Output
			if err := decodeNodeData(node.Data, &out); err == nil && out.Type != "" {
				cfg.Outputs[name] = out
			}
		case "user":
			var user config.User
			_ = decodeNodeData(node.Data, &user)
			if user.DisplayName == "" {
				user.DisplayName = node.Label
			}
			cfg.Users[name] = user
		case "upstream", "fallback":
			var up config.Upstream
			if err := decodeNodeData(node.Data, &up); err == nil && up.Type != "" {
				cfg.Upstreams[name] = up
			}
		case "rule":
			rule := config.Rule{Name: name}
			_ = decodeNodeData(node.Data, &rule)
			if rule.Name == "" {
				rule.Name = name
			}
			for _, edge := range graph.Edges {
				if edge.Source != node.ID {
					continue
				}
				targetName := strings.TrimPrefix(edge.Target, "upstream:")
				if targetName == edge.Target {
					continue
				}
				if edge.Label == "fallback" {
					rule.DomainFallbackVia = targetName
				} else if rule.Via == "" {
					rule.Via = targetName
				}
			}
			rules = append(rules, rule)
		}
	}
	if len(rules) > 0 {
		cfg.Rules = rules
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
	if node.Label != "" {
		return safeConfigName(node.Label)
	}
	parts := strings.SplitN(node.ID, ":", 2)
	if len(parts) == 2 {
		return safeConfigName(parts[1])
	}
	return safeConfigName(node.ID)
}

func loadConfig() (config.Config, error) {
	cfg, err := config.Load(config.Path)
	if errors.Is(err, os.ErrNotExist) {
		return config.Default("", "auto", "auto"), nil
	}
	return cfg, err
}

func saveConfig(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	return config.SaveAtomic(config.Path, cfg)
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
