package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

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

// Server wires all dependencies into an http.Handler.
type Server struct {
	cfg        *config.Config
	configPath string
	instances  *instance.Manager
	prov       *provisioner.Provisioner
	nginx      *nginx.Manager
	boot       *bootstrap.Runner
	reg        *registry.Registry
	vers       *versionstore.Store
	unreg      *unregistered.Store
	authStore  *authstore.Store
	mux        *http.ServeMux
}

func New(
	cfg *config.Config,
	configPath string,
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
		cfg:        cfg,
		configPath: configPath,
		instances:  instances,
		prov:       prov,
		nginx:      nginxMgr,
		boot:       bootRunner,
		reg:        reg,
		vers:       vers,
		unreg:      unreg,
		authStore:  authStore,
		mux:        http.NewServeMux(),
	}
	s.routes()
	return s
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
	case action == "" && r.Method == http.MethodDelete:
		s.handleDeletePalace(w, r, name)
	case action == "logs" && r.Method == http.MethodGet:
		s.handleLogs(w, r, name)
	case action == "discover" && r.Method == http.MethodGet:
		s.handleDiscoverPalace(w, r, name)
	case action == "register" && r.Method == http.MethodPost:
		s.handleRegisterPalace(w, r, name)
	case (action == "start" || action == "stop" || action == "restart") && r.Method == http.MethodPost:
		s.handlePalaceAction(w, r, name, action)
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
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
