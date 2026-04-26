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

// Command names whose minimum user rank (member/wizard/god/owner) is configurable, matching
// mansionsource-go internal/server/rank_cmds.go DefaultCommandRanks and wizard promotion commands.
var rankConfigCommands = []struct {
	Name  string
	Label string
}{
	{"rankset", "`rankset` — set required rank for a command"},
	{"setrank", "`setrank` — same as rankset"},
	{"rankshow", "`rankshow` — list command rank settings"},
	{"rankclear", "`rankclear` — clear a rank override"},
	{"wizpass", "`wizpass` — set wizard/operator promotion password"},
	{"oppass", "`oppass` — same as wizpass"},
	{"godpass", "`godpass` — set god promotion password"},
	{"ownerpass", "`ownerpass` — set owner promotion password"},
}

// Built-in default ranks (config.CommandRank: 1=member … 4=owner)
var rankPromotionDefaults = map[string]int{
	"rankset":   4,
	"setrank":   4,
	"rankshow":  4,
	"rankclear": 4,
	"wizpass":   3,
	"oppass":    3,
	"godpass":   4,
	"ownerpass": 4,
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
		Name        string `json:"name"`
		Label       string `json:"label"`
		DefaultRank int    `json:"defaultRank"`
		Override    *int   `json:"override"`
		Effective   int    `json:"effective"`
	}
	rows := make([]row, 0, len(rankConfigCommands))
	for _, c := range rankConfigCommands {
		def, ok := rankPromotionDefaults[strings.ToLower(c.Name)]
		if !ok {
			continue
		}
		ov, has := overrides[strings.ToLower(c.Name)]
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
			Name:        c.Name,
			Label:       c.Label,
			DefaultRank: def,
			Override:    op,
			Effective:   eff,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"commands":  rows,
		"schema":    "command-ranks v1 — see mansionsource-go config.CommandRank and serverprefs command_ranks",
		"rankNames": map[int]string{1: "member", 2: "wizard", 3: "god", 4: "owner"},
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

	allowed := make(map[string]struct{})
	for k := range rankPromotionDefaults {
		allowed[k] = struct{}{}
	}

	norm := make(map[string]*int)
	for k, v := range req.Ranks {
		ks := strings.ToLower(strings.TrimSpace(k))
		if _, ok := allowed[ks]; !ok {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown or unsupported command name %q", k))
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
		if *p < 1 || *p > 4 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("ranks in this panel must be member (1) through owner (4), or use default: clear %q", cmd))
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

	// Other commands' overrides are preserved; we only update keys we were sent
	for cmd, p := range req.Ranks {
		if p == nil {
			delete(overrides, cmd)
			continue
		}
		def := rankPromotionDefaults[cmd]
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
		// Stable order in JSON: sort keys
		keys := make([]string, 0, len(overrides))
		for k := range overrides {
			keys = append(keys, k)
		}
		sort.Strings(keys)
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
