package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// releaseCache holds the most recently fetched GitHub release info so we
// avoid hammering the API on every page load.  TTL is set at construction.
type releaseCache struct {
	mu        sync.Mutex
	ttl       time.Duration
	fetchedAt time.Time
	info      *releaseInfo
}

type releaseInfo struct {
	Tag         string `json:"tag"`
	PublishedAt string `json:"publishedAt"`
	ReleaseURL  string `json:"releaseUrl"`
	// First 400 chars of the release body (markdown), for a quick summary.
	Summary string `json:"summary"`
}

func (c *releaseCache) get() *releaseInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.info != nil && time.Since(c.fetchedAt) < c.ttl {
		return c.info
	}
	return nil
}

func (c *releaseCache) set(info *releaseInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info = info
	c.fetchedAt = time.Now()
}

// handleManagerVersion returns the running version, git hash, and the latest
// published release from GitHub (cached for 30 minutes).
func (s *Server) handleManagerVersion(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ver := normaliseVersion(s.version)

	type versionResp struct {
		Current    string       `json:"current"`
		GitHash    string       `json:"gitHash"`
		GithubRepo string       `json:"githubRepo"`
		Latest     *releaseInfo `json:"latest"`
		UpdateAvailable bool   `json:"updateAvailable"`
	}

	resp := versionResp{
		Current:    ver,
		GitHash:    s.gitHash,
		GithubRepo: s.cfg.Manager.GithubRepo,
	}

	if s.cfg.Manager.GithubRepo != "" {
		// Try the cache first; fetch from GitHub if stale.
		info := s.updateCache.get()
		if info == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			if fetched, err := fetchLatestRelease(ctx, s.cfg.Manager.GithubRepo); err == nil {
				s.updateCache.set(fetched)
				info = fetched
			}
		}
		resp.Latest = info
		if info != nil {
			resp.UpdateAvailable = semverLt(ver, info.Tag)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleManagerSelfUpdate downloads and installs a new palace-manager release
// from GitHub, then restarts the systemd service.  Output is streamed as SSE.
func (s *Server) handleManagerSelfUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg.Manager.GithubRepo == "" {
		writeError(w, http.StatusBadRequest, "self-update requires manager.githubRepo to be set in config.json")
		return
	}

	var body struct {
		Tag string `json:"tag"`
	}
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	sw, ok := sseWriter(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	if err := s.doSelfUpdate(r.Context(), body.Tag, sw); err != nil {
		streamLine(sw, fmt.Sprintf("ERROR: %v", err))
		return
	}
}

func (s *Server) doSelfUpdate(ctx context.Context, requestedTag string, sw io.Writer) error {
	repo := s.cfg.Manager.GithubRepo

	// Resolve tag.
	tag := requestedTag
	if tag == "" || tag == "latest" {
		streamLine(sw, "Fetching latest release from GitHub...")
		fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		info, err := fetchLatestRelease(fetchCtx, repo)
		if err != nil {
			return fmt.Errorf("fetch latest release: %w", err)
		}
		tag = info.Tag
		// Bust the cache so the UI sees the new version after restart.
		s.updateCache.set(info)
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	streamLine(sw, fmt.Sprintf("Target version: %s", tag))

	assetName, err := managerAssetName(tag)
	if err != nil {
		return err
	}

	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, assetName)
	streamLine(sw, fmt.Sprintf("Downloading %s...", downloadURL))

	dlCtx, dlCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer dlCancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "palace-manager-selfupdate/"+s.version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download HTTP %d — check that tag %s has Linux release assets at github.com/%s", resp.StatusCode, tag, repo)
	}

	streamLine(sw, "Extracting binary from tarball...")
	binaryData, err := extractBinaryFromTarball(resp.Body)
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}
	streamLine(sw, fmt.Sprintf("Extracted palace-manager (%d bytes)", len(binaryData)))

	// Resolve install target — current executable's real path.
	targetPath, err := os.Executable()
	if err != nil {
		targetPath = "/usr/local/bin/palace-manager"
	}
	if resolved, err := filepath.EvalSymlinks(targetPath); err == nil {
		targetPath = resolved
	}

	streamLine(sw, fmt.Sprintf("Installing to %s...", targetPath))
	if err := installBinary(binaryData, targetPath); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	streamLine(sw, fmt.Sprintf("Installed %s successfully.", tag))
	streamLine(sw, "Restarting palace-manager service in 3 seconds...")
	streamLine(sw, fmt.Sprintf(`{"ok":true,"restarting":true,"version":%q}`, tag))

	go func() {
		time.Sleep(3 * time.Second)
		_ = exec.Command("systemctl", "restart", "palace-manager").Run()
	}()

	return nil
}

// fetchLatestRelease calls the GitHub releases API and returns structured info.
func fetchLatestRelease(ctx context.Context, repo string) (*releaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "palace-manager-selfupdate")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API HTTP %d for %s", resp.StatusCode, repo)
	}

	var data struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if data.TagName == "" {
		return nil, fmt.Errorf("no tag_name in GitHub response")
	}

	summary := data.Body
	if len(summary) > 400 {
		summary = summary[:400] + "…"
	}

	return &releaseInfo{
		Tag:         data.TagName,
		PublishedAt: data.PublishedAt,
		ReleaseURL:  data.HTMLURL,
		Summary:     summary,
	}, nil
}

// semverLt returns true when a is strictly older than b.
// Both strings may have an optional "v" prefix (e.g. "v1.2.3" or "1.2.3").
// Non-numeric pre-release suffixes are ignored.  "dev" is always considered
// older than any real version so dev builds always show an update notice.
func semverLt(a, b string) bool {
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parseSemver(s string) [3]int {
	s = strings.TrimPrefix(s, "v")
	// Strip pre-release suffixes (e.g. "-rc1", "+meta").
	for _, sep := range []string{"-", "+"} {
		if idx := strings.Index(s, sep); idx >= 0 {
			s = s[:idx]
		}
	}
	parts := strings.SplitN(s, ".", 3)
	var out [3]int
	for i, p := range parts {
		if i >= 3 {
			break
		}
		fmt.Sscanf(p, "%d", &out[i])
	}
	return out
}

// managerAssetName returns the release tarball filename for the current OS/arch.
func managerAssetName(tag string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("self-update is only supported on Linux (running on %s)", runtime.GOOS)
	}
	ver := strings.TrimPrefix(tag, "v")

	archSuffix := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
		"arm":   "armv7",
		"386":   "386",
	}
	suffix, ok := archSuffix[runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("unsupported architecture for self-update: %s", runtime.GOARCH)
	}

	return fmt.Sprintf("palace-manager_%s_linux_%s.tar.gz", ver, suffix), nil
}

// extractBinaryFromTarball reads a gzipped tar and returns the bytes of the
// first regular file entry whose base name is "palace-manager".
func extractBinaryFromTarball(r io.Reader) ([]byte, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) == "palace-manager" {
			const maxSize = 256 << 20
			return io.ReadAll(io.LimitReader(tr, maxSize))
		}
	}
	return nil, fmt.Errorf("palace-manager binary not found inside tarball")
}

// installBinary writes binaryData to a temp file next to targetPath, sets
// 0755, then atomically renames over the target.  The running process keeps
// its old inode; systemd loads the new binary on restart.
func installBinary(binaryData []byte, targetPath string) error {
	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, ".palace-manager-update-")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(binaryData); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func normaliseVersion(v string) string {
	if v != "" && !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	if v == "" {
		return "dev"
	}
	return v
}
