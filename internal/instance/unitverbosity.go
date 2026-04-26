package instance

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var reExecVerbosity = regexp.MustCompile(`(^|\s)-v\s+(\d+)`)

// ReadUnitVerbosity returns the configured -v level (1..5) for palman-<name>.service.
// If -v is not present, level 1 is assumed.
func ReadUnitVerbosity(name string) (int, error) {
	path := UnitPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "ExecStart=") || strings.HasPrefix(line, "ExecStart=-") {
			continue
		}
		execLine := strings.TrimPrefix(line, "ExecStart=")
		m := reExecVerbosity.FindStringSubmatch(execLine)
		if len(m) < 3 {
			return 1, nil
		}
		n, err := strconv.Atoi(strings.TrimSpace(m[2]))
		if err != nil {
			return 1, nil
		}
		if n < 1 {
			n = 1
		}
		if n > 5 {
			n = 5
		}
		return n, nil
	}
	return 0, fmt.Errorf("%s: no ExecStart= line found", path)
}

// PatchUnitVerbosity rewrites or appends -v <level> in palman-<name>.service.
func PatchUnitVerbosity(name string, level int) error {
	if level < 1 || level > 5 {
		return fmt.Errorf("verbosity must be between 1 and 5")
	}
	path := UnitPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw := strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	patched := false
	for _, line := range lines {
		if strings.HasPrefix(line, "ExecStart=") && !strings.HasPrefix(line, "ExecStart=-") && !patched {
			execLine := strings.TrimPrefix(line, "ExecStart=")
			if reExecVerbosity.MatchString(execLine) {
				execLine = reExecVerbosity.ReplaceAllString(execLine, "${1}-v "+strconv.Itoa(level))
			} else {
				execLine = strings.TrimSpace(execLine) + " -v " + strconv.Itoa(level)
			}
			out = append(out, "ExecStart="+execLine)
			patched = true
			continue
		}
		out = append(out, line)
	}
	if !patched {
		return fmt.Errorf("%s: no ExecStart= line found", path)
	}
	final := strings.Join(out, "\n") + "\n"
	if err := os.WriteFile(path, []byte(final), 0o644); err != nil {
		return err
	}
	return ReloadDaemon()
}
