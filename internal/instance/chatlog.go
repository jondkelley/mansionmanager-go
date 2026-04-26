package instance

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"palace-manager/internal/pserverprefs"
)

func chatLogTypesConfigured(types string) bool {
	for _, p := range strings.Split(types, ",") {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "basic", "whisper", "esp":
			return true
		}
	}
	return false
}

// mergeChatLogPrefs scans pserver.prefs then pserver.pat; later files override
// earlier ones for each CHATLOG* field when the new value is non-empty.
func mergeChatLogPrefs(dataDir string) (types, file, format string) {
	paths := []string{
		filepath.Join(dataDir, "pserver.prefs"),
		filepath.Join(dataDir, "pserver.pat"),
	}
	var st pserverprefs.PrefState
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		next, _, _ := pserverprefs.ParsePrefState(string(b))
		if strings.TrimSpace(next.ChatLogTypes) != "" {
			st.ChatLogTypes = next.ChatLogTypes
		}
		if strings.TrimSpace(next.ChatLogFile) != "" {
			st.ChatLogFile = next.ChatLogFile
		}
		if strings.TrimSpace(next.ChatLogFormat) != "" {
			st.ChatLogFormat = next.ChatLogFormat
		}
	}
	return st.ChatLogTypes, st.ChatLogFile, st.ChatLogFormat
}

func resolveChatLogPath(dataDir, fileField string) string {
	p := strings.TrimSpace(fileField)
	if p == "" {
		return filepath.Join(dataDir, "chat.log")
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dataDir, p)
}

func normalizeChatLogFormat(f string) string {
	if strings.EqualFold(strings.TrimSpace(f), "csv") {
		return "csv"
	}
	return "json"
}

// TailChatLog returns the last wantLines non-empty lines of the chat audit log
// (JSON lines or CSV rows, per mansionsource-go internal/chatlog).
func (m *Manager) TailChatLog(name string, wantLines int) (lines []string, fileExists bool, configured bool, format string, logPath string, err error) {
	p, ok := m.reg.Get(name)
	if !ok {
		return nil, false, false, "json", "", fmt.Errorf("palace %q not found in registry", name)
	}
	types, fileField, fmtPref := mergeChatLogPrefs(p.DataDir)
	configured = chatLogTypesConfigured(types)
	format = normalizeChatLogFormat(fmtPref)
	logPath = resolveChatLogPath(p.DataDir, fileField)

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, configured, format, logPath, nil
		}
		return nil, false, configured, format, logPath, err
	}
	defer f.Close()
	fileExists = true

	var all []string
	scanner := bufio.NewScanner(f)
	const maxScan = 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxScan)
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fileExists, configured, format, logPath, scanErr
	}
	if len(all) <= wantLines {
		return all, fileExists, configured, format, logPath, nil
	}
	return all[len(all)-wantLines:], fileExists, configured, format, logPath, nil
}
