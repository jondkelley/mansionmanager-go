package instance

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// PatchUnitListenPorts rewrites -p and -H port flags on the first ExecStart= line of a palman unit file.
func PatchUnitListenPorts(unitPath string, tcpPort, httpPort int) error {
	if tcpPort <= 0 || tcpPort > 65535 || httpPort <= 0 || httpPort > 65535 {
		return fmt.Errorf("invalid tcp/http port (%d / %d)", tcpPort, httpPort)
	}
	// Distinct from reExecTCP in manager.go: ReplaceAllString needs a capturing group before -p/-H.
	tcpRep := regexp.MustCompile(`(^|\s)-p\s+\d+`)
	httpRep := regexp.MustCompile(`(^|\s)-H\s+\d+`)
	data, err := os.ReadFile(unitPath)
	if err != nil {
		return err
	}
	raw := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	patched := false
	for _, line := range lines {
		if strings.HasPrefix(line, "ExecStart=") && !strings.HasPrefix(line, "ExecStart=-") && !patched {
			rest := strings.TrimPrefix(line, "ExecStart=")
			rest = tcpRep.ReplaceAllString(rest, "${1}-p "+strconv.Itoa(tcpPort))
			rest = httpRep.ReplaceAllString(rest, "${1}-H "+strconv.Itoa(httpPort))
			out = append(out, "ExecStart="+rest)
			patched = true
			continue
		}
		out = append(out, line)
	}
	if !patched {
		return fmt.Errorf("%s: no ExecStart= line found", unitPath)
	}
	final := strings.Join(out, "\n") + "\n"
	return os.WriteFile(unitPath, []byte(final), 0644)
}

// UnitBootEnabled reports whether the unit is enabled for multi-user.target (systemctl is-enabled).
func UnitBootEnabled(palaceName string) bool {
	u := unitName(palaceName)
	out, _ := exec.Command("systemctl", "is-enabled", u).CombinedOutput()
	s := strings.TrimSpace(string(out))
	return s == "enabled" || s == "enabled-runtime"
}

// UnitIsActive reports active state (systemctl is-active).
func UnitIsActive(palaceName string) bool {
	u := unitName(palaceName)
	out, _ := exec.Command("systemctl", "is-active", u).CombinedOutput()
	return strings.TrimSpace(string(out)) == "active"
}

// ReloadDaemon runs systemctl daemon-reload (after unit file edits or renames).
func ReloadDaemon() error {
	cmd := exec.Command("systemctl", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SystemctlEnable runs systemctl enable for palman-<name>.service (boot symlink).
func SystemctlEnable(palaceName string) error {
	cmd := exec.Command("systemctl", "enable", unitName(palaceName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
