package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"palace-manager/internal/authstore"
	"palace-manager/internal/serverprefsform"
)

// handleServerPrefsFormGet returns a guided-edit DTO plus which sensitive keys exist on disk.
func (s *Server) handleServerPrefsFormGet(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermSettings) {
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	path := filepath.Join(dir, "serverprefs.json")
	top, err := serverprefsform.LoadRawMap(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, statErr := os.Stat(path)
	writeJSON(w, http.StatusOK, map[string]any{
		"form":           serverprefsform.MapToForm(top),
		"fileExists":     statErr == nil,
		"preservedKeys":  serverprefsform.PreservedKeysPresent(top),
		"schema":         "serverprefs-form v1 — aligned with mansionsource-go internal/serverprefs (incl. wiz_authoring, wiz_authoring_annotation)",
	})
}

// handleServerPrefsFormPut merges the guided form into serverprefs.json, preserves sensitive keys, restarts.
func (s *Server) handleServerPrefsFormPut(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermSettings) {
		return
	}
	var req struct {
		Form serverprefsform.ServerPrefsFormDTO `json:"form"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	path := filepath.Join(dir, "serverprefs.json")
	top, err := serverprefsform.LoadRawMap(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	merged, err := serverprefsform.ApplyFormToMap(top, req.Form)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	data = append(data, '\n')
	if len(data) > maxPalaceServerFile {
		writeError(w, http.StatusRequestEntityTooLarge, "serverprefs.json too large after merge")
		return
	}
	oldSz := fileSizeOrZero(path)
	if err := s.quotaRejectAfterChange(palaceName, oldSz, int64(len(data))); err != nil {
		writeError(w, http.StatusInsufficientStorage, err.Error())
		return
	}
	if err := writeFileAtomicAs(path, s.palaceLinuxUser(palaceName), strings.NewReader(string(data))); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.instances.Restart(palaceName); err != nil {
		writeError(w, http.StatusInternalServerError, "saved serverprefs.json but restart failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"name":      palaceName,
		"file":      "serverprefs.json",
		"restarted": true,
	})
}
