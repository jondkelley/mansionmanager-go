package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"palace-manager/internal/api"
	"palace-manager/internal/authstore"
	"palace-manager/internal/bootstrap"
	"palace-manager/internal/config"
	"palace-manager/internal/instance"
	"palace-manager/internal/nginx"
	"palace-manager/internal/provisioner"
	"palace-manager/internal/registry"
	"palace-manager/internal/unregistered"
	"palace-manager/internal/versionstore"
)

// Set at link time for release builds:
//
//	go build -ldflags "-X main.version=1.2.3 -X main.gitHash=abc1234 -X main.defaultGithubRepo=owner/repo" ./cmd/palace-manager
var version = "dev"
var gitHash = ""

// defaultGithubRepo is baked in at build time from ${{ github.repository }} so that
// official releases can check for updates without any manual config.
var defaultGithubRepo = ""

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bootstrap":
			runBootstrap(os.Args[2:])
			return
		case "version", "--version", "-version":
			fmt.Println("palace-manager", version)
			return
		case "help", "--help", "-help":
			printUsage()
			return
		}
	}

	runServe()
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func runServe() {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", envOr("PALACE_MANAGER_CONFIG", "/etc/palace-manager/config.json"), "path to config.json")
	port := fs.Int("port", 0, "override manager HTTP port from config")
	_ = fs.Parse(os.Args[1:])

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *port != 0 {
		cfg.Manager.Port = *port
	}
	// Fall back to the repo baked in at build time so official releases have
	// the self-update feature enabled without any manual config.json edit.
	if cfg.Manager.GithubRepo == "" && defaultGithubRepo != "" {
		cfg.Manager.GithubRepo = defaultGithubRepo
	}

	reg, err := registry.Load(registry.DefaultPath)
	if err != nil {
		log.Fatalf("registry: %v", err)
	}

	unregPath := envOr("PALACE_MANAGER_UNREGISTERED", unregistered.DefaultPath)
	unregStore, err := unregistered.Load(unregPath)
	if err != nil {
		log.Fatalf("unregistered palaces store: %v", err)
	}

	userStore, err := authstore.Load(envOr("PALACE_MANAGER_USERS", authstore.DefaultPath))
	if err != nil {
		log.Fatalf("users: %v", err)
	}
	if err := authstore.EnsureBootstrap(userStore, cfg); err != nil {
		log.Fatalf("users bootstrap: %v", err)
	}

	instMgr := instance.NewManager(reg, unregStore)
	prov := provisioner.New(cfg)
	nginxMgr := nginx.NewManager(&cfg.Nginx)
	bootRunner := bootstrap.NewRunner(cfg)
	vers := versionstore.New(cfg)

	// Write /etc/palace-manager/pserver-client.conf so pserver instances on this
	// machine can discover the manager's API URL without manual configuration.
	writePserverClientConf(cfg)

	srv := api.New(cfg, *configPath, version, gitHash, instMgr, prov, nginxMgr, bootRunner, reg, vers, unregStore, userStore)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go nginxMgr.Start(ctx)
	srv.Start(ctx)

	addr := srv.Addr()
	buildLabel := version
	if gitHash != "" {
		buildLabel += " (" + gitHash + ")"
	}
	log.Printf("palace-manager %s listening on http://%s", buildLabel, addr)

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: srv,
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

func parseBootstrapSteps(s string) []bootstrap.StepID {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]bootstrap.StepID, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, bootstrap.StepID(p))
	}
	return out
}

func runBootstrap(args []string) {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	configPath := fs.String("config", envOr("PALACE_MANAGER_CONFIG", "/etc/palace-manager/config.json"), "config file path")
	mediaHost := fs.String("media-host", "", "media hostname for TLS (e.g. media.example.com)")
	email := fs.String("email", "", "email for Let's Encrypt")
	staging := fs.Bool("staging", false, "use Let's Encrypt staging CA")
	stepsStr := fs.String("steps", "", "comma-separated: deps,dns,cert,dhparam,hook,nginx,config")
	edgeScheme := fs.String("edge-scheme", "", "override nginx edge scheme: https | http | dual (default from config)")
	_ = fs.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	steps := parseBootstrapSteps(*stepsStr)
	opts := bootstrap.Options{
		MediaHost:  *mediaHost,
		Email:      *email,
		Staging:    *staging,
		Steps:      steps,
		ConfigPath: *configPath,
		EdgeScheme: strings.TrimSpace(*edgeScheme),
	}

	runner := bootstrap.NewRunner(cfg)
	ctx := context.Background()
	if err := runner.Run(ctx, opts, os.Stdout); err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
}

// writePserverClientConf writes /etc/palace-manager/pserver-client.conf so that
// pserver instances running on the same host can auto-discover the manager URL.
// The conf file uses simple key=value lines (comments start with #).
// The listen host 0.0.0.0 is remapped to 127.0.0.1 for loopback reachability.
func writePserverClientConf(cfg *config.Config) {
	host := cfg.Manager.Host
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	url := fmt.Sprintf("http://%s:%d", host, cfg.Manager.Port)
	content := fmt.Sprintf("# Auto-generated by palace-manager — do not edit by hand.\nurl=%s\n", url)
	confPath := "/etc/palace-manager/pserver-client.conf"
	if err := os.MkdirAll("/etc/palace-manager", 0755); err != nil {
		log.Printf("warning: could not create /etc/palace-manager: %v", err)
		return
	}
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		log.Printf("warning: could not write %s: %v", confPath, err)
		return
	}
	log.Printf("wrote %s (url=%s)", confPath, url)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `palace-manager %s

Usage:
  palace-manager [flags]              Start the management API server
  palace-manager bootstrap [flags]    Bootstrap host (TLS, nginx, deps)
  palace-manager version              Print version

Serve flags:
  -config PATH    Config file (default /etc/palace-manager/config.json)
                  Override with PALACE_MANAGER_CONFIG
  -port N         Override HTTP port from config
  Users file: PALACE_MANAGER_USERS (default /etc/palace-manager/users.json)
  Unregistered snapshot: PALACE_MANAGER_UNREGISTERED

Bootstrap flags:
  -config PATH       Config file path
  -media-host HOST   Media hostname for TLS cert
  -email ADDR        Required for cert step
  -staging           Use LE staging CA
  -steps LIST        Comma-separated: deps,dns,cert,dhparam,hook,nginx,config
  -edge-scheme MODE  https | http | dual (optional; default from config)

`, version)
}
