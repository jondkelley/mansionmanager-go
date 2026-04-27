package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"palace-manager/internal/authstore"

	"golang.org/x/crypto/bcrypt"
)

const (
	hostPassDefaultPath = "/etc/palacehostpass"
	hostPassLegacyPath  = "/etc/hostpass"
)

func resolveHostPassPath() string {
	// mansionsource-go loads and watches /etc/palacehostpass specifically.
	return hostPassDefaultPath
}

func migrateLegacyHostPassIfNeeded(path string) {
	if path != hostPassDefaultPath {
		return
	}
	if _, err := os.Stat(hostPassDefaultPath); err == nil {
		return
	}
	b, err := os.ReadFile(hostPassLegacyPath)
	if err != nil || len(b) == 0 {
		return
	}
	_ = os.WriteFile(hostPassDefaultPath, b, 0o644)
}

func isBcryptHashString(s string) bool {
	_, err := bcrypt.Cost([]byte(s))
	return err == nil
}

func (s *Server) routeWizPasses(w http.ResponseWriter, r *http.Request) {
	if !s.requirePrimaryAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleWizPassesList(w, r)
	case http.MethodPost:
		s.handleWizPassesCreate(w, r)
	case http.MethodDelete:
		s.handleWizPassesDelete(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWizPassesList(w http.ResponseWriter, _ *http.Request) {
	path := resolveHostPassPath()
	migrateLegacyHostPassIfNeeded(path)
	if _, err := os.Stat(path); err == nil {
		_ = os.Chmod(path, 0o644)
	}
	entries, err := readHostPassEntries(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	globalCount := 0
	for _, e := range entries {
		if e.Scope == "global" {
			globalCount++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":        path,
		"globalCount": globalCount,
		"entries":     entries,
	})
}

type hostPassEntry struct {
	Line     int    `json:"line"`
	Scope    string `json:"scope"`
	Username string `json:"username,omitempty"`
}

func readHostPassEntries(path string) ([]hostPassEntry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries := make([]hostPassEntry, 0)
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if isBcryptHashString(line) {
			entries = append(entries, hostPassEntry{
				Line:  lineNo,
				Scope: "global",
			})
			continue
		}
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		user := strings.TrimSpace(line[:idx])
		hash := strings.TrimSpace(line[idx+1:])
		if user == "" || !isBcryptHashString(hash) {
			continue
		}
		entries = append(entries, hostPassEntry{
			Line:     lineNo,
			Scope:    "user",
			Username: user,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Server) handleWizPassesCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope    string `json:"scope"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope != "global" && scope != "user" {
		writeError(w, http.StatusBadRequest, `scope must be "global" or "user"`)
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	username := strings.TrimSpace(req.Username)
	if scope == "user" {
		if username == "" {
			writeError(w, http.StatusBadRequest, "username is required for user scope")
			return
		}
		if strings.ContainsRune(username, ':') {
			writeError(w, http.StatusBadRequest, "username must not contain ':'")
			return
		}
	}

	hash, err := authstore.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	newLine := hash
	if scope == "user" {
		newLine = username + ":" + hash
	}
	path := resolveHostPassPath()
	migrateLegacyHostPassIfNeeded(path)
	if err := s.appendHostPassLine(path, newLine); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeAudit(r.Context(), "wizpass.create", "", map[string]string{"scope": scope, "username": username})
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":       true,
		"path":     path,
		"scope":    scope,
		"username": username,
	})
}

func (s *Server) appendHostPassLine(path, line string) error {
	s.hostPassMu.Lock()
	defer s.hostPassMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	needsLeadingNewline := false
	if data, err := os.ReadFile(path); err == nil {
		needsLeadingNewline = len(data) > 0 && data[len(data)-1] != '\n'
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if needsLeadingNewline {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}
	// Keep readable by the pserver service user.
	if err := f.Chmod(0o644); err != nil {
		return err
	}
	return nil
}

func (s *Server) handleWizPassesDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Line int `json:"line"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Line <= 0 {
		writeError(w, http.StatusBadRequest, "line must be a positive integer")
		return
	}
	path := resolveHostPassPath()
	migrateLegacyHostPassIfNeeded(path)
	removed, err := s.deleteHostPassLine(path, req.Line)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !removed {
		writeError(w, http.StatusNotFound, "entry not found")
		return
	}
	s.writeAudit(r.Context(), "wizpass.delete", "", map[string]string{"line": strconv.Itoa(req.Line)})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"path": path,
		"line": req.Line,
	})
}

func (s *Server) deleteHostPassLine(path string, lineNo int) (bool, error) {
	s.hostPassMu.Lock()
	defer s.hostPassMu.Unlock()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(data), "\n")
	if lineNo < 1 || lineNo > len(lines) {
		return false, nil
	}

	removedLine := strings.TrimSpace(lines[lineNo-1])
	if removedLine == "" || strings.HasPrefix(removedLine, "#") {
		return false, nil
	}

	out := make([]string, 0, len(lines)-1)
	for i, line := range lines {
		if i == lineNo-1 {
			continue
		}
		out = append(out, line)
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	body := strings.Join(out, "\n")
	if body != "" {
		body += "\n"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}
