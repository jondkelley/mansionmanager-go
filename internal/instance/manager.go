package instance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"palace-manager/internal/registry"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusFailed   Status = "failed"
	StatusUnknown  Status = "unknown"
)

type Instance struct {
	Name          string         `json:"name"`
	User          string         `json:"user"`
	TCPPort       int            `json:"tcpPort"`
	HTTPPort      int            `json:"httpPort"`
	DataDir       string         `json:"dataDir"`
	Status        Status         `json:"status"`
	ProvisionedAt string         `json:"provisionedAt,omitempty"`
	UnitName      string         `json:"unitName"`
}

type systemdUnit struct {
	Unit        string `json:"unit"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
}

type Manager struct {
	reg *registry.Registry
}

func NewManager(reg *registry.Registry) *Manager {
	return &Manager{reg: reg}
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
			inst.User = p.User
			inst.TCPPort = p.TCPPort
			inst.HTTPPort = p.HTTPPort
			inst.DataDir = p.DataDir
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
				Name:     p.Name,
				User:     p.User,
				TCPPort:  p.TCPPort,
				HTTPPort: p.HTTPPort,
				DataDir:  p.DataDir,
				UnitName: u,
				// list-units + glob sometimes omits loaded units; ask systemd directly.
				Status: queryUnitStatus(u),
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

func (m *Manager) Disable(name string) error {
	if err := systemctl("stop", unitName(name)); err != nil {
		return err
	}
	if err := systemctl("disable", unitName(name)); err != nil {
		return err
	}
	unitPath := fmt.Sprintf("/etc/systemd/system/%s", unitName(name))
	_ = os.Remove(unitPath)
	return systemctl("daemon-reload")
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
