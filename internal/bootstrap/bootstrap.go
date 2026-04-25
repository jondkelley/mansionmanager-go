package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"palace-manager/internal/config"
	ngx "palace-manager/internal/nginx"
)

// StepID identifies a bootstrap step.
type StepID string

const (
	StepDeps    StepID = "deps"
	StepDNS     StepID = "dns"
	StepCert    StepID = "cert"
	StepDHParam StepID = "dhparam"
	StepHook    StepID = "hook"
	StepNginx   StepID = "nginx"
	StepConfig  StepID = "config"
)

var AllSteps = []StepID{StepDeps, StepDNS, StepCert, StepDHParam, StepHook, StepNginx, StepConfig}

type StepState string

const (
	StateOK      StepState = "ok"
	StateFailed  StepState = "failed"
	StateSkipped StepState = "skipped"
	StateUnknown StepState = "unknown"
)

type StepStatus struct {
	ID      StepID    `json:"id"`
	State   StepState `json:"state"`
	Message string    `json:"message"`
}

// Options controls the bootstrap run.
type Options struct {
	MediaHost  string   `json:"mediaHost"`
	Email      string   `json:"email"`
	CertDir    string   `json:"certDir"` // override; auto-derived if empty
	Staging    bool     `json:"staging"`
	Steps      []StepID `json:"steps"` // nil = all steps
	ConfigPath string   `json:"configPath"`
	// EdgeScheme mirrors nginx.edgeScheme: https | http | dual. When http, cert/dhparam/hook steps are skipped.
	EdgeScheme string `json:"edgeScheme"`
}

type Runner struct {
	cfg *config.Config
}

func NewRunner(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg}
}

// CheckStatus returns the current state of each bootstrap step without making changes.
func (r *Runner) CheckStatus() []StepStatus {
	mediaHost := r.cfg.Nginx.MediaHost
	certDir := r.cfg.Nginx.CertDir

	edge := r.cfg.Nginx.EdgeScheme
	statuses := []StepStatus{
		{ID: StepDeps, State: checkDeps()},
		{ID: StepDNS, State: checkDNS(mediaHost)},
		{ID: StepCert, State: certStepState(edge, certDir, mediaHost)},
		{ID: StepDHParam, State: dhStepState(edge)},
		{ID: StepHook, State: hookStepState(edge)},
		{ID: StepNginx, State: checkNginxConf()},
		{ID: StepConfig, State: StateUnknown, Message: "configuration state"},
	}
	return statuses
}

// Run executes the bootstrap sequence, writing progress to w as SSE-style lines.
func (r *Runner) Run(ctx context.Context, opts Options, w io.Writer) error {
	steps := opts.Steps
	if len(steps) == 0 {
		steps = AllSteps
	}

	mediaHost := opts.MediaHost
	if mediaHost == "" {
		mediaHost = r.cfg.Nginx.MediaHost
	}

	certDir := opts.CertDir
	if certDir == "" {
		certDir = fmt.Sprintf("/etc/letsencrypt/live/%s", mediaHost)
	}

	effectiveEdge := strings.TrimSpace(opts.EdgeScheme)
	if effectiveEdge == "" {
		effectiveEdge = strings.TrimSpace(r.cfg.Nginx.EdgeScheme)
	}

	emit := func(step StepID, state StepState, msg string) {
		s := StepStatus{ID: step, State: state, Message: msg}
		b, _ := json.Marshal(s)
		fmt.Fprintf(w, "data: %s\n\n", b)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	log := func(msg string) {
		fmt.Fprintf(w, "data: %s\n\n", jsonLog(msg))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	for _, step := range steps {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		switch step {
		case StepDeps:
			log("Installing system dependencies (nginx, certbot, rsync)...")
			if err := installSystemDeps(w); err != nil {
				emit(StepDeps, StateFailed, err.Error())
				return err
			}
			emit(StepDeps, StateOK, "dependencies installed")

		case StepDNS:
			log(fmt.Sprintf("Checking DNS for %s...", mediaHost))
			state, msg := advisoryDNSCheck(mediaHost)
			emit(StepDNS, state, msg)
			// DNS mismatch is advisory only; continue

		case StepCert:
			if strings.EqualFold(effectiveEdge, "http") {
				emit(StepCert, StateSkipped, "edgeScheme is http — skipping Let's Encrypt")
				break
			}
			certFile := filepath.Join(certDir, "fullchain.pem")
			if _, err := os.Stat(certFile); err == nil {
				emit(StepCert, StateSkipped, fmt.Sprintf("certificate already exists at %s", certDir))
				break
			}
			if opts.Email == "" {
				emit(StepCert, StateFailed, "email is required for Let's Encrypt (--email flag)")
				return fmt.Errorf("email required for certbot")
			}
			log(fmt.Sprintf("Obtaining Let's Encrypt certificate for %s...", mediaHost))
			args := []string{
				"certonly", "--nginx",
				"--non-interactive", "--agree-tos",
				"--email", opts.Email,
				"-d", mediaHost,
			}
			if opts.Staging {
				args = append(args, "--staging")
			}
			if err := runCmd(w, "certbot", args...); err != nil {
				emit(StepCert, StateFailed, err.Error())
				return err
			}
			emit(StepCert, StateOK, fmt.Sprintf("certificate issued at %s", certDir))

		case StepDHParam:
			if strings.EqualFold(effectiveEdge, "http") {
				emit(StepDHParam, StateSkipped, "edgeScheme is http — skipping dhparam")
				break
			}
			dhPath := "/etc/letsencrypt/ssl-dhparams.pem"
			if _, err := os.Stat(dhPath); err == nil {
				emit(StepDHParam, StateSkipped, "dhparam already exists")
				break
			}
			log("Generating DH parameters (this may take a minute)...")
			if err := runCmd(w, "openssl", "dhparam", "-out", dhPath, "2048"); err != nil {
				emit(StepDHParam, StateFailed, err.Error())
				return err
			}
			emit(StepDHParam, StateOK, "dhparam generated")

		case StepHook:
			if strings.EqualFold(effectiveEdge, "http") {
				emit(StepHook, StateSkipped, "edgeScheme is http — skipping certbot renewal hook")
				break
			}
			hookPath := "/etc/letsencrypt/renewal-hooks/deploy/nginx-reload.sh"
			const hookContent = "#!/bin/sh\nsystemctl reload nginx\n"
			if existing, err := os.ReadFile(hookPath); err == nil && string(existing) == hookContent {
				emit(StepHook, StateSkipped, "renewal hook already in place")
				break
			}
			if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
				emit(StepHook, StateFailed, err.Error())
				return err
			}
			if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
				emit(StepHook, StateFailed, err.Error())
				return err
			}
			emit(StepHook, StateOK, "certbot renewal hook written")

		case StepNginx:
			log("Running initial nginx media config generation...")
			args := []string{
				"--scan-homes",
				"--match-scheme", r.cfg.Nginx.MatchScheme,
				"--edge-scheme", r.cfg.Nginx.EdgeScheme,
				"--cert-dir", certDir,
				"--media-host", mediaHost,
				"--nginx-conf", ngx.MediaProxySiteConf,
				"--reload",
			}
			if err := runCmd(w, r.cfg.Nginx.GenScript, args...); err != nil {
				// Non-fatal: no palaces yet means no mediaserverurl.txt files
				emit(StepNginx, StateSkipped, fmt.Sprintf("gen-media-nginx.sh: %v (no palaces yet?)", err))
				break
			}
			emit(StepNginx, StateOK, "nginx config generated and reloaded")

		case StepConfig:
			r.cfg.Nginx.CertDir = certDir
			r.cfg.Nginx.MediaHost = mediaHost
			if effectiveEdge != "" {
				r.cfg.Nginx.EdgeScheme = strings.ToLower(effectiveEdge)
			}
			configPath := opts.ConfigPath
			if configPath == "" {
				configPath = "/etc/palace-manager/config.json"
			}
			if err := r.cfg.Save(configPath); err != nil {
				emit(StepConfig, StateFailed, fmt.Sprintf("could not save config: %v", err))
				return err
			}
			emit(StepConfig, StateOK, fmt.Sprintf("config saved to %s", configPath))
		}
	}

	return nil
}

func runCmd(w io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// detectPackageManager returns apt, dnf, or yum (prefer dnf when both exist).
func detectPackageManager() string {
	if _, err := exec.LookPath("dnf"); err == nil {
		return "dnf"
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "apt"
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return "yum"
	}
	return ""
}

func tryInstallEPEL(w io.Writer, pm string) {
	// python3-certbot-nginx is typically in EPEL on RHEL / Alma / Rocky / CentOS Stream.
	var cmd *exec.Cmd
	switch pm {
	case "dnf":
		cmd = exec.Command("dnf", "install", "-y", "epel-release")
	case "yum":
		cmd = exec.Command("yum", "install", "-y", "epel-release")
	default:
		return
	}
	cmd.Stdout = w
	cmd.Stderr = w
	_ = cmd.Run()
}

func installSystemDeps(w io.Writer) error {
	switch detectPackageManager() {
	case "apt":
		if err := runCmd(w, "apt-get", "update"); err != nil {
			return err
		}
		return runCmd(w, "apt-get", "install", "-y",
			"nginx", "certbot", "python3-certbot-nginx", "rsync")
	case "dnf":
		fmt.Fprintf(w, "data: %s\n\n", jsonLog("Attempting epel-release (ignored if already present or unavailable)…"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		tryInstallEPEL(w, "dnf")
		return runCmd(w, "dnf", "install", "-y",
			"nginx", "certbot", "python3-certbot-nginx", "rsync")
	case "yum":
		fmt.Fprintf(w, "data: %s\n\n", jsonLog("Attempting epel-release (ignored if already present or unavailable)…"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		tryInstallEPEL(w, "yum")
		return runCmd(w, "yum", "install", "-y",
			"nginx", "certbot", "python3-certbot-nginx", "rsync")
	default:
		return fmt.Errorf("no supported package manager found (install apt-get, dnf, or yum)")
	}
}

func jsonLog(msg string) string {
	b, _ := json.Marshal(map[string]string{"log": msg})
	return string(b)
}

func checkDeps() StepState {
	for _, bin := range []string{"nginx", "certbot"} {
		if _, err := exec.LookPath(bin); err != nil {
			return StateUnknown
		}
	}
	return StateOK
}

func checkDNS(host string) StepState {
	if _, err := net.LookupHost(host); err != nil {
		return StateUnknown
	}
	return StateOK
}

func advisoryDNSCheck(host string) (StepState, string) {
	addrs, err := net.LookupHost(host)
	if err != nil {
		return StateUnknown, fmt.Sprintf("DNS lookup for %s failed: %v", host, err)
	}

	// Try to get our public IP for comparison
	publicIP, _ := fetchPublicIP()

	if publicIP != "" {
		for _, addr := range addrs {
			if addr == publicIP {
				return StateOK, fmt.Sprintf("%s → %s (matches this server)", host, addr)
			}
		}
		return StateUnknown, fmt.Sprintf(
			"WARNING: %s resolves to %v but this server's public IP appears to be %s — update DNS before certbot",
			host, addrs, publicIP,
		)
	}

	return StateOK, fmt.Sprintf("%s resolves to %v", host, addrs)
}

// DNSCheckResult is returned by CheckDNS.
type DNSCheckResult struct {
	Host       string   `json:"host"`
	Match      bool     `json:"match"`
	ResolvedTo []string `json:"resolvedTo"`
	ServerIP   string   `json:"serverIP,omitempty"`
	Warning    string   `json:"warning,omitempty"`
	LookupErr  string   `json:"lookupError,omitempty"`
}

// CheckDNS resolves host and reports whether any resolved address belongs to
// a local network interface or this machine's public IP.
func CheckDNS(host string) DNSCheckResult {
	addrs, err := net.LookupHost(host)
	if err != nil {
		return DNSCheckResult{Host: host, Match: false, LookupErr: err.Error()}
	}

	addrSet := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		addrSet[a] = true
	}

	// Check every local interface address first (works when the public IP is
	// bound directly, e.g. bare-metal or most VPS providers).
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		ifAddrs, _ := iface.Addrs()
		for _, a := range ifAddrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && addrSet[ip.String()] {
				return DNSCheckResult{Host: host, Match: true, ResolvedTo: addrs}
			}
		}
	}

	// Fallback: fetch public IP (handles NAT / cloud with 1:1 NAT).
	publicIP, _ := fetchPublicIP()
	if publicIP != "" && addrSet[publicIP] {
		return DNSCheckResult{Host: host, Match: true, ResolvedTo: addrs, ServerIP: publicIP}
	}

	return DNSCheckResult{
		Host:       host,
		Match:      false,
		ResolvedTo: addrs,
		ServerIP:   publicIP,
		Warning:    fmt.Sprintf("%s resolves to %v but no address matches this machine", host, addrs),
	}
}

func fetchPublicIP() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var buf strings.Builder
	io.Copy(&buf, resp.Body)
	return strings.TrimSpace(buf.String()), nil
}

func certStepState(edgeScheme, certDir, mediaHost string) StepState {
	if strings.EqualFold(strings.TrimSpace(edgeScheme), "http") {
		return StateSkipped
	}
	return checkCert(certDir, mediaHost)
}

func dhStepState(edgeScheme string) StepState {
	if strings.EqualFold(strings.TrimSpace(edgeScheme), "http") {
		return StateSkipped
	}
	return checkDHParam()
}

func hookStepState(edgeScheme string) StepState {
	if strings.EqualFold(strings.TrimSpace(edgeScheme), "http") {
		return StateSkipped
	}
	return checkRenewalHook()
}

func checkCert(certDir, mediaHost string) StepState {
	if certDir == "" {
		certDir = fmt.Sprintf("/etc/letsencrypt/live/%s", mediaHost)
	}
	if _, err := os.Stat(filepath.Join(certDir, "fullchain.pem")); err == nil {
		return StateOK
	}
	return StateUnknown
}

func checkRenewalHook() StepState {
	hookPath := "/etc/letsencrypt/renewal-hooks/deploy/nginx-reload.sh"
	if _, err := os.Stat(hookPath); err == nil {
		return StateOK
	}
	return StateUnknown
}

func checkDHParam() StepState {
	if _, err := os.Stat("/etc/letsencrypt/ssl-dhparams.pem"); err == nil {
		return StateOK
	}
	return StateUnknown
}

func checkNginxConf() StepState {
	if _, err := os.Stat(ngx.MediaProxySiteConf); err == nil {
		return StateOK
	}
	return StateUnknown
}
