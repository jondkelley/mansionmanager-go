package versionstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"palace-manager/internal/config"
	"palace-manager/internal/registry"
)

const indexFileName = "versions.json"

// Entry is one archived pserver build (copied from the template after download).
type Entry struct {
	Semver     string `json:"semver"`
	Tag        string `json:"tag,omitempty"`
	BuiltUTC   string `json:"builtUtc,omitempty"`
	Target     string `json:"target,omitempty"`
	ReleasedBy string `json:"releasedBy,omitempty"`
	ArchivedAt string `json:"archivedAt"`
	BinaryPath string `json:"binaryPath"`
}

type indexDoc struct {
	Versions []Entry `json:"versions"`
}

// TemplateInfo mirrors version.txt (live read from template dir).
type TemplateInfo struct {
	Semver     string `json:"semver,omitempty"`
	Tag        string `json:"tag,omitempty"`
	BuiltUTC   string `json:"builtUtc,omitempty"`
	Target     string `json:"target,omitempty"`
	ReleasedBy string `json:"releasedBy,omitempty"`
}

// Snapshot is returned by GET /api/binary-versions.
type Snapshot struct {
	InstallPath string        `json:"installPath"`
	VersionsDir string        `json:"versionsDir"`
	Template    *TemplateInfo `json:"template,omitempty"`
	Versions    []Entry       `json:"versions"`
}

type Store struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Store {
	return &Store{cfg: cfg}
}

func (s *Store) indexPath() string {
	return filepath.Join(s.cfg.Pserver.VersionsDir, indexFileName)
}

// ReadTemplateInfo reads version.txt from the current template directory.
func (s *Store) ReadTemplateInfo() (*TemplateInfo, error) {
	p := filepath.Join(s.cfg.Pserver.TemplateDir, "version.txt")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	kv := parseVersionTxt(data)
	t := TemplateInfo{}
	if v := kv["semver"]; v != "" {
		t.Semver = v
	}
	if v := kv["tag"]; v != "" {
		t.Tag = v
	}
	if v := kv["built_utc"]; v != "" {
		t.BuiltUTC = v
	}
	if v := kv["target"]; v != "" {
		t.Target = v
	}
	if v := kv["released_by"]; v != "" {
		t.ReleasedBy = v
	}
	return &t, nil
}

func parseVersionTxt(data []byte) map[string]string {
	out := make(map[string]string)
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(line[:i]))
		val := strings.TrimSpace(line[i+1:])
		out[key] = val
	}
	return out
}

func semverFromKV(kv map[string]string) string {
	if v := strings.TrimSpace(kv["semver"]); v != "" {
		return sanitizeSemverDir(v)
	}
	tag := strings.TrimSpace(kv["tag"])
	tag = strings.TrimPrefix(tag, "v")
	tag = strings.TrimSpace(tag)
	if tag != "" {
		return sanitizeSemverDir(tag)
	}
	return ""
}

func sanitizeSemverDir(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '\x00':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func (s *Store) ensureDir() error {
	return os.MkdirAll(s.cfg.Pserver.VersionsDir, 0755)
}

func (s *Store) loadIndex() (*indexDoc, error) {
	data, err := os.ReadFile(s.indexPath())
	if os.IsNotExist(err) {
		return &indexDoc{}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc indexDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *Store) saveIndex(doc *indexDoc) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.indexPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.indexPath())
}

// Snapshot returns archived versions plus live template metadata.
func (s *Store) Snapshot() (*Snapshot, error) {
	doc, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	ti, err := s.ReadTemplateInfo()
	if err != nil {
		return nil, err
	}
	vers := append([]Entry(nil), doc.Versions...)
	sort.Slice(vers, func(i, j int) bool {
		return vers[i].ArchivedAt > vers[j].ArchivedAt
	})
	return &Snapshot{
		InstallPath: s.cfg.Pserver.InstallPath,
		VersionsDir: s.cfg.Pserver.VersionsDir,
		Template:    ti,
		Versions:    vers,
	}, nil
}

// ArchiveFromTemplate copies pserver from the template into versions/<semver>/ and updates the index.
func (s *Store) ArchiveFromTemplate() error {
	kvPath := filepath.Join(s.cfg.Pserver.TemplateDir, "version.txt")
	binSrc := filepath.Join(s.cfg.Pserver.TemplateDir, "pserver")
	kvData, err := os.ReadFile(kvPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("no version.txt in template — skipped archiving")
		}
		return err
	}
	kv := parseVersionTxt(kvData)
	sem := semverFromKV(kv)
	if sem == "" {
		return errors.New("version.txt has no semver/tag — skipped archiving")
	}
	fi, err := os.Stat(binSrc)
	if err != nil {
		return fmt.Errorf("template pserver: %w", err)
	}
	if fi.IsDir() {
		return fmt.Errorf("template pserver path is a directory")
	}

	destDir := filepath.Join(s.cfg.Pserver.VersionsDir, sem)
	destBin := filepath.Join(destDir, "pserver")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	if err := copyFile(binSrc, destBin, 0755); err != nil {
		return err
	}

	doc, err := s.loadIndex()
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	ent := Entry{
		Semver:     sem,
		Tag:        kv["tag"],
		BuiltUTC:   kv["built_utc"],
		Target:     kv["target"],
		ReleasedBy: kv["released_by"],
		ArchivedAt: now,
		BinaryPath: destBin,
	}
	found := false
	for i := range doc.Versions {
		if doc.Versions[i].Semver == sem {
			doc.Versions[i] = ent
			found = true
			break
		}
	}
	if !found {
		doc.Versions = append(doc.Versions, ent)
	}
	return s.saveIndex(doc)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// ResolveBinary returns the filesystem path for the pserver binary for this semver pin.
func (s *Store) ResolveBinary(semver string) (string, error) {
	semver = strings.TrimSpace(semver)
	if semver == "" || strings.EqualFold(semver, "latest") {
		return s.cfg.Pserver.InstallPath, nil
	}
	doc, err := s.loadIndex()
	if err != nil {
		return "", err
	}
	for _, v := range doc.Versions {
		if strings.EqualFold(v.Semver, semver) {
			if v.BinaryPath != "" && fileExists(v.BinaryPath) {
				return v.BinaryPath, nil
			}
			p := filepath.Join(s.cfg.Pserver.VersionsDir, v.Semver, "pserver")
			if fileExists(p) {
				return p, nil
			}
			return "", fmt.Errorf("binary for %s not found on disk", v.Semver)
		}
	}
	return "", fmt.Errorf("unknown archived version %q — run Updates first", semver)
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// PatchUnitExecStart replaces the first argument of ExecStart= with binaryPath.
func PatchUnitExecStart(unitPath, binaryPath string) error {
	data, err := os.ReadFile(unitPath)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	done := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "ExecStart=") && !strings.HasPrefix(t, "ExecStart=-") {
			rest := strings.TrimPrefix(t, "ExecStart=")
			fields := strings.Fields(rest)
			if len(fields) < 1 {
				return fmt.Errorf("bad ExecStart line in %s", unitPath)
			}
			fields[0] = binaryPath
			lines[i] = "ExecStart=" + strings.Join(fields, " ")
			done = true
			break
		}
	}
	if !done {
		return fmt.Errorf("no ExecStart= line in %s", unitPath)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	tmp := unitPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, unitPath)
}

func unitPathForUser(linuxUser string) string {
	return filepath.Join("/etc/systemd/system", fmt.Sprintf("palman-%s.service", linuxUser))
}

func (s *Store) canonicalSemverEntry(pin string) (string, error) {
	doc, err := s.loadIndex()
	if err != nil {
		return "", err
	}
	for _, v := range doc.Versions {
		if strings.EqualFold(v.Semver, pin) {
			return v.Semver, nil
		}
	}
	return "", fmt.Errorf("not found")
}

// ApplyPalaceVersion updates registry, patches systemd, optionally restarts.
func (s *Store) ApplyPalaceVersion(reg *registry.Registry, palaceName, semver string, restart bool) error {
	p, ok := reg.Get(palaceName)
	if !ok {
		return fmt.Errorf("palace %q not in registry", palaceName)
	}
	semver = strings.TrimSpace(semver)
	normalized := semver
	if normalized == "" {
		normalized = "latest"
	}
	binPath, err := s.ResolveBinary(normalized)
	if err != nil {
		return err
	}
	unit := unitPathForUser(p.User)
	if !fileExists(unit) {
		return fmt.Errorf("systemd unit not found: %s", unit)
	}
	if err := PatchUnitExecStart(unit, binPath); err != nil {
		return err
	}
	dr := exec.Command("systemctl", "daemon-reload")
	dr.Stdout = os.Stdout
	dr.Stderr = os.Stderr
	if err := dr.Run(); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}
	storeSem := normalized
	if strings.EqualFold(normalized, "latest") {
		storeSem = ""
	}
	canonical := storeSem
	if canonical != "" {
		if c, err := s.canonicalSemverEntry(canonical); err == nil && c != "" {
			canonical = c
		}
	}
	if err := reg.UpdatePserverVersion(palaceName, canonical); err != nil {
		return err
	}
	if restart {
		u := fmt.Sprintf("palman-%s.service", p.User)
		rs := exec.Command("systemctl", "restart", u)
		rs.Stdout = os.Stdout
		rs.Stderr = os.Stderr
		if err := rs.Run(); err != nil {
			return fmt.Errorf("restart: %w", err)
		}
	}
	return nil
}

// ApplyAllPalaces sets every registry palace to the same version.
func (s *Store) ApplyAllPalaces(reg *registry.Registry, semver string, restart bool) error {
	all := reg.All()
	if len(all) == 0 {
		return errors.New("no palaces in registry")
	}
	var lastErr error
	for _, p := range all {
		if err := s.ApplyPalaceVersion(reg, p.Name, semver, false); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	if restart {
		for _, p := range all {
			u := fmt.Sprintf("palman-%s.service", p.User)
			cmd := exec.Command("systemctl", "restart", u)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}
