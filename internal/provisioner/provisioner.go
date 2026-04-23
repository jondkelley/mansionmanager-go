package provisioner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"palace-manager/internal/config"
	"palace-manager/internal/registry"
)

type ProvisionResult struct {
	OK       bool   `json:"ok"`
	User     string `json:"user"`
	TCPPort  int    `json:"tcpPort"`
	HTTPPort int    `json:"httpPort"`
	DataDir  string `json:"dataDir"`
}

type UpdateResult struct {
	OK bool `json:"ok"`
}

// LogrotateResult is emitted as JSON by provision-palace.sh --logrotate-only.
type LogrotateResult struct {
	OK            bool   `json:"ok"`
	User          string `json:"user"`
	LogrotatePath string `json:"logrotatePath"`
	LogPath       string `json:"logPath"`
	SystemdUnit   string `json:"systemdUnit"`
}

type Provisioner struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Provisioner {
	return &Provisioner{cfg: cfg}
}

// Provision runs provision-palace.sh and streams output to w.
// It returns the parsed result once the script exits.
func (p *Provisioner) Provision(name string, tcpPort, httpPort int, w io.Writer) (*ProvisionResult, error) {
	args := []string{
		"--user", name,
		"--tcp-port", fmt.Sprintf("%d", tcpPort),
		"--http-port", fmt.Sprintf("%d", httpPort),
		"--from-template",
		"--no-cron", // manager owns the nginx regen loop
		"--json",
	}

	env := append(os.Environ(),
		"PALACE_TEMPLATE_DIR="+p.cfg.Pserver.TemplateDir,
		"PSERVER_BIN="+p.cfg.Pserver.InstallPath,
		"PALACE_REVERSE_PROXY_MEDIA="+config.ReverseProxyMediaBase(p.cfg.Nginx.EdgeScheme, p.cfg.Nginx.MediaHost),
	)

	return runScript(p.cfg.Scripts.Provision, args, env, w, func(line string) (*ProvisionResult, bool) {
		var r ProvisionResult
		if strings.HasPrefix(line, "{") && json.Unmarshal([]byte(line), &r) == nil && r.OK {
			if r.DataDir == "" {
				r.DataDir = fmt.Sprintf("/home/%s/palace", name)
			}
			return &r, true
		}
		return nil, false
	})
}

// Update runs update-pserver.sh and streams output to w.
func (p *Provisioner) Update(restartAll bool, w io.Writer) (*UpdateResult, error) {
	args := []string{"--json"}
	if restartAll {
		args = append(args, "--restartall")
	}

	env := append(os.Environ(),
		"PALACE_TEMPLATE_DIR="+p.cfg.Pserver.TemplateDir,
		"PSERVER_INSTALL_PATH="+p.cfg.Pserver.InstallPath,
		"PALACE_SDIST_URL="+p.cfg.Pserver.SdistURL,
	)

	res, err := runScript(p.cfg.Scripts.Update, args, env, w, func(line string) (*UpdateResult, bool) {
		var r UpdateResult
		if strings.HasPrefix(line, "{") && json.Unmarshal([]byte(line), &r) == nil {
			return &r, true
		}
		return nil, false
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return &UpdateResult{OK: true}, nil
	}
	return res, nil
}

// EnsureLogrotate writes /etc/logrotate.d/palace-<user> for an existing palace (same rules as full provision).
func (p *Provisioner) EnsureLogrotate(linuxUser, dataDir, systemdUnit string, w io.Writer) (*LogrotateResult, error) {
	args := []string{
		"--logrotate-only",
		"--user", linuxUser,
		"--json",
	}
	if dataDir != "" {
		args = append(args, "--data-dir", dataDir)
	}
	if systemdUnit != "" {
		args = append(args, "--systemd-unit", systemdUnit)
	}
	return runScript(p.cfg.Scripts.Provision, args, os.Environ(), w, func(line string) (*LogrotateResult, bool) {
		var r LogrotateResult
		if strings.HasPrefix(line, "{") && json.Unmarshal([]byte(line), &r) == nil && r.OK {
			return &r, true
		}
		return nil, false
	})
}

// PurgeUser removes the Linux user and their home directory.
func (p *Provisioner) PurgeUser(name string) error {
	cmd := exec.Command("deluser", "--remove-home", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runScript runs a shell script, tees output to w, and extracts a typed result
// from any line matched by the extract func.
func runScript[T any](script string, args, env []string, w io.Writer, extract func(string) (*T, bool)) (*T, error) {
	cmd := exec.Command(script, args...)
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// Merge stderr into stdout so all script output is line-scanned and framed as SSE.
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", script, err)
	}

	var result *T
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		// Same framing as api.streamLine — raw Fprintln breaks the UI SSE parser when mixed
		// with handler-emitted events (multiple JSON objects end up in one chunk, JSON.parse fails).
		fmt.Fprintf(w, "data: %s\n\n", strings.TrimRight(line, "\r\n"))
		if r, ok := extract(line); ok {
			result = r
		}
	}

	if err := cmd.Wait(); err != nil {
		return result, fmt.Errorf("script %s: %w", script, err)
	}
	return result, nil
}

// RegistryEntry builds a registry.Palace from a ProvisionResult.
func RegistryEntry(name string, r *ProvisionResult) registry.Palace {
	return registry.Palace{
		Name:     name,
		User:     r.User,
		TCPPort:  r.TCPPort,
		HTTPPort: r.HTTPPort,
		DataDir:  r.DataDir,
	}
}
