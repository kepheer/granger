package runtime

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"granger/internal/config"
	"granger/internal/driver"
	"granger/internal/engine"
	"granger/internal/protocols"
	"granger/pkg/runner"
)

type Dashboard struct {
	CollectedAt       time.Time              `json:"collected_at"`
	CPU               CPU                    `json:"cpu"`
	Memory            Memory                 `json:"memory"`
	Disk              Disk                   `json:"disk"`
	Network           Network                `json:"network"`
	NetworkInterfaces int                    `json:"network_interfaces"`
	RouteRules        int                    `json:"route_rules"`
	Outputs           int                    `json:"outputs"`
	Upstreams         int                    `json:"upstreams"`
	Users             int                    `json:"users"`
	Runtime           []driver.RuntimeStatus `json:"runtime"`
	Protocols         []protocols.Status     `json:"protocols"`
	Services          []Service              `json:"services"`
}

type CPU struct {
	Load1  float64 `json:"load_1"`
	Load5  float64 `json:"load_5"`
	Load15 float64 `json:"load_15"`
	Cores  int     `json:"cores"`
}

type Memory struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type Disk struct {
	TotalBytes  uint64  `json:"total_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type Network struct {
	RXBytes uint64 `json:"rx_bytes"`
	TXBytes uint64 `json:"tx_bytes"`
}

type Service struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
	State  string `json:"state"`
}

func Collect(cfg config.Config, r runner.Runner, e engine.Engine) Dashboard {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if time.Since(cachedAt) < 3*time.Second {
		return cachedDashboard
	}
	runtimeStatuses := e.Runtime(cfg)
	cachedDashboard = Dashboard{
		CollectedAt:       time.Now(),
		CPU:               readCPU(),
		Memory:            readMemory(),
		Disk:              readDisk("/"),
		Network:           readNetwork(),
		NetworkInterfaces: interfaceCount(),
		RouteRules:        len(cfg.Rules),
		Outputs:           len(cfg.Outputs),
		Upstreams:         len(cfg.Upstreams),
		Users:             len(cfg.Users),
		Runtime:           runtimeStatuses,
		Protocols:         protocols.New(r).StatusAll(),
		Services:          collectServices(r, runtimeStatuses),
	}
	cachedAt = time.Now()
	return cachedDashboard
}

var (
	cacheMu         sync.Mutex
	cachedAt        time.Time
	cachedDashboard Dashboard
)

func readCPU() CPU {
	load := readFirstLine("/proc/loadavg")
	fields := strings.Fields(load)
	return CPU{
		Load1:  parseFloat(field(fields, 0)),
		Load5:  parseFloat(field(fields, 1)),
		Load15: parseFloat(field(fields, 2)),
		Cores:  runtime.NumCPU(),
	}
}

func readMemory() Memory {
	values := map[string]uint64{}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return Memory{}
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, _ := strconv.ParseUint(fields[1], 10, 64)
		values[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	return Memory{TotalBytes: total, AvailableBytes: available, UsedPercent: percent(total-available, total)}
}

func readDisk(path string) Disk {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return Disk{}
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	return Disk{TotalBytes: total, FreeBytes: free, UsedPercent: percent(total-free, total)}
}

func interfaceCount() int {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return 0
	}
	return len(entries)
}

func readNetwork() Network {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return Network{}
	}
	defer f.Close()
	var total Network
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}
		fields := strings.Fields(strings.Replace(line, ":", " ", 1))
		if len(fields) < 10 || fields[0] == "lo" {
			continue
		}
		rx, _ := strconv.ParseUint(fields[1], 10, 64)
		tx, _ := strconv.ParseUint(fields[9], 10, 64)
		total.RXBytes += rx
		total.TXBytes += tx
	}
	return total
}

func collectServices(r runner.Runner, statuses []driver.RuntimeStatus) []Service {
	names := []string{"granger.service", "dnsmasq.service"}
	seen := map[string]bool{"granger.service": true, "dnsmasq.service": true}
	for _, status := range statuses {
		name := strings.TrimSpace(status.Service)
		if name == "" || seen[name] {
			continue
		}
		names = append(names, name)
		seen[name] = true
	}
	out := make([]Service, 0, len(names))
	for _, name := range names {
		result := r.Run("Status "+name, 3*time.Second, nil, "systemctl", "is-active", name)
		out = append(out, Service{Name: name, Active: result.OK, State: strings.TrimSpace(result.Output)})
	}
	return out
}

func readFirstLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}

func field(fields []string, index int) string {
	if index >= len(fields) {
		return ""
	}
	return fields[index]
}

func parseFloat(value string) float64 {
	n, _ := strconv.ParseFloat(value, 64)
	return n
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) * 100 / float64(total)
}
