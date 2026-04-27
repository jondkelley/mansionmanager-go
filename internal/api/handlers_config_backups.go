package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"palace-manager/internal/authstore"
)

// Config backup filenames: <basename>-mm-dd-yy.bak (UTC date tag).
const configBackupDateTagLayout = "01-02-06"

// configBackupMaxKeep is the number of dated snapshots to retain per file (pserver.pat, pserver.prefs, serverprefs.json).
const configBackupMaxKeep = 30

var reConfigBackupFile = regexp.MustCompile(`^(pserver\.pat|pserver\.prefs|serverprefs\.json)-(\d{2}-\d{2}-\d{2})\.bak$`)

type configBackupListItem struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	ModTime  string `json:"modTime,omitempty"`
	DateTag  string `json:"dateTag"`
}

type configBackupKindGroup struct {
	ID    string                 `json:"id"`
	Label string                 `json:"label"`
	Items []configBackupListItem `json:"items"`
}

func configBackupDir(dataDir string) string {
	return filepath.Join(dataDir, "backups")
}

func configBackupDestName(base string, t time.Time) string {
	tag := t.UTC().Format(configBackupDateTagLayout)
	return fmt.Sprintf("%s-%s.bak", base, tag)
}

func parseConfigBackupDateTag(tag string) time.Time {
	t, err := time.ParseInLocation(configBackupDateTagLayout, tag, time.UTC)
	if err != nil {
		return time.Time{}
	}
	return t
}

func kindForBaseName(base string) string {
	switch base {
	case "pserver.pat":
		return "pat"
	case "pserver.prefs":
		return "prefs"
	case "serverprefs.json":
		return "serverprefs"
	default:
		return ""
	}
}

func (s *Server) handlePalaceConfigBackupsList(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermBackups) {
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	bdir := configBackupDir(dir)
	entries, err := os.ReadDir(bdir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{
				"backupDir": bdir,
				"kinds":     emptyConfigBackupKinds(),
			})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	byKind := map[string][]configBackupListItem{
		"pat":         {},
		"prefs":       {},
		"serverprefs": {},
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		m := reConfigBackupFile.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		base, dateTag := m[1], m[2]
		k := kindForBaseName(base)
		if k == "" {
			continue
		}
		fi, err := ent.Info()
		if err != nil {
			continue
		}
		mt := ""
		if !fi.ModTime().IsZero() {
			mt = fi.ModTime().UTC().Format(time.RFC3339)
		}
		byKind[k] = append(byKind[k], configBackupListItem{
			Filename: name,
			Size:     fi.Size(),
			ModTime:  mt,
			DateTag:  dateTag,
		})
	}
	for _, k := range []string{"pat", "prefs", "serverprefs"} {
		sort.Slice(byKind[k], func(i, j int) bool {
			ti := parseConfigBackupDateTag(byKind[k][i].DateTag)
			tj := parseConfigBackupDateTag(byKind[k][j].DateTag)
			if !ti.Equal(tj) {
				return ti.After(tj)
			}
			return byKind[k][i].Filename > byKind[k][j].Filename
		})
	}
	kinds := []configBackupKindGroup{
		{ID: "pat", Label: "pserver.pat", Items: byKind["pat"]},
		{ID: "prefs", Label: "pserver.prefs", Items: byKind["prefs"]},
		{ID: "serverprefs", Label: "serverprefs.json", Items: byKind["serverprefs"]},
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"backupDir": bdir,
		"kinds":     kinds,
	})
}

func emptyConfigBackupKinds() []configBackupKindGroup {
	return []configBackupKindGroup{
		{ID: "pat", Label: "pserver.pat", Items: []configBackupListItem{}},
		{ID: "prefs", Label: "pserver.prefs", Items: []configBackupListItem{}},
		{ID: "serverprefs", Label: "serverprefs.json", Items: []configBackupListItem{}},
	}
}

func (s *Server) handlePalaceConfigBackupsSnapshot(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermBackups) {
		return
	}
	s.configBackupMu.Lock()
	defer s.configBackupMu.Unlock()
	created, err := s.doPalaceConfigBackup(palaceName, time.Now().UTC())
	if err != nil {
		if errors.Is(err, errPalaceQuotaExceeded) {
			writeError(w, http.StatusInsufficientStorage, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeAudit(r.Context(), "palace.config_backup.snapshot", palaceName, map[string]string{"files": strings.Join(created, ",")})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": palaceName, "created": created})
}

// doPalaceConfigBackup copies live config files into dataDir/backups with the given UTC date tag.
// Caller must hold s.configBackupMu.
func (s *Server) doPalaceConfigBackup(palaceName string, nowUTC time.Time) ([]string, error) {
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		return nil, err
	}
	if err := s.assertConfigSnapshotQuotaHeadroom(palaceName, dir, nowUTC); err != nil {
		return nil, err
	}
	lu := s.palaceLinuxUser(palaceName)
	bdir := configBackupDir(dir)
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		return nil, err
	}
	_ = chownPath(bdir, lu)

	bases := []string{"pserver.pat", "pserver.prefs", "serverprefs.json"}
	var created []string
	for _, base := range bases {
		src := filepath.Join(dir, base)
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return created, err
		}
		dst := filepath.Join(bdir, configBackupDestName(base, nowUTC))
		if err := copyFileToAs(src, dst, lu); err != nil {
			return created, fmt.Errorf("%s: %w", base, err)
		}
		created = append(created, filepath.Base(dst))
	}
	if err := pruneExcessConfigBackups(bdir); err != nil {
		log.Printf("prune config backups [%s]: %v", palaceName, err)
	}
	return created, nil
}

// pruneExcessConfigBackups removes older dated .bak files so at most configBackupMaxKeep remain per base name.
func pruneExcessConfigBackups(bdir string) error {
	entries, err := os.ReadDir(bdir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	byBase := map[string][]string{}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		m := reConfigBackupFile.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		base := m[1]
		byBase[base] = append(byBase[base], name)
	}
	for _, files := range byBase {
		if len(files) <= configBackupMaxKeep {
			continue
		}
		sort.Slice(files, func(i, j int) bool {
			mi := reConfigBackupFile.FindStringSubmatch(files[i])
			mj := reConfigBackupFile.FindStringSubmatch(files[j])
			if len(mi) < 3 || len(mj) < 3 {
				return files[i] > files[j]
			}
			ti := parseConfigBackupDateTag(mi[2])
			tj := parseConfigBackupDateTag(mj[2])
			if !ti.Equal(tj) {
				return ti.After(tj)
			}
			return files[i] > files[j]
		})
		for i := configBackupMaxKeep; i < len(files); i++ {
			p := filepath.Join(bdir, files[i])
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				log.Printf("prune config backup remove %s: %v", p, err)
			}
		}
	}
	return nil
}

func copyFileToAs(src, dst, linuxUser string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeFileAtomicAs(dst, linuxUser, f)
}

func (s *Server) handlePalaceConfigBackupsRestore(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requirePalacePerm(w, r, palaceName, authstore.PermBackups) {
		return
	}
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	fn := strings.TrimSpace(req.Filename)
	if fn != filepath.Base(fn) || strings.Contains(fn, "..") {
		writeError(w, http.StatusBadRequest, "invalid filename")
		return
	}
	m := reConfigBackupFile.FindStringSubmatch(fn)
	if m == nil {
		writeError(w, http.StatusBadRequest, "not a recognized backup file")
		return
	}
	base := m[1]

	s.configBackupMu.Lock()
	defer s.configBackupMu.Unlock()

	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	bakPath := filepath.Join(configBackupDir(dir), fn)
	bakPath = filepath.Clean(bakPath)
	if !strings.HasPrefix(bakPath, filepath.Clean(configBackupDir(dir))) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	if _, err := os.Stat(bakPath); err != nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}

	dest := filepath.Join(dir, base)
	lu := s.palaceLinuxUser(palaceName)

	bakFi, err := os.Stat(bakPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !bakFi.Mode().IsRegular() {
		writeError(w, http.StatusBadRequest, "invalid backup file")
		return
	}
	if err := s.quotaRejectAfterChange(palaceName, fileSizeOrZero(dest), bakFi.Size()); err != nil {
		writeError(w, http.StatusInsufficientStorage, err.Error())
		return
	}

	if err := s.instances.Stop(palaceName); err != nil {
		writeError(w, http.StatusInternalServerError, "stop failed: "+err.Error())
		return
	}
	bakFile, err := os.Open(bakPath)
	if err != nil {
		_ = s.instances.Start(palaceName)
		writeError(w, http.StatusInternalServerError, "open backup: "+err.Error())
		return
	}
	defer bakFile.Close()
	if err := writeFileAtomicAs(dest, lu, bakFile); err != nil {
		_ = s.instances.Start(palaceName)
		writeError(w, http.StatusInternalServerError, "restore failed: "+err.Error())
		return
	}
	if base == "pserver.pat" || base == "pserver.prefs" {
		if err := s.ensurePalaceYPInPrefs(palaceName); err != nil {
			log.Printf("config-backup restore YP merge [%s]: %v", palaceName, err)
		}
	}
	if err := s.instances.Start(palaceName); err != nil {
		writeError(w, http.StatusInternalServerError, "restored file but start failed: "+err.Error())
		return
	}
	s.writeAudit(r.Context(), "palace.config_backup.restore", palaceName, map[string]string{"file": base, "from": fn})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": palaceName, "file": base, "restoredFrom": fn})
}

func (s *Server) runScheduledConfigBackupsForAllPalaces(nowUTC time.Time) {
	s.configBackupMu.Lock()
	defer s.configBackupMu.Unlock()
	for _, p := range s.reg.All() {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		_, err := s.palaceDataDir(name)
		if err != nil {
			continue
		}
		if _, err := s.doPalaceConfigBackup(name, nowUTC); err != nil {
			if errors.Is(err, errPalaceQuotaExceeded) {
				log.Printf("scheduled config backup [%s]: skipped (home over quota)", name)
				continue
			}
			log.Printf("scheduled config backup [%s]: %v", name, err)
		}
	}
}

func durationUntilNextMidnightUTC() time.Duration {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return time.Until(next)
}

func (s *Server) midnightUTCConfigBackupLoop(ctx context.Context) {
	timer := time.NewTimer(durationUntilNextMidnightUTC())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.runScheduledConfigBackupsForAllPalaces(time.Now().UTC())
			timer.Reset(durationUntilNextMidnightUTC())
		}
	}
}
