package protocols

import (
	"errors"
	"sort"
	"strings"
	"time"

	"granger/pkg/runner"
)

const installTimeout = 10 * time.Minute

const singBoxAPTSource = `Types: deb
URIs: https://deb.sagernet.org/
Suites: *
Components: *
Enabled: yes
Signed-By: /etc/apt/keyrings/sagernet.asc
`

const amneziaDebianAPTSource = `deb https://ppa.launchpadcontent.net/amnezia/ppa/ubuntu focal main
deb-src https://ppa.launchpadcontent.net/amnezia/ppa/ubuntu focal main
`

type Status struct {
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	Installed   bool            `json:"installed"`
	Installable bool            `json:"installable"`
	State       string          `json:"state"`
	Summary     string          `json:"summary"`
	Commands    []runner.Result `json:"commands,omitempty"`
}

type Installer interface {
	Name() string
	DisplayName() string
	Check(runner.Runner) Status
	Install(runner.Runner) []runner.Result
	Uninstall(runner.Runner) []runner.Result
}

type Manager struct {
	Runner runner.Runner
}

func New(r runner.Runner) Manager {
	return Manager{Runner: r}
}

func (m Manager) StatusAll() []Status {
	installers := Supported()
	out := make([]Status, 0, len(installers))
	for _, installer := range installers {
		out = append(out, installer.Check(m.Runner))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (m Manager) Install(name string) (Status, []runner.Result, error) {
	installer, ok := Find(name)
	if !ok {
		res := runner.Result{Title: "Install protocol", Command: "protocol:" + name, Output: "unknown protocol installer", OK: false, Status: "unknown"}
		return Status{Name: name, DisplayName: name, Installed: false, Installable: false, State: "unknown", Summary: "Unknown protocol installer.", Commands: []runner.Result{res}}, []runner.Result{res}, errors.New("unknown protocol installer: " + name)
	}
	before := installer.Check(m.Runner)
	if !before.Installable {
		res := runner.Result{Title: "Install " + installer.DisplayName(), Command: "manual", Output: before.Summary, OK: false, Status: "manual-required"}
		before.Commands = []runner.Result{res}
		return before, []runner.Result{res}, nil
	}
	results := installer.Install(m.Runner)
	after := installer.Check(m.Runner)
	after.Commands = results
	return after, results, nil
}

func (m Manager) Uninstall(name string) (Status, []runner.Result, error) {
	installer, ok := Find(name)
	if !ok {
		return Status{}, nil, errors.New("unknown protocol installer: " + name)
	}
	results := installer.Uninstall(m.Runner)
	after := installer.Check(m.Runner)
	after.Commands = results
	return after, results, nil
}

func Supported() []Installer {
	return []Installer{
		aptInstaller{name: "openvpn", display: "OpenVPN", binaries: []string{"openvpn"}, packages: []string{"openvpn"}},
		aptInstaller{name: "wireguard", display: "WireGuard", binaries: []string{"wg", "wg-quick"}, packages: []string{"wireguard-tools"}},
		amneziaWGInstaller{},
		singBoxInstaller{},
		snxRSInstaller{},
		xrayInstaller{},
	}
}

func Find(name string) (Installer, bool) {
	name = normalize(name)
	for _, installer := range Supported() {
		if installer.Name() == name {
			return installer, true
		}
	}
	return nil, false
}

type aptInstaller struct {
	name     string
	display  string
	binaries []string
	packages []string
}

func (i aptInstaller) Name() string        { return i.name }
func (i aptInstaller) DisplayName() string { return i.display }

func (i aptInstaller) Check(r runner.Runner) Status {
	results := checkBinaries(r, i.display, i.binaries)
	installed := allOK(results)
	summary := i.display + " is ready."
	state := "installed"
	if !installed {
		state = "available"
		summary = i.display + " is not installed. Granger can install package(s): " + strings.Join(i.packages, ", ") + "."
	}
	return Status{Name: i.name, DisplayName: i.display, Installed: installed, Installable: true, State: state, Summary: summary, Commands: results}
}

func (i aptInstaller) Install(r runner.Runner) []runner.Result {
	args := append([]string{"install", "-y", "--no-install-recommends"}, i.packages...)
	results := []runner.Result{
		r.Run("Update apt metadata", installTimeout, nil, "env", "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "update"),
	}
	if !results[0].OK {
		return results
	}
	results = append(results, r.Run("Install "+i.display, installTimeout, nil, "env", append([]string{"DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get"}, args...)...))
	return results
}

func (i aptInstaller) Uninstall(r runner.Runner) []runner.Result {
	return []runner.Result{aptRemove(r, "Remove "+i.display, i.packages...)}
}

type amneziaWGInstaller struct{}

func (amneziaWGInstaller) Name() string        { return "amneziawg" }
func (amneziaWGInstaller) DisplayName() string { return "AmneziaWG" }

func (i amneziaWGInstaller) Check(r runner.Runner) Status {
	results := checkBinaries(r, i.DisplayName(), []string{"awg", "awg-quick"})
	installed := allOK(results)
	if installed {
		return Status{Name: i.Name(), DisplayName: i.DisplayName(), Installed: true, Installable: true, State: "installed", Summary: "AmneziaWG is ready.", Commands: results}
	}
	return Status{Name: i.Name(), DisplayName: i.DisplayName(), Installed: false, Installable: true, State: "available", Summary: "Installs AmneziaWG using the upstream Amnezia Linux kernel module instructions for Debian/Ubuntu.", Commands: results}
}

func (i amneziaWGInstaller) Install(r runner.Runner) []runner.Result {
	results := []runner.Result{
		r.Run("Check AmneziaWG supported OS", 5*time.Second, nil, "sh", "-c", `. /etc/os-release; case "$ID:$VERSION_ID" in debian:12|debian:13|ubuntu:22.04|ubuntu:24.04) exit 0;; *) echo "unsupported OS: $ID $VERSION_ID"; exit 1;; esac`),
	}
	if failed(results) {
		return results
	}
	results = append(results,
		r.Run("Install AmneziaWG prerequisites", installTimeout, nil, "sh", "-c", `. /etc/os-release; headers=linux-headers-amd64; [ "$ID" = ubuntu ] && headers=linux-headers-generic; DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get install -y --no-install-recommends software-properties-common python3-launchpadlib gnupg2 dkms build-essential "$headers"`),
	)
	if failed(results) {
		return results
	}
	results = append(results, r.Run("Configure AmneziaWG package source", installTimeout, strings.NewReader(amneziaDebianAPTSource), "sh", "-c", `. /etc/os-release; if [ "$ID" = ubuntu ]; then add-apt-repository -y ppa:amnezia/ppa; else apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 57290828 && mkdir -p /etc/apt/sources.list.d && tee /etc/apt/sources.list.d/amneziawg.list >/dev/null; fi`))
	if failed(results) {
		return results
	}
	results = append(results,
		r.Run("Update apt metadata", installTimeout, nil, "env", "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "update"),
		aptInstall(r, "Install AmneziaWG", "amneziawg"),
	)
	return results
}

func (i amneziaWGInstaller) Uninstall(r runner.Runner) []runner.Result {
	return []runner.Result{aptRemove(r, "Remove AmneziaWG", "amneziawg")}
}

type singBoxInstaller struct{}

func (singBoxInstaller) Name() string        { return "sing-box" }
func (singBoxInstaller) DisplayName() string { return "sing-box" }

func (i singBoxInstaller) Check(r runner.Runner) Status {
	results := checkBinaries(r, i.DisplayName(), []string{"sing-box"})
	installed := allOK(results)
	if installed {
		return Status{Name: i.Name(), DisplayName: i.DisplayName(), Installed: true, Installable: true, State: "installed", Summary: "sing-box is ready.", Commands: results}
	}
	return Status{Name: i.Name(), DisplayName: i.DisplayName(), Installed: false, Installable: true, State: "available", Summary: "Installs sing-box from the official SagerNet APT repository.", Commands: results}
}

func (i singBoxInstaller) Install(r runner.Runner) []runner.Result {
	results := []runner.Result{
		r.Run("Install sing-box repository prerequisites", installTimeout, nil, "env", "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "install", "-y", "--no-install-recommends", "ca-certificates", "curl", "gnupg"),
	}
	if failed(results) {
		return results
	}
	results = append(results,
		r.Run("Create apt keyring directory", 10*time.Second, nil, "mkdir", "-p", "/etc/apt/keyrings"),
		r.Run("Download sing-box signing key", 30*time.Second, nil, "curl", "-fsSL", "https://sing-box.app/gpg.key", "-o", "/etc/apt/keyrings/sagernet.asc"),
		r.Run("Set sing-box key permissions", 10*time.Second, nil, "chmod", "a+r", "/etc/apt/keyrings/sagernet.asc"),
		r.Run("Write sing-box apt source", 10*time.Second, strings.NewReader(singBoxAPTSource), "tee", "/etc/apt/sources.list.d/sagernet.sources"),
	)
	if failed(results) {
		return results
	}
	results = append(results,
		r.Run("Update apt metadata", installTimeout, nil, "env", "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "update"),
		aptInstall(r, "Install sing-box", "sing-box"),
	)
	return results
}

func (i singBoxInstaller) Uninstall(r runner.Runner) []runner.Result {
	return []runner.Result{aptRemove(r, "Remove sing-box", "sing-box")}
}

type snxRSInstaller struct{}

func (snxRSInstaller) Name() string        { return "snx-rs" }
func (snxRSInstaller) DisplayName() string { return "SNX-RS" }

func (i snxRSInstaller) Check(r runner.Runner) Status {
	results := checkBinaries(r, i.DisplayName(), []string{"snxctl"})
	installed := allOK(results)
	if installed {
		return Status{Name: i.Name(), DisplayName: i.DisplayName(), Installed: true, Installable: true, State: "installed", Summary: "SNX-RS is ready.", Commands: results}
	}
	return Status{Name: i.Name(), DisplayName: i.DisplayName(), Installed: false, Installable: true, State: "available", Summary: "Installs SNX-RS from the upstream APT repository. A simulated apt install is run before changing packages.", Commands: results}
}

func (i snxRSInstaller) Install(r runner.Runner) []runner.Result {
	results := []runner.Result{
		r.Run("Check SNX-RS supported OS", 5*time.Second, nil, "sh", "-c", `. /etc/os-release; case "$ID:$VERSION_ID" in debian:12|debian:13|ubuntu:22.04|ubuntu:24.04) exit 0;; *) echo "unsupported OS: $ID $VERSION_ID"; exit 1;; esac`),
		r.Run("Install SNX-RS repository prerequisites", installTimeout, nil, "env", "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "install", "-y", "--no-install-recommends", "ca-certificates", "curl"),
	}
	if failed(results) {
		return results
	}
	results = append(results,
		r.Run("Configure SNX-RS apt source", 30*time.Second, nil, "curl", "-fsSL", "-o", "/etc/apt/sources.list.d/snx-rs.sources", "https://ancwrd1.github.io/snx-rs/snx-rs.sources"),
		r.Run("Update apt metadata", installTimeout, nil, "env", "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "update"),
		r.Run("Simulate SNX-RS install", installTimeout, nil, "env", "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "-s", "install", "snx-rs"),
	)
	if failed(results) {
		return results
	}
	results = append(results, aptInstall(r, "Install SNX-RS", "snx-rs"))
	return results
}

func (i snxRSInstaller) Uninstall(r runner.Runner) []runner.Result {
	return []runner.Result{aptRemove(r, "Remove SNX-RS", "snx-rs")}
}

type xrayInstaller struct{}

func (xrayInstaller) Name() string        { return "xray" }
func (xrayInstaller) DisplayName() string { return "Xray" }

func (i xrayInstaller) Check(r runner.Runner) Status {
	results := checkBinaries(r, i.DisplayName(), []string{"xray"})
	installed := allOK(results)
	if installed {
		return Status{Name: i.Name(), DisplayName: i.DisplayName(), Installed: true, Installable: true, State: "installed", Summary: "Xray is ready.", Commands: results}
	}
	return Status{Name: i.Name(), DisplayName: i.DisplayName(), Installed: false, Installable: true, State: "available", Summary: "Installs Xray using the official XTLS/Xray-install script.", Commands: results}
}

func (i xrayInstaller) Install(r runner.Runner) []runner.Result {
	results := []runner.Result{
		r.Run("Install Xray installer prerequisites", installTimeout, nil, "env", "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "install", "-y", "--no-install-recommends", "ca-certificates", "curl", "unzip"),
		r.Run("Create Granger installer cache", 10*time.Second, nil, "mkdir", "-p", "/var/lib/granger/installers"),
	}
	if failed(results) {
		return results
	}
	results = append(results,
		r.Run("Download Xray official installer", 30*time.Second, nil, "curl", "-fsSL", "https://github.com/XTLS/Xray-install/raw/main/install-release.sh", "-o", "/var/lib/granger/installers/xray-install-release.sh"),
		r.Run("Install Xray", installTimeout, nil, "bash", "/var/lib/granger/installers/xray-install-release.sh", "install", "-u", "root"),
	)
	return results
}

func (i xrayInstaller) Uninstall(r runner.Runner) []runner.Result {
	return []runner.Result{r.Run("Remove Xray", installTimeout, nil, "bash", "/var/lib/granger/installers/xray-install-release.sh", "remove")}
}

func checkBinaries(r runner.Runner, display string, binaries []string) []runner.Result {
	results := make([]runner.Result, 0, len(binaries))
	for _, binary := range binaries {
		results = append(results, r.Run("Check "+display+" binary "+binary, 2*time.Second, nil, "sh", "-c", "command -v "+shellQuote(binary)))
	}
	return results
}

func allOK(results []runner.Result) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if !result.OK {
			return false
		}
	}
	return true
}

func failed(results []runner.Result) bool {
	for _, result := range results {
		if !result.OK {
			return true
		}
	}
	return false
}

func aptInstall(r runner.Runner, title string, packages ...string) runner.Result {
	args := append([]string{"DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "install", "-y", "--no-install-recommends"}, packages...)
	return r.Run(title, installTimeout, nil, "env", args...)
}

func aptRemove(r runner.Runner, title string, packages ...string) runner.Result {
	args := append([]string{"DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "apt-get", "remove", "-y"}, packages...)
	return r.Run(title, installTimeout, nil, "env", args...)
}

func normalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
