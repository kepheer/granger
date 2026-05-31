package webgui

import (
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
	mux.HandleFunc("/runtime", a.handleRuntime)
	mux.HandleFunc("/upstreams", a.handleUpstreams)
	mux.HandleFunc("/upstreams/", a.handleUpstream)
	mux.HandleFunc("/snx/", a.handleSNX)
	mux.HandleFunc("/routing/graph", a.handleRoutingGraph)
	mux.HandleFunc("/routing/dry-run", a.handleRoutingDryRun)
	mux.HandleFunc("/protocols", a.handleProtocols)
	mux.HandleFunc("/protocols/install", a.handleProtocolInstall)
	return mux
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
	cfg.Upstreams[name] = up
	if err := config.SaveAtomic(config.Path, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, apiResponse{OK: true, Config: &cfg})
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
		if err := config.SaveAtomic(config.Path, cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg})
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
		if err := config.SaveAtomic(config.Path, cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg})
	case http.MethodDelete:
		delete(cfg.Upstreams, name)
		if err := config.SaveAtomic(config.Path, cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg})
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
	if err := config.SaveAtomic(config.Path, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Config: &cfg})
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
		if err := config.SaveAtomic(config.Path, next); err != nil {
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

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
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
	if strings.TrimSpace(body) == "" {
		return "", errors.New("inline config is empty")
	}
	name := safeConfigName(requestedName)
	if name == "" {
		name = safeConfigName(upstreamName) + ".conf"
	}
	dst := filepath.Join(config.UpstreamsDir, name)
	cleanRoot, err := filepath.Abs(config.UpstreamsDir)
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
	if err := os.MkdirAll(config.UpstreamsDir, 0700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(config.UpstreamsDir, ".upload-*")
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
