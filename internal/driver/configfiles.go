package driver

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"granger/pkg/runner"
)

func activateConfig(title, src, dst string) runner.Result {
	if strings.TrimSpace(src) == "" {
		return runner.Result{Title: title, Command: "config", Output: "config path is empty", OK: false, Status: "error"}
	}
	if _, err := os.Stat(src); err != nil {
		return runner.Result{Title: title, Command: src, Output: err.Error(), OK: false, Status: "error"}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return runner.Result{Title: title, Command: dst, Output: err.Error(), OK: false, Status: "error"}
	}
	if current, err := os.Readlink(dst); err == nil && current == src {
		return runner.Result{Title: title, Command: dst, Output: "config is active", OK: true, Status: "ok"}
	}
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return runner.Result{Title: title, Command: dst, Output: err.Error(), OK: false, Status: "error"}
	}
	if err := os.Symlink(src, dst); err != nil {
		return runner.Result{Title: title, Command: dst, Output: err.Error(), OK: false, Status: "error"}
	}
	return runner.Result{Title: title, Command: dst, Output: "config activated", OK: true, Status: "ok"}
}

func normalizeQuickConfig(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(body), "\n")
	var out []string
	inInterface := false
	tableWritten := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if inInterface && !tableWritten {
				out = append(out, "Table = off")
			}
			inInterface = strings.EqualFold(trimmed, "[Interface]")
			tableWritten = false
		}
		if inInterface && strings.HasPrefix(strings.ToLower(trimmed), "dns") && strings.Contains(trimmed, "=") {
			continue
		}
		if inInterface && strings.HasPrefix(strings.ToLower(trimmed), "table") && strings.Contains(trimmed, "=") {
			if !tableWritten {
				out = append(out, "Table = off")
				tableWritten = true
			}
			continue
		}
		out = append(out, line)
	}
	if inInterface && !tableWritten {
		out = append(out, "Table = off")
	}
	next := []byte(strings.Join(out, "\n"))
	if string(next) == string(body) {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".granger-quick-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(next); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
