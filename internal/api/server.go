package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"palace-manager/internal/bootstrap"
	"palace-manager/internal/config"
	"palace-manager/internal/instance"
	"palace-manager/internal/nginx"
	"palace-manager/internal/provisioner"
	"palace-manager/internal/registry"
	"palace-manager/web"
)

// Server wires all dependencies into an http.Handler.
type Server struct {
	cfg       *config.Config
	instances *instance.Manager
	prov      *provisioner.Provisioner
	nginx     *nginx.Manager
	boot      *bootstrap.Runner
	reg       *registry.Registry
	mux       *http.ServeMux
}

func New(
	cfg *config.Config,
	instances *instance.Manager,
	prov *provisioner.Provisioner,
	nginxMgr *nginx.Manager,
	bootRunner *bootstrap.Runner,
	reg *registry.Registry,
) *Server {
	s := &Server{
		cfg:       cfg,
		instances: instances,
		prov:      prov,
		nginx:     nginxMgr,
		boot:      bootRunner,
		reg:       reg,
		mux:       http.NewServeMux(),
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

	// Static web UI — no auth so the login form is always reachable.
	sub, _ := fs.Sub(web.FS, "public")
	fileServer := http.FileServer(http.FS(sub))
	s.mux.Handle("/", fileServer)

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
	name := parts[0]
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
	case (action == "start" || action == "stop" || action == "restart") && r.Method == http.MethodPost:
		s.handlePalaceAction(w, r, name, action)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
