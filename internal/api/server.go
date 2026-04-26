package api

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"palace-manager/internal/authstore"
	"palace-manager/internal/bootstrap"
	"palace-manager/internal/config"
	"palace-manager/internal/instance"
	"palace-manager/internal/nginx"
	"palace-manager/internal/provisioner"
	"palace-manager/internal/registry"
	"palace-manager/internal/unregistered"
	"palace-manager/internal/versionstore"
	"palace-manager/web"
)

const pserverAutoUpdateInterval = 2 * time.Hour

// pserverUpdateState tracks the running / last-completed state of pserver binary updates
// (both background auto-updates and manual ones triggered via the UI).
type pserverUpdateState struct {
	mu          sync.Mutex
	running     bool
	startedAt   time.Time
	lastRun     time.Time
	lastVersion string // semver from last successful update
	lastErr     string
}

// Server wires all dependencies into an http.Handler.
type Server struct {
	cfg            *config.Config
	configPath     string
	version        string
	gitHash        string
	instances      *instance.Manager
	prov           *provisioner.Provisioner
	nginx          *nginx.Manager
	boot           *bootstrap.Runner
	reg            *registry.Registry
	vers           *versionstore.Store
	unreg          *unregistered.Store
	authStore      *authstore.Store
	mux            *http.ServeMux
	updateCache    *releaseCache
	pserverUpdate  *pserverUpdateState
	hostPassMu     sync.Mutex
	configBackupMu sync.Mutex
}

func New(
	cfg *config.Config,
	configPath string,
	version string,
	gitHash string,
	instances *instance.Manager,
	prov *provisioner.Provisioner,
	nginxMgr *nginx.Manager,
	bootRunner *bootstrap.Runner,
	reg *registry.Registry,
	vers *versionstore.Store,
	unreg *unregistered.Store,
	authStore *authstore.Store,
) *Server {
	s := &Server{
		cfg:           cfg,
		configPath:    configPath,
		version:       version,
		gitHash:       gitHash,
		instances:     instances,
		prov:          prov,
		nginx:         nginxMgr,
		boot:          bootRunner,
		reg:           reg,
		vers:          vers,
		unreg:         unreg,
		authStore:     authStore,
		mux:           http.NewServeMux(),
		updateCache:   &releaseCache{ttl: 30 * time.Minute},
		pserverUpdate: &pserverUpdateState{},
	}
	s.routes()
	return s
}

// Start launches background tasks (pserver auto-update loop, daily config backups). Call once after New.
func (s *Server) Start(ctx context.Context) {
	go s.pserverAutoUpdateLoop(ctx)
	go s.midnightUTCConfigBackupLoop(ctx)
}

// pserverAutoUpdateLoop downloads the latest pserver binary every 2 hours silently.
// Palaces that are set to "latest" automatically benefit on their next restart.
func (s *Server) pserverAutoUpdateLoop(ctx context.Context) {
	ticker := time.NewTicker(pserverAutoUpdateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runPserverUpdateJob(ctx)
		}
	}
}

// runPserverUpdateJob runs the pserver update script in the background and archives
// the new binary. It is a no-op if an update is already in progress.
func (s *Server) runPserverUpdateJob(ctx context.Context) {
	st := s.pserverUpdate
	st.mu.Lock()
	if st.running {
		st.mu.Unlock()
		return
	}
	st.running = true
	st.startedAt = time.Now()
	st.mu.Unlock()

	defer func() {
		st.mu.Lock()
		st.running = false
		st.lastRun = time.Now()
		st.mu.Unlock()
	}()

	if _, err := s.prov.Update(false, io.Discard); err != nil {
		st.mu.Lock()
		st.lastErr = err.Error()
		st.mu.Unlock()
		log.Printf("pserver auto-update: %v", err)
		return
	}

	if err := s.vers.ArchiveFromTemplate(); err != nil {
		log.Printf("pserver auto-update archive: %v", err)
	}

	ti, _ := s.vers.ReadTemplateInfo()
	st.mu.Lock()
	st.lastErr = ""
	if ti != nil {
		if ti.Semver != "" {
			st.lastVersion = ti.Semver
		} else if ti.Tag != "" {
			st.lastVersion = ti.Tag
		}
	}
	st.mu.Unlock()

	log.Printf("pserver auto-update completed: %s", st.lastVersion)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.cfg.Manager.Host, s.cfg.Manager.Port)
}

func (s *Server) routes() {
	auth := s.authMiddleware

	s.mux.Handle("/api/ui/config", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handleUIConfig(w, r)
	}))

	// Static web UI — no auth so the login form is always reachable.
	sub, _ := fs.Sub(web.FS, "public")
	fileServer := http.FileServer(http.FS(sub))
	s.mux.Handle("/", fileServer)

	// Session & users
	s.mux.Handle("/api/session", auth(http.HandlerFunc(s.routeSession)))
	s.mux.Handle("/api/session/password", auth(http.HandlerFunc(s.handleSessionPassword)))
	s.mux.Handle("/api/users", auth(http.HandlerFunc(s.routeUsers)))
	s.mux.Handle("/api/users/", auth(http.HandlerFunc(s.routeUserByName)))
	s.mux.Handle("/api/wizpasses", auth(http.HandlerFunc(s.routeWizPasses)))

	// Palace instances
	s.mux.Handle("/api/palaces", auth(http.HandlerFunc(s.routePalaces)))
	s.mux.Handle("/api/palaces/", auth(http.HandlerFunc(s.routePalaceByName)))

	// Binary update
	s.mux.Handle("/api/update", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleUpdate(w, r)
	})))

	s.mux.Handle("/api/binary-versions", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleBinaryVersions(w, r)
	})))

	s.mux.Handle("/api/rollout", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleRolloutAll(w, r)
	})))

	// Nginx
	s.mux.Handle("/api/nginx/status", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleNginxStatus(w, r)
	})))
	s.mux.Handle("/api/nginx/regen", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleNginxRegen(w, r)
	})))
	s.mux.Handle("/api/nginx/settings", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodPut:
			s.handleNginxSettings(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	s.mux.Handle("/api/nginx/dns-check", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleNginxDNSCheck(w, r)
	})))

	// Bootstrap
	s.mux.Handle("/api/bootstrap/status", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleBootstrapStatus(w, r)
	})))
	s.mux.Handle("/api/bootstrap/run", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleBootstrapRun(w, r)
	})))

	s.mux.Handle("/api/host/logrotate-enable-all", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleHostLogrotateEnableAll(w, r)
	})))

	// Manager self-update
	s.mux.Handle("/api/manager/version", auth(http.HandlerFunc(s.handleManagerVersion)))
	s.mux.Handle("/api/manager/update", auth(http.HandlerFunc(s.handleManagerSelfUpdate)))

	// pserver update status
	s.mux.Handle("/api/pserver/update-status", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handlePserverUpdateStatus(w, r)
	})))

	// pserver self-service endpoints — no Basic Auth; authenticated via servhash.txt.
	// These allow pserver instances running on this machine to trigger upgrades and
	// rollbacks in-game without needing to expose manager credentials to the palace process.
	s.mux.Handle("/api/pserver/version-check", http.HandlerFunc(s.handlePserverVersionCheck))
	s.mux.Handle("/api/pserver/upgrade", http.HandlerFunc(s.handlePserverSelfUpgrade))
	s.mux.Handle("/api/pserver/rollback", http.HandlerFunc(s.handlePserverSelfRollback))
}

func (s *Server) routePalaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListPalaces(w, r)
	case http.MethodPost:
		s.handleProvision(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) routePalaceByName(w http.ResponseWriter, r *http.Request) {
	// /api/palaces/<name>[/<action>]
	rest := strings.TrimPrefix(r.URL.Path, "/api/palaces/")
	parts := strings.SplitN(rest, "/", 2)
	rawName := parts[0]
	name, err := url.PathUnescape(rawName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid palace name")
		return
	}
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}

	if name == "" {
		writeError(w, http.StatusBadRequest, "palace name required")
		return
	}

	switch {
	case action == "" && r.Method == http.MethodGet:
		s.handleGetPalace(w, r, name)
	case action == "" && r.Method == http.MethodPut:
		s.handlePalaceAdminUpdate(w, r, name)
	case action == "" && r.Method == http.MethodDelete:
		s.handleDeletePalace(w, r, name)
	case action == "logs" && r.Method == http.MethodGet:
		s.handleLogs(w, r, name)
	case action == "chat-logs" && r.Method == http.MethodGet:
		s.handleChatLogs(w, r, name)
	case action == "discover" && r.Method == http.MethodGet:
		s.handleDiscoverPalace(w, r, name)
	case action == "register" && r.Method == http.MethodPost:
		s.handleRegisterPalace(w, r, name)
	case (action == "start" || action == "stop" || action == "restart") && r.Method == http.MethodPost:
		s.handlePalaceAction(w, r, name, action)
	case action == "prefs-form" && r.Method == http.MethodGet:
		s.handlePalacePrefsForm(w, r, name)
	// Guided serverprefs.json editor (keys merged per internal/serverprefsform; includes wiz_authoring / wiz_authoring_annotation).
	case action == "serverprefs-form" && r.Method == http.MethodGet:
		s.handleServerPrefsFormGet(w, r, name)
	case action == "serverprefs-form" && r.Method == http.MethodPut:
		s.handleServerPrefsFormPut(w, r, name)
	case action == "server-prefs" && r.Method == http.MethodPut:
		s.handlePalaceServerPrefsSave(w, r, name)
	case action == "misc" && r.Method == http.MethodGet:
		s.handlePalaceMiscGet(w, r, name)
	case action == "misc" && r.Method == http.MethodPut:
		s.handlePalaceMiscSave(w, r, name)
	case action == "command-ranks" && r.Method == http.MethodGet:
		s.handleCommandRanksGet(w, r, name)
	case action == "command-ranks" && r.Method == http.MethodPut:
		s.handleCommandRanksPut(w, r, name)
	case action == "reload-config" && r.Method == http.MethodPost:
		s.handlePalaceReloadConfig(w, r, name)
	case action == "ratbot/files" && r.Method == http.MethodGet:
		s.handlePalaceRatbotFilesList(w, r, name)
	case action == "ratbot/file" && r.Method == http.MethodGet:
		s.handlePalaceRatbotFileGet(w, r, name)
	case action == "ratbot/file" && r.Method == http.MethodPut:
		s.handlePalaceRatbotFileSave(w, r, name)
	case action == "pserver-version" && r.Method == http.MethodPost:
		s.handlePalacePserverVersion(w, r, name)
	case action == "media/files" && r.Method == http.MethodGet:
		s.handlePalaceMediaFiles(w, r, name)
	case action == "media/download" && r.Method == http.MethodGet:
		s.handlePalaceMediaDownload(w, r, name)
	case action == "media/rename" && r.Method == http.MethodPost:
		s.handlePalaceMediaRename(w, r, name)
	case action == "media/upload" && r.Method == http.MethodPost:
		s.handlePalaceMediaUpload(w, r, name)
	case action == "media/file" && r.Method == http.MethodDelete:
		s.handlePalaceMediaDelete(w, r, name)
	case action == "server-files" && r.Method == http.MethodGet:
		s.handlePalaceServerRoot(w, r, name)
	case strings.HasPrefix(action, "server-files/"):
		filePart := strings.TrimPrefix(action, "server-files/")
		filePart, err := url.PathUnescape(filePart)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid file path")
			return
		}
		switch r.Method {
		case http.MethodGet:
			s.handlePalaceServerFile(w, r, name, filePart)
		case http.MethodPut:
			s.handlePalaceServerFileSave(w, r, name, filePart)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case action == "pat-upload" && r.Method == http.MethodPost:
		s.handlePalacePatUpload(w, r, name)
	case action == "home-backup" && r.Method == http.MethodGet:
		s.handlePalaceHomeBackup(w, r, name)
	case action == "config-backups" && r.Method == http.MethodGet:
		s.handlePalaceConfigBackupsList(w, r, name)
	case action == "config-backups/snapshot" && r.Method == http.MethodPost:
		s.handlePalaceConfigBackupsSnapshot(w, r, name)
	case action == "config-backups/restore" && r.Method == http.MethodPost:
		s.handlePalaceConfigBackupsRestore(w, r, name)
	case action == "stats" && r.Method == http.MethodGet:
		s.handlePalaceStats(w, r, name)
	case action == "palace-users" && r.Method == http.MethodGet:
		s.handlePalaceUsers(w, r, name)
	case action == "palace-users/moderate" && r.Method == http.MethodPost:
		s.handlePalaceUsersModerate(w, r, name)
	case action == "banlist" && r.Method == http.MethodGet:
		s.handlePalaceBanlist(w, r, name)
	case action == "banlist/unban" && r.Method == http.MethodPost:
		s.handlePalaceBanlistUnban(w, r, name)
	case action == "props" && r.Method == http.MethodGet:
		s.handlePalaceProps(w, r, name)
	case action == "props/command" && r.Method == http.MethodPost:
		s.handlePalacePropsCommand(w, r, name)
	case action == "pages" && r.Method == http.MethodGet:
		s.handlePalacePages(w, r, name)
	case action == "pages/send" && r.Method == http.MethodPost:
		s.handlePalacePagesSend(w, r, name)
	case action == "pages/gmsg" && r.Method == http.MethodPost:
		s.handlePalacePagesGmsg(w, r, name)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
