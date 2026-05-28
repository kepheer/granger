package platform

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const OSReleasePath = "/etc/os-release"

type OSRelease struct {
	ID         string
	VersionID  string
	PrettyName string
	Arch       string
	Values     map[string]string
}

func CurrentOS() (OSRelease, error) {
	return ReadOSRelease(OSReleasePath)
}

func ReadOSRelease(path string) (OSRelease, error) {
	f, err := os.Open(path)
	if err != nil {
		return OSRelease{}, err
	}
	defer f.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values[key] = unquoteOSReleaseValue(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return OSRelease{}, err
	}
	return OSRelease{
		ID:         values["ID"],
		VersionID:  values["VERSION_ID"],
		PrettyName: values["PRETTY_NAME"],
		Arch:       runtime.GOARCH,
		Values:     values,
	}, nil
}

func unquoteOSReleaseValue(value string) string {
	if value == "" {
		return ""
	}
	if value[0] == '"' && len(value) >= 2 {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
	}
	if value[0] == '\'' && len(value) >= 2 && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}
