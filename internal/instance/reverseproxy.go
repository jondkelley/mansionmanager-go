package instance

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var reReverseProxyFlag = regexp.MustCompile(`--reverseproxymedia\s+\S+`)

// PatchExecReverseProxy replaces the --reverseproxymedia argument in an ExecStart command line.
func PatchExecReverseProxy(execLine, newBase string) (string, error) {
	if !strings.Contains(execLine, "--reverseproxymedia") {
		return "", fmt.Errorf("ExecStart missing --reverseproxymedia")
	}
	return reReverseProxyFlag.ReplaceAllString(execLine, "--reverseproxymedia "+newBase), nil
}

// PatchUnitReverseProxy updates palman-*.service in place with a new --reverseproxymedia base URL.
func PatchUnitReverseProxy(unitPath, newBase string) error {
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
			newRest, err := PatchExecReverseProxy(rest, newBase)
			if err != nil {
				return fmt.Errorf("%s: %w", unitPath, err)
			}
			out = append(out, "ExecStart="+newRest)
			patched = true
			continue
		}
		out = append(out, line)
	}
	if !patched {
		return fmt.Errorf("%s: no ExecStart= with --reverseproxymedia found", unitPath)
	}
	final := strings.Join(out, "\n") + "\n"
	return os.WriteFile(unitPath, []byte(final), 0644)
}
