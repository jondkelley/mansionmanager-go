package instance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"palace-manager/internal/registry"
	"palace-manager/internal/unregistered"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusFailed   Status = "failed"
	StatusUnknown  Status = "unknown"
)

type Instance struct {
	Name           string `json:"name"`
	User           string `json:"user"`
	TCPPort        int    `json:"tcpPort"`
	HTTPPort       int    `json:"httpPort"`
	DataDir        string `json:"dataDir"`
	Status         Status `json:"status"`
	ProvisionedAt  string `json:"provisionedAt,omitempty"`
	UnitName       string `json:"unitName"`
	PserverVersion string `json:"pserverVersion,omitempty"` // pinned semver, or omitted/"latest"
	Registered     bool   `json:"registered"`               // false = systemd unit exists but not in palace-manager registry
	MediaDir       string `json:"mediaDir,omitempty"`       // absolute path from palman unit -m (or …/media)
	YPHost         string `json:"ypHost,omitempty"`         // directory announce host → YPMYEXTADDR
	YPPort         int    `json:"ypPort,omitempty"`         // directory announce port → YPMYEXTPORT
	QuotaBytesMax  int64  `json:"quotaBytesMax,omitempty"`  // 0 = unlimited
	HomeUsedBytes  int64  `json:"homeUsedBytes,omitempty"`  // set when quotaBytesMax > 0
	QuotaExceeded  bool   `json:"quotaExceeded,omitempty"`  // homeUsedBytes > quotaBytesMax
}

type systemdUnit struct {
	Unit        string `json:"unit"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
}

type Manager struct {
	reg   *registry.Registry
	unreg *unregistered.Store
}

func NewManager(reg *registry.Registry, unreg *unregistered.Store) *Manager {
	return &Manager{reg: reg, unreg: unreg}
}

func (m *Manager) List() ([]Instance, error) {
	units, err := listUnits()
	if err != nil {
		return nil, err
	}

	var instances []Instance
	for _, u := range units {
		name := unitToName(u.Unit)
		if name == "" {
			continue
		}

		inst := Instance{
			Name:     name,
			UnitName: u.Unit,
			Status:   toStatus(u.ActiveState),
		}

		if p, ok := m.reg.Get(name); ok {
			inst.Registered = true
			inst.User = p.User
			inst.TCPPort = p.TCPPort
			inst.HTTPPort = p.HTTPPort
			inst.DataDir = p.DataDir
			inst.YPHost = p.YPHost
			inst.YPPort = p.YPPort
			inst.QuotaBytesMax = p.QuotaBytesMax
			if !p.ProvisionedAt.IsZero() {
				inst.ProvisionedAt = p.ProvisionedAt.Format("2006-01-02T15:04:05Z")
			}
		}

		instances = append(instances, inst)
	}

	// Also include registry entries that don't have a unit yet
	for _, p := range m.reg.All() {
		found := false
		for _, inst := range instances {
			if inst.Name == p.Name {
				found = true
				break
			}
		}
		if !found {
			u := fmt.Sprintf("palman-%s.service", p.Name)
			instances = append(instances, Instance{
				Name:          p.Name,
				Registered:    true,
				User:          p.User,
				TCPPort:       p.TCPPort,
				HTTPPort:      p.HTTPPort,
				DataDir:       p.DataDir,
				YPHost:        p.YPHost,
				YPPort:        p.YPPort,
				QuotaBytesMax: p.QuotaBytesMax,
				UnitName:      u,
				// list-units + glob sometimes omits loaded units; ask systemd directly.
				Status: queryUnitStatus(u),
			})
		}
	}

	// Tombstone records: unregistered but not discoverable via list-units (or no unit file).
	if m.unreg != nil {
		seen := make(map[string]struct{}, len(instances))
		for _, inst := range instances {
			seen[inst.Name] = struct{}{}
		}
		for _, rec := range m.unreg.All() {
			if _, ok := seen[rec.Name]; ok {
				continue
			}
			u := unitName(rec.Name)
			pv := rec.PserverVersion
			instances = append(instances, Instance{
				Name:           rec.Name,
				User:           rec.User,
				TCPPort:        rec.TCPPort,
				HTTPPort:       rec.HTTPPort,
				DataDir:        rec.DataDir,
				YPHost:         rec.YPHost,
				YPPort:         rec.YPPort,
				QuotaBytesMax:  rec.QuotaBytesMax,
				Registered:     false,
				UnitName:       u,
				Status:         queryUnitStatus(u),
				PserverVersion: pv,
			})
		}
	}

	return instances, nil
}

func (m *Manager) Get(name string) (Instance, error) {
	instances, err := m.List()
	if err != nil {
		return Instance{}, err
	}
	for _, inst := range instances {
		if inst.Name == name {
			return inst, nil
		}
	}
	return Instance{}, fmt.Errorf("palace %q not found", name)
}

func (m *Manager) Start(name string) error {
	return systemctl("start", unitName(name))
}

func (m *Manager) Stop(name string) error {
	return systemctl("stop", unitName(name))
}

func (m *Manager) Restart(name string) error {
	return systemctl("restart", unitName(name))
}

// Reload sends SIGHUP so pserver reloads pat, prefs, and serverprefs without a full restart.
func (m *Manager) Reload(name string) error {
	return systemctl("kill", "-s", "HUP", unitName(name))
}

// UnitPath returns the filesystem path for palman-<name>.service.
func UnitPath(name string) string {
	return filepath.Join("/etc/systemd/system", unitName(name))
}

var (
	reUnitUser             = regexp.MustCompile(`(?m)^User=(.+)$`)
	reUnitWorkingDirectory = regexp.MustCompile(`(?m)^WorkingDirectory=(.+)$`)
	reUnitExecStart        = regexp.MustCompile(`(?m)^ExecStart=(.+)$`)
	reExecTCP              = regexp.MustCompile(`(?:^|\s)-p\s+(\d+)`)
	reExecHTTP             = regexp.MustCompile(`(?:^|\s)-H\s+(\d+)`)
)

// DiscoverFromUnit reads /etc/systemd/system/palman-<name>.service and extracts
// service user, WorkingDirectory, and TCP/HTTP ports from ExecStart (-p / -H).
func DiscoverFromUnit(name string) (linuxUser string, tcpPort, httpPort int, dataDir string, err error) {
	path := UnitPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("unit file %s: %w", path, err)
	}
	content := string(data)
	if m := reUnitUser.FindStringSubmatch(content); len(m) > 1 {
		linuxUser = strings.TrimSpace(m[1])
	}
	if m := reUnitWorkingDirectory.FindStringSubmatch(content); len(m) > 1 {
		dataDir = strings.TrimSpace(m[1])
	}
	var execLine string
	if m := reUnitExecStart.FindStringSubmatch(content); len(m) > 1 {
		execLine = strings.TrimSpace(m[1])
	}
	if execLine != "" {
		if m := reExecTCP.FindStringSubmatch(execLine); len(m) > 1 {
			tcpPort, _ = strconv.Atoi(m[1])
		}
		if m := reExecHTTP.FindStringSubmatch(execLine); len(m) > 1 {
			httpPort, _ = strconv.Atoi(m[1])
		}
	}
	if linuxUser == "" {
		linuxUser = name
	}
	if dataDir == "" {
		dataDir = filepath.Join("/home", linuxUser, "palace")
	}
	return linuxUser, tcpPort, httpPort, dataDir, nil
}

// EnableNow runs systemctl enable --now for the palace unit (boot + start).
func (m *Manager) EnableNow(name string) error {
	return systemctl("enable", "--now", unitName(name))
}

// Disable stops the palman unit and disables it at boot.
// If removeUnitFile is true, the unit file under /etc/systemd/system is deleted (used when purging the Linux user).
// For unregister-only, pass false so the unit remains on disk and can be discovered and re-registered.
func (m *Manager) Disable(name string, removeUnitFile bool) error {
	if err := systemctl("stop", unitName(name)); err != nil {
		return err
	}
	if err := systemctl("disable", unitName(name)); err != nil {
		return err
	}
	if removeUnitFile {
		_ = os.Remove(UnitPath(name))
	}
	return systemctl("daemon-reload")
}

// RewriteReverseProxyMedia updates --reverseproxymedia on every palman-*.service unit, then daemon-reload.
func (m *Manager) RewriteReverseProxyMedia(newBase string) error {
	units, err := listUnits()
	if err != nil {
		return err
	}
	var lastErr error
	for _, u := range units {
		path := filepath.Join("/etc/systemd/system", u.Unit)
		if err := PatchUnitReverseProxy(path, newBase); err != nil {
			lastErr = fmt.Errorf("%s: %w", u.Unit, err)
		}
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	return lastErr
}

func (m *Manager) RestartAll() error {
	units, err := listUnits()
	if err != nil {
		return err
	}
	var lastErr error
	for _, u := range units {
		if err := systemctl("restart", u.Unit); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (m *Manager) TailLog(name string, lines int) ([]string, error) {
	p, ok := m.reg.Get(name)
	if !ok {
		return nil, fmt.Errorf("palace %q not found in registry", name)
	}
	logPath := filepath.Join(p.DataDir, "pserver.log")

	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}

	if len(all) <= lines {
		return all, nil
	}
	return all[len(all)-lines:], nil
}

func listUnits() ([]systemdUnit, error) {
	out, err := exec.Command(
		"systemctl", "list-units",
		"--type=service", "--all",
		"--output=json",
		"palman-*.service",
	).Output()
	if err != nil {
		// systemctl exits 1 when no units match; treat empty as ok
		if len(out) == 0 {
			return nil, nil
		}
	}

	// systemd's table JSON uses short keys ("active", "sub") matching the column headers,
	// not active_state / sub_state — see systemctl list-units --output=json.
	var raw []struct {
		Unit        string `json:"unit"`
		Active      string `json:"active"`
		Sub         string `json:"sub"`
		ActiveState string `json:"active_state"`
		SubState    string `json:"sub_state"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing systemctl output: %w", err)
	}

	units := make([]systemdUnit, 0, len(raw))
	for _, r := range raw {
		if !strings.HasPrefix(r.Unit, "palman-") || !strings.HasSuffix(r.Unit, ".service") {
			continue
		}
		active := r.ActiveState
		if active == "" {
			active = r.Active
		}
		sub := r.SubState
		if sub == "" {
			sub = r.Sub
		}
		units = append(units, systemdUnit{
			Unit:        r.Unit,
			ActiveState: active,
			SubState:    sub,
		})
	}
	return units, nil
}

func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func unitName(palaceName string) string {
	return fmt.Sprintf("palman-%s.service", palaceName)
}

func unitToName(unit string) string {
	name := strings.TrimPrefix(unit, "palman-")
	name = strings.TrimSuffix(name, ".service")
	if name == unit {
		return ""
	}
	return name
}

func toStatus(activeState string) Status {
	switch activeState {
	case "active", "activating", "reloading":
		return StatusActive
	case "inactive", "deactivating":
		return StatusInactive
	case "failed":
		return StatusFailed
	default:
		return StatusUnknown
	}
}

// queryUnitStatus resolves LoadState/ActiveState when list-units did not return this unit
// (pattern matching quirks) or for extra verification.
func queryUnitStatus(unit string) Status {
	out, err := exec.Command("systemctl", "show", unit, "-p", "LoadState", "-p", "ActiveState").Output()
	if err != nil {
		return StatusUnknown
	}
	var loadState, activeState string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "LoadState=") {
			loadState = strings.TrimPrefix(line, "LoadState=")
		}
		if strings.HasPrefix(line, "ActiveState=") {
			activeState = strings.TrimPrefix(line, "ActiveState=")
		}
	}
	switch loadState {
	case "not-found", "masked":
		return StatusUnknown
	}
	if loadState == "" && activeState == "" {
		return StatusUnknown
	}
	return toStatus(activeState)
}
