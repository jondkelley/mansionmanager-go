package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"palace-manager/internal/instance"
	"palace-manager/internal/palacequota"
	"palace-manager/internal/registry"
)

var rePalaceInstanceName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

func validatePalaceInstanceName(name string) error {
	if !rePalaceInstanceName.MatchString(name) {
		return fmt.Errorf("invalid palace name: use lowercase letters, digits, underscore, hyphen (1–63 chars)")
	}
	return nil
}

// handlePalaceAdminUpdate applies registry + systemd changes for an existing palace (admin only).
// PUT /api/palaces/:name — body: { name, tcpPort, httpPort, quotaBytesMax }.
func (s *Server) handlePalaceAdminUpdate(w http.ResponseWriter, r *http.Request, oldName string) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	pal, ok := s.reg.Get(oldName)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", oldName))
		return
	}

	var req struct {
		Name          string `json:"name"`
		TCPPort       int    `json:"tcpPort"`
		HTTPPort      int    `json:"httpPort"`
		QuotaBytesMax int64  `json:"quotaBytesMax"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	newName := strings.TrimSpace(req.Name)
	if newName == "" {
		newName = oldName
	}
	newName = strings.ToLower(newName)
	if err := validatePalaceInstanceName(newName); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TCPPort <= 0 || req.TCPPort > 65535 || req.HTTPPort <= 0 || req.HTTPPort > 65535 {
		writeError(w, http.StatusBadRequest, "tcpPort and httpPort must be between 1 and 65535")
		return
	}

	q := palacequota.NormalizeMax(req.QuotaBytesMax)
	rename := newName != oldName
	portChange := req.TCPPort != pal.TCPPort || req.HTTPPort != pal.HTTPPort
	needSystemd := rename || portChange

	if s.reg.PortInUseExcept(req.TCPPort, req.HTTPPort, oldName) {
		writeError(w, http.StatusConflict, "port already in use by another palace")
		return
	}

	var wasActive, wasEnabled bool
	if needSystemd {
		wasActive = instance.UnitIsActive(oldName)
		wasEnabled = instance.UnitBootEnabled(oldName)
		if err := s.instances.Stop(oldName); err != nil {
			writeError(w, http.StatusInternalServerError, "stop unit: "+err.Error())
			return
		}
		if portChange {
			if err := instance.CheckTCPListenPortsFree(req.TCPPort, req.HTTPPort); err != nil {
				if wasActive {
					_ = s.instances.Start(oldName)
				}
				writeError(w, http.StatusConflict, err.Error())
				return
			}
		}
		if rename {
			if err := s.instances.Disable(oldName, false); err != nil {
				if wasActive {
					_ = s.instances.Start(oldName)
				}
				writeError(w, http.StatusInternalServerError, "disable unit: "+err.Error())
				return
			}
		}
		if portChange {
			if err := instance.PatchUnitListenPorts(instance.UnitPath(oldName), req.TCPPort, req.HTTPPort); err != nil {
				if rename && wasEnabled {
					_ = instance.SystemctlEnable(oldName)
				}
				if wasActive {
					_ = s.instances.Start(oldName)
				}
				writeError(w, http.StatusInternalServerError, "patch unit ports: "+err.Error())
				return
			}
		}
		if rename {
			if err := instance.RenamePalaceUnitFile(oldName, newName); err != nil {
				if wasEnabled {
					_ = instance.SystemctlEnable(oldName)
				}
				if wasActive {
					_ = s.instances.Start(oldName)
				}
				writeError(w, http.StatusInternalServerError, "rename unit file: "+err.Error())
				return
			}
		}
		if err := instance.ReloadDaemon(); err != nil {
			writeError(w, http.StatusInternalServerError, "daemon-reload: "+err.Error())
			return
		}
		if rename && wasEnabled {
			if err := instance.SystemctlEnable(newName); err != nil {
				writeError(w, http.StatusInternalServerError, "enable unit: "+err.Error())
				return
			}
		}
		startAs := oldName
		if rename {
			startAs = newName
		}
		if wasActive {
			if err := s.instances.Start(startAs); err != nil {
				writeError(w, http.StatusInternalServerError, "start unit: "+err.Error())
				return
			}
		}
	}

	updated := registry.Palace{
		Name:           newName,
		User:           pal.User,
		TCPPort:        req.TCPPort,
		HTTPPort:       req.HTTPPort,
		DataDir:        pal.DataDir,
		ProvisionedAt:  pal.ProvisionedAt,
		PserverVersion: pal.PserverVersion,
		YPHost:         pal.YPHost,
		YPPort:         pal.YPPort,
		QuotaBytesMax:  q,
	}
	if err := s.reg.PutPalace(oldName, updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rename && s.authStore != nil {
		if err := s.authStore.RenamePalaceInTenantBindings(oldName, newName); err != nil {
			writeError(w, http.StatusInternalServerError, "users: "+err.Error())
			return
		}
	}
	if err := s.ensurePalaceYPInPrefs(newName); err != nil {
		writeError(w, http.StatusInternalServerError, "prefs: "+err.Error())
		return
	}

	go func() {
		s.nginx.Trigger()
		s.nginx.TriggerDelayed(2 * time.Second)
	}()

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": newName, "previousName": oldName})
}
