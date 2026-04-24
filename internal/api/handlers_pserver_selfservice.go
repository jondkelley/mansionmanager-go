package api

// handlers_pserver_selfservice.go — unauthenticated (servhash-gated) endpoints that
// allow pserver instances to check for updates, trigger an upgrade, or roll back to
// a previous binary without human interaction via the web UI.
//
// Authentication mechanism: each pserver writes its license SHA-1 to
// servhash.txt next to pserver.pat at startup.  The palace manager reads
// that same file from the palace's DataDir to verify identity.  This ties
// the upgrade/rollback privilege to the server key already installed on
// the machine — an external caller who does not control the palace's data
// directory cannot forge a valid hash.

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"palace-manager/internal/registry"
)

// findPalaceByServHash scans all registered palaces and returns the first one
// whose <DataDir>/servhash.txt matches the supplied hash (trimmed, case-sensitive).
func (s *Server) findPalaceByServHash(hash string) (registry.Palace, bool) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return registry.Palace{}, false
	}
	for _, p := range s.reg.All() {
		if p.DataDir == "" {
			continue
		}
		path := filepath.Join(p.DataDir, "servhash.txt")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == hash {
			return p, true
		}
	}
	return registry.Palace{}, false
}

// handlePserverVersionCheck is GET /api/pserver/version-check?hash=<hash>&version=<current>
// Returns the latest available pserver version and whether it differs from the caller's version.
func (s *Server) handlePserverVersionCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	hash := r.URL.Query().Get("hash")
	palace, ok := s.findPalaceByServHash(hash)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unknown or missing servhash")
		return
	}

	currentVersion := strings.TrimSpace(r.URL.Query().Get("version"))

	ti, _ := s.vers.ReadTemplateInfo()
	latestVer := ""
	if ti != nil {
		if ti.Semver != "" {
			latestVer = ti.Semver
		} else if ti.Tag != "" {
			latestVer = strings.TrimPrefix(ti.Tag, "v")
		}
	}

	hasUpdate := false
	if currentVersion != "" && latestVer != "" {
		hasUpdate = semverLt(currentVersion, latestVer)
	}

	type resp struct {
		PalaceName  string `json:"palaceName"`
		Installed   string `json:"installed"`
		Latest      string `json:"latest"`
		HasUpdate   bool   `json:"hasUpdate"`
	}
	writeJSON(w, http.StatusOK, resp{
		PalaceName: palace.Name,
		Installed:  currentVersion,
		Latest:     latestVer,
		HasUpdate:  hasUpdate,
	})
}

// handlePserverSelfUpgrade is POST /api/pserver/upgrade
// Body: { "hash": "...", "version": "<current-running-version>" }
// Runs the pserver update script, archives the new version, pins the palace to it,
// and schedules a systemd restart.
func (s *Server) handlePserverSelfUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body struct {
		Hash    string `json:"hash"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	palace, ok := s.findPalaceByServHash(body.Hash)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unknown or missing servhash")
		return
	}

	// Prevent concurrent upgrade operations.
	st := s.pserverUpdate
	st.mu.Lock()
	if st.running {
		st.mu.Unlock()
		writeError(w, http.StatusConflict, "a pserver update is already in progress")
		return
	}
	st.running = true
	st.startedAt = time.Now()
	st.mu.Unlock()

	log.Printf("pserver self-upgrade requested by palace %q (hash %s, current %s)", palace.Name, body.Hash, body.Version)

	go func() {
		defer func() {
			st.mu.Lock()
			st.running = false
			st.lastRun = time.Now()
			st.mu.Unlock()
		}()

		// Download and install the latest pserver binary.
		if _, err := s.prov.Update(false, io.Discard); err != nil {
			log.Printf("pserver self-upgrade (palace %q): update failed: %v", palace.Name, err)
			st.mu.Lock()
			st.lastErr = err.Error()
			st.mu.Unlock()
			return
		}

		// Archive the new binary so the version is pinnable.
		if err := s.vers.ArchiveFromTemplate(); err != nil {
			log.Printf("pserver self-upgrade (palace %q): archive failed: %v", palace.Name, err)
		}

		// Read the new semver so we can pin this palace to it.
		ti, _ := s.vers.ReadTemplateInfo()
		newSemver := ""
		if ti != nil {
			if ti.Semver != "" {
				newSemver = ti.Semver
			} else if ti.Tag != "" {
				newSemver = strings.TrimPrefix(ti.Tag, "v")
			}
		}

		st.mu.Lock()
		st.lastErr = ""
		if newSemver != "" {
			st.lastVersion = newSemver
		}
		st.mu.Unlock()

		// Apply the new version to this palace's systemd unit and restart.
		if err := s.vers.ApplyPalaceVersion(s.reg, palace.Name, newSemver, true); err != nil {
			log.Printf("pserver self-upgrade (palace %q): apply failed: %v", palace.Name, err)
			st.mu.Lock()
			st.lastErr = err.Error()
			st.mu.Unlock()
			return
		}

		log.Printf("pserver self-upgrade (palace %q): upgraded to %s, restarted", palace.Name, newSemver)
	}()

	type resp struct {
		OK         bool   `json:"ok"`
		Message    string `json:"message"`
		PalaceName string `json:"palaceName"`
	}
	writeJSON(w, http.StatusAccepted, resp{
		OK:         true,
		Message:    fmt.Sprintf("upgrade initiated for palace %q; server will restart shortly", palace.Name),
		PalaceName: palace.Name,
	})
}

// handlePserverSelfRollback is POST /api/pserver/rollback
// Body: { "hash": "..." }
// Finds the previous archived pserver version and applies it, then restarts.
func (s *Server) handlePserverSelfRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body struct {
		Hash string `json:"hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	palace, ok := s.findPalaceByServHash(body.Hash)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unknown or missing servhash")
		return
	}

	snap, err := s.vers.Snapshot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not read version store: %v", err))
		return
	}

	// Versions are returned sorted by ArchivedAt descending (most recent first).
	versions := snap.Versions
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].ArchivedAt > versions[j].ArchivedAt
	})

	if len(versions) < 2 {
		writeError(w, http.StatusConflict, "no previous version available for rollback (need at least 2 archived versions)")
		return
	}

	currentPin := strings.TrimSpace(palace.PserverVersion)

	// Find the rollback target: the most recent archived version that differs from the current pin.
	rollbackSemver := ""
	if currentPin == "" || strings.EqualFold(currentPin, "latest") {
		// Palace is on "latest" — roll back to the most recent pinned archive.
		rollbackSemver = versions[0].Semver
	} else {
		// Palace is pinned — find the next older version.
		for i, v := range versions {
			if strings.EqualFold(v.Semver, currentPin) {
				if i+1 < len(versions) {
					rollbackSemver = versions[i+1].Semver
				}
				break
			}
		}
		if rollbackSemver == "" {
			// Current pin not found in archive — fall back to oldest available.
			rollbackSemver = versions[len(versions)-1].Semver
		}
	}

	log.Printf("pserver self-rollback requested by palace %q (current=%q, target=%q)", palace.Name, currentPin, rollbackSemver)

	if err := s.vers.ApplyPalaceVersion(s.reg, palace.Name, rollbackSemver, true); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("rollback failed: %v", err))
		return
	}

	log.Printf("pserver self-rollback (palace %q): rolled back to %s, restarted", palace.Name, rollbackSemver)

	type resp struct {
		OK          bool   `json:"ok"`
		Message     string `json:"message"`
		PalaceName  string `json:"palaceName"`
		RolledBackTo string `json:"rolledBackTo"`
	}
	writeJSON(w, http.StatusOK, resp{
		OK:           true,
		Message:      fmt.Sprintf("rolled back to %s; server will restart shortly", rollbackSemver),
		PalaceName:   palace.Name,
		RolledBackTo: rollbackSemver,
	})
}
