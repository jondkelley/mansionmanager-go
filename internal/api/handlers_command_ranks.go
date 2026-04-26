package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func isAllowedRankCommandKey(key string) bool {
	key = strings.TrimSpace(strings.ToLower(key))
	if _, ok := defaultCommandRanks[key]; ok {
		return true
	}
	if len(key) == 0 || len(key) > 48 {
		return false
	}
	for i, r := range key {
		if i == 0 {
			if r < 'a' || r > 'z' {
				return false
			}
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func readCommandRanksMap(path string) (map[string]int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, err
	}
	raw, ok := top["command_ranks"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return map[string]int{}, nil
	}
	var m map[string]int
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("command_ranks: %w", err)
	}
	return m, nil
}

func sortedCommandKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *Server) handleCommandRanksGet(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	path := filepath.Join(dir, "serverprefs.json")
	overrides, err := readCommandRanksMap(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if overrides == nil {
		overrides = map[string]int{}
	}

	type row struct {
		Name         string `json:"name"`
		Label        string `json:"label"`
		DefaultRank  int    `json:"defaultRank"`
		Override     *int   `json:"override"`
		Effective    int    `json:"effective"`
		ExtraInPrefs bool   `json:"extraInPrefs,omitempty"`
	}

	seen := make(map[string]struct{})
	rows := make([]row, 0, len(defaultCommandRanks)+len(overrides))

	for _, cmd := range sortedCommandKeys(defaultCommandRanks) {
		seen[cmd] = struct{}{}
		def := defaultCommandRanks[cmd]
		ov, has := overrides[cmd]
		var op *int
		if has {
			i := ov
			op = &i
		}
		eff := def
		if has {
			eff = ov
		}
		rows = append(rows, row{
			Name:        cmd,
			Label:     fmt.Sprintf("`%s` — built-in default: %s", cmd, rankTierWord(def)),
			DefaultRank: def,
			Override:    op,
			Effective:   eff,
		})
	}

	for _, cmd := range sortedCommandKeys(overrides) {
		if _, ok := seen[cmd]; ok {
			continue
		}
		def := intrinsicDefaultRank(cmd)
		ov := overrides[cmd]
		op := ov
		rows = append(rows, row{
			Name:         cmd,
			Label:        fmt.Sprintf("`%s` — not in server DefaultCommandRanks; implicit default: wizard", cmd),
			DefaultRank:  def,
			Override:     &op,
			Effective:    ov,
			ExtraInPrefs: true,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"commands": rows,
		"schema":   "command-ranks v2 — full DefaultCommandRanks + extra serverprefs keys; sync with mansionsource-go internal/server/rank_cmds.go",
		"rankNames": map[int]string{
			0: "guest",
			1: "member",
			2: "wizard",
			3: "god",
			4: "owner",
		},
		"commandCount": len(rows),
	})
}

func (s *Server) handleCommandRanksPut(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	var req struct {
		Ranks map[string]*int `json:"ranks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Ranks == nil {
		req.Ranks = make(map[string]*int)
	}

	norm := make(map[string]*int)
	for k, v := range req.Ranks {
		ks := strings.ToLower(strings.TrimSpace(k))
		if !isAllowedRankCommandKey(ks) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid command name %q (use a-z, digits, underscore; or a known server command)", k))
			return
		}
		norm[ks] = v
	}
	req.Ranks = norm
	if len(req.Ranks) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": palaceName, "note": "no rank keys in request; nothing changed"})
		return
	}

	for cmd, p := range req.Ranks {
		if p == nil {
			continue
		}
		if *p < 0 || *p > 4 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("rank for %q must be 0 (guest) through 4 (owner), or null for built-in default", cmd))
			return
		}
	}

	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	fullPath := filepath.Join(dir, "serverprefs.json")
	oldSz := fileSizeOrZero(fullPath)

	overrides, err := readCommandRanksMap(fullPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if overrides == nil {
		overrides = make(map[string]int)
	}

	for cmd, p := range req.Ranks {
		if p == nil {
			delete(overrides, cmd)
			continue
		}
		def := intrinsicDefaultRank(cmd)
		if *p == def {
			delete(overrides, cmd)
		} else {
			overrides[cmd] = *p
		}
	}

	var top map[string]any
	b, rerr := os.ReadFile(fullPath)
	switch {
	case rerr != nil && os.IsNotExist(rerr):
		top = make(map[string]any)
	case rerr != nil:
		writeError(w, http.StatusInternalServerError, rerr.Error())
		return
	case len(b) == 0:
		top = make(map[string]any)
	default:
		if err := json.Unmarshal(b, &top); err != nil {
			writeError(w, http.StatusInternalServerError, "serverprefs.json: "+err.Error())
			return
		}
		if top == nil {
			top = make(map[string]any)
		}
	}

	if len(overrides) == 0 {
		delete(top, "command_ranks")
	} else {
		keys := sortedCommandKeys(overrides)
		ord := make(map[string]int, len(overrides))
		for _, k := range keys {
			ord[k] = overrides[k]
		}
		top["command_ranks"] = ord
	}

	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(out) > maxPalaceServerFile {
		writeError(w, http.StatusRequestEntityTooLarge, "serverprefs too large after update")
		return
	}
	if err := s.quotaRejectAfterChange(palaceName, oldSz, int64(len(out))); err != nil {
		writeError(w, http.StatusInsufficientStorage, err.Error())
		return
	}
	if err := writeFileAtomicAs(fullPath, s.palaceLinuxUser(palaceName), strings.NewReader(string(out)+"\n")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": palaceName, "file": "serverprefs.json"})
}

// handlePalaceReloadConfig sends SIGHUP to pserver so it reloads pserver.pat, pserver.prefs, and serverprefs.json
// (see mansionsource-go ReadScriptFile / SIGHUP), without a full process restart.
func (s *Server) handlePalaceReloadConfig(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	if err := s.instances.Reload(palaceName); err != nil {
		writeError(w, http.StatusInternalServerError, "reload: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"note": "SIGHUP sent; pserver should reload pserver.pat, pserver.prefs, and serverprefs.json (check unit is active and ExecReload=kill -HUP is configured)",
	})
}
