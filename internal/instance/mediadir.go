package instance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var reExecMedia = regexp.MustCompile(`(?:^|\s)-m\s+(\S+)`)

// DiscoverMediaDir returns the absolute Palace media folder from the systemd unit (-m flag).
// When -m is absent it defaults to <WorkingDirectory or ~/palace>/media.
func DiscoverMediaDir(name string) (string, error) {
	path := UnitPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("unit file %s: %w", path, err)
	}
	content := string(data)
	var execLine string
	if m := reUnitExecStart.FindStringSubmatch(content); len(m) > 1 {
		execLine = strings.TrimSpace(m[1])
	}
	if m := reExecMedia.FindStringSubmatch(execLine); len(m) > 1 {
		mp := strings.TrimSpace(m[1])
		mp = strings.TrimSuffix(strings.Trim(mp, `"`), `/`)
		return filepath.Clean(mp), nil
	}

	_, _, _, dataDir, err := DiscoverFromUnit(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "media"), nil
}
