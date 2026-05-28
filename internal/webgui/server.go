package webgui

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
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
		Handler:           securityHeaders(spaHandler(dir)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("serving Granger GUI from %s on http://%s", dir, listen)
	return server.ListenAndServe()
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
