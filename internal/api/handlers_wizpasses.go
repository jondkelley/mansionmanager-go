package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"palace-manager/internal/authstore"

	"golang.org/x/crypto/bcrypt"
)

const (
	hostPassDefaultPath = "/etc/palacehostpass"
	hostPassLegacyPath  = "/etc/hostpass"
)

func resolveHostPassPath() string {
	if _, err := os.Stat(hostPassDefaultPath); err == nil {
		return hostPassDefaultPath
	}
	if _, err := os.Stat(hostPassLegacyPath); err == nil {
		return hostPassLegacyPath
	}
	return hostPassDefaultPath
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
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWizPassesList(w http.ResponseWriter, _ *http.Request) {
	path := resolveHostPassPath()
	globalCount, users, err := readHostPassSummary(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":        path,
		"globalCount": globalCount,
		"users":       users,
	})
}

func readHostPassSummary(path string) (int, []string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	globalCount := 0
	userSet := map[string]struct{}{}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if isBcryptHashString(line) {
			globalCount++
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
		userSet[user] = struct{}{}
	}
	if err := sc.Err(); err != nil {
		return 0, nil, err
	}
	users := make([]string, 0, len(userSet))
	for user := range userSet {
		users = append(users, user)
	}
	sort.Strings(users)
	return globalCount, users, nil
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
	if err := s.appendHostPassLine(path, newLine); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
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
	return nil
}
