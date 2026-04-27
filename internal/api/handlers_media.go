package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"palace-manager/internal/authstore"
	"palace-manager/internal/instance"
	"palace-manager/internal/mediadisk"
	"palace-manager/internal/patgrep"
)

func (s *Server) handlePalaceMediaFiles(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermMedia) {
		return
	}
	root, err := instance.DiscoverMediaDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		writeError(w, http.StatusNotFound, "media directory does not exist on disk")
		return
	}

	q := r.URL.Query().Get("q")
	rows, tb, nf, err := mediadisk.List(root, q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	patPath := filepath.Join(filepath.Dir(root), "pserver.pat")
	patScanErr := patgrep.AnnotateMediaRows(patPath, rows)

	out := map[string]any{
		"media_dir":        root,
		"pat_path":         patPath,
		"files":            rows,
		"total_bytes":      tb,
		"total_file_count": nf,
		"quota_exceeded":   false,
		"refs_note":        "Room and Door columns show the first match from a parse of pserver.pat (room background, picture layer, or hotspot pict), not live server state.",
	}
	if patScanErr != "" {
		out["pat_scan_error"] = patScanErr
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePalaceMediaDownload(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermMedia) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("name"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	root, err := instance.DiscoverMediaDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	full, err := mediadisk.ResolveSafe(root, q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	fi, err := os.Stat(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if fi.IsDir() {
		writeError(w, http.StatusBadRequest, "cannot download folder")
		return
	}
	ct := mime.TypeByExtension(filepath.Ext(full))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	base := filepath.Base(full)
	safe := strings.ReplaceAll(base, `"`, "'")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safe))
	http.ServeFile(w, r, full)
}

func (s *Server) handlePalaceMediaRename(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermMedia) {
		return
	}
	var body struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	root, err := instance.DiscoverMediaDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	fromWeb := filepath.ToSlash(strings.TrimSpace(body.From))
	oldFull, err := mediadisk.ResolveSafe(root, fromWeb)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source path")
		return
	}
	ofi, err := os.Stat(oldFull)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	if ofi.IsDir() {
		writeError(w, http.StatusBadRequest, "cannot rename folder")
		return
	}

	newRel, err := mediadisk.NormalizeRenameDestination(fromWeb, body.To)
	if err != nil {
		msg := err.Error()
		if errors.Is(err, mediadisk.ErrUnsafePath) {
			msg = "invalid destination"
		}
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	newFull, err := mediadisk.ResolveSafe(root, newRel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination path")
		return
	}
	if _, err := os.Stat(newFull); err == nil {
		writeError(w, http.StatusBadRequest, "a file already exists at that path")
		return
	}
	if err := os.MkdirAll(filepath.Dir(newFull), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create destination folder")
		return
	}
	if err := os.Rename(oldFull, newFull); err != nil {
		writeError(w, http.StatusInternalServerError, "rename failed")
		return
	}
	s.writeAudit(r.Context(), "palace.media.rename", palaceName, map[string]string{"from": fromWeb, "to": filepath.ToSlash(newRel)})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"name": filepath.ToSlash(newRel),
	})
}

func (s *Server) handlePalaceMediaDelete(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermMedia) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("name"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	root, err := instance.DiscoverMediaDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	full, err := mediadisk.ResolveSafe(root, q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	fi, err := os.Stat(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if fi.IsDir() {
		writeError(w, http.StatusBadRequest, "cannot delete folder")
		return
	}
	if err := os.Remove(full); err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	s.writeAudit(r.Context(), "palace.media.delete", palaceName, map[string]string{"name": q})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handlePalaceMediaUpload(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermMedia) {
		return
	}

	root, err := instance.DiscoverMediaDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := r.ParseMultipartForm(mediadisk.MaxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart")
		return
	}
	fhs := r.MultipartForm.File["file"]
	if len(fhs) == 0 {
		writeError(w, http.StatusBadRequest, "no file field")
		return
	}

	var oldSum, newSum int64
	for _, fh := range fhs {
		base := filepath.Base(fh.Filename)
		if base == "" || base == "." || base == string(filepath.Separator) {
			continue
		}
		dest, err := mediadisk.ResolveSafe(root, base)
		if err != nil {
			continue
		}
		ns := fh.Size
		if ns <= 0 {
			ns = mediadisk.MaxUploadBytes
		}
		newSum += ns
		oldSum += fileSizeOrZero(dest)
	}
	if err := s.quotaRejectAfterChange(palaceName, oldSum, newSum); err != nil {
		writeError(w, http.StatusInsufficientStorage, err.Error())
		return
	}

	var saved []string
	for _, fh := range fhs {
		base := filepath.Base(fh.Filename)
		if base == "" || base == "." || base == string(filepath.Separator) {
			continue
		}
		dest, err := mediadisk.ResolveSafe(root, base)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid filename")
			return
		}
		src, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "open upload failed")
			return
		}
		err = mediadisk.SaveUploaded(dest, io.LimitReader(src, mediadisk.MaxUploadBytes))
		_ = src.Close()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "write failed")
			return
		}
		saved = append(saved, base)
	}
	if len(saved) == 0 {
		writeError(w, http.StatusBadRequest, "no files saved")
		return
	}
	s.writeAudit(r.Context(), "palace.media.upload", palaceName, map[string]string{"files": strings.Join(saved, ",")})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "saved": saved})
}
