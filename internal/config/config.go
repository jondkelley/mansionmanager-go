package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"
)

type Manager struct {
	Port     int    `json:"port"`
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	// Theme selects the web UI palette: "basic" (light, default) or "metal" (original brushed dark chrome).
	Theme string `json:"theme"`
}

type Scripts struct {
	Provision string `json:"provision"`
	Update    string `json:"update"`
}

type Pserver struct {
	TemplateDir string `json:"templateDir"`
	InstallPath string `json:"installPath"`
	SdistURL    string `json:"sdistUrl"`
	// VersionsDir stores archived pserver builds (from version.txt after each update) and versions.json.
	VersionsDir string `json:"versionsDir"`
}

type Nginx struct {
	GenScript     string        `json:"genScript"`
	RegenInterval time.Duration `json:"regenInterval"`
	MediaHost     string        `json:"mediaHost"`
	CertDir       string        `json:"certDir"`
	EdgeScheme    string        `json:"edgeScheme"`
	MatchScheme   string        `json:"matchScheme"`
}

type Config struct {
	Manager Manager `json:"manager"`
	Scripts Scripts `json:"scripts"`
	Pserver Pserver `json:"pserver"`
	Nginx   Nginx   `json:"nginx"`
}

type rawNginx struct {
	GenScript     string `json:"genScript"`
	RegenInterval string `json:"regenInterval"`
	MediaHost     string `json:"mediaHost"`
	CertDir       string `json:"certDir"`
	EdgeScheme    string `json:"edgeScheme"`
	MatchScheme   string `json:"matchScheme"`
}

type rawConfig struct {
	Manager Manager  `json:"manager"`
	Scripts Scripts  `json:"scripts"`
	Pserver Pserver  `json:"pserver"`
	Nginx   rawNginx `json:"nginx"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var raw rawConfig
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, err
	}

	cfg := &Config{
		Manager: raw.Manager,
		Scripts: raw.Scripts,
		Pserver: raw.Pserver,
		Nginx: Nginx{
			GenScript:   raw.Nginx.GenScript,
			MediaHost:   raw.Nginx.MediaHost,
			CertDir:     raw.Nginx.CertDir,
			EdgeScheme:  raw.Nginx.EdgeScheme,
			MatchScheme: raw.Nginx.MatchScheme,
		},
	}

	if raw.Nginx.RegenInterval != "" {
		d, err := time.ParseDuration(raw.Nginx.RegenInterval)
		if err != nil {
			return nil, err
		}
		cfg.Nginx.RegenInterval = d
	}

	cfg.applyDefaults()
	return cfg, nil
}

// ApplyDefaults fills empty nginx/manager/pserver fields (call after patching config in memory).
func (c *Config) ApplyDefaults() {
	c.applyDefaults()
}

func (c *Config) applyDefaults() {
	if c.Manager.Port == 0 {
		c.Manager.Port = 3000
	}
	if c.Manager.Host == "" {
		c.Manager.Host = "0.0.0.0"
	}
	if c.Manager.Username == "" {
		c.Manager.Username = "admin"
	}
	if c.Pserver.TemplateDir == "" {
		c.Pserver.TemplateDir = "/root/palace-template"
	}
	if c.Pserver.InstallPath == "" {
		c.Pserver.InstallPath = "/usr/local/bin/pserver"
	}
	if c.Pserver.VersionsDir == "" {
		c.Pserver.VersionsDir = "/var/lib/palace-manager/pserver-versions"
	}
	if c.Pserver.SdistURL == "" {
		c.Pserver.SdistURL = defaultSdistURL()
	}
	if c.Nginx.GenScript == "" {
		c.Nginx.GenScript = "/usr/local/bin/gen-media-nginx.sh"
	}
	if c.Nginx.RegenInterval == 0 {
		c.Nginx.RegenInterval = 2 * time.Minute
	}
	if c.Nginx.MediaHost == "" {
		c.Nginx.MediaHost = "media.thepalace.app"
	}
	// Certbot stores certs under /etc/letsencrypt/live/<certificate-name>/ which matches the hostname we request.
	if c.Nginx.CertDir == "" {
		c.Nginx.CertDir = fmt.Sprintf("/etc/letsencrypt/live/%s", c.Nginx.MediaHost)
	}
	if c.Nginx.EdgeScheme == "" {
		c.Nginx.EdgeScheme = "https"
	}
	if c.Nginx.MatchScheme == "" {
		c.Nginx.MatchScheme = "both"
	}
	if c.Manager.Theme == "" {
		c.Manager.Theme = "basic"
	}
	if c.Manager.Theme != "metal" && c.Manager.Theme != "basic" {
		c.Manager.Theme = "basic"
	}
	if c.Scripts.Provision == "" {
		c.Scripts.Provision = "/usr/local/lib/palace-manager/scripts/provision-palace.sh"
	}
	if c.Scripts.Update == "" {
		c.Scripts.Update = "/usr/local/lib/palace-manager/scripts/update-pserver.sh"
	}
}

// defaultSdistURL returns the sdist download URL for the current OS and
// architecture, matching the layout published at sdist.thepalace.app.
func defaultSdistURL() string {
	const base = "https://sdist.thepalace.app"

	// Map Go arch names to the sdist naming convention.
	archName := map[string]string{
		"amd64": "amd64",
		"386":   "i386",
		"arm64": "arm64",
		"arm":   "arm",
	}
	arch, ok := archName[runtime.GOARCH]
	if !ok {
		arch = runtime.GOARCH
	}

	switch runtime.GOOS {
	case "linux":
		return fmt.Sprintf("%s/linux/latest-linux-%s.tar.gz", base, arch)
	case "darwin":
		return fmt.Sprintf("%s/mac/latest-darwin-%s.tar.gz", base, arch)
	case "freebsd":
		return fmt.Sprintf("%s/freebsd/latest-freebsd-%s.tar.gz", base, arch)
	case "windows":
		return fmt.Sprintf("%s/win/latest-windows-%s.zip", base, arch)
	default:
		// Fallback: best-effort linux amd64
		return fmt.Sprintf("%s/linux/latest-linux-amd64.tar.gz", base)
	}
}

func DefaultConfig() *Config {
	c := &Config{}
	c.applyDefaults()
	return c
}

func (c *Config) Save(path string) error {
	raw := rawConfig{
		Manager: c.Manager,
		Scripts: c.Scripts,
		Pserver: c.Pserver,
		Nginx: rawNginx{
			GenScript:     c.Nginx.GenScript,
			RegenInterval: c.Nginx.RegenInterval.String(),
			MediaHost:     c.Nginx.MediaHost,
			CertDir:       c.Nginx.CertDir,
			EdgeScheme:    c.Nginx.EdgeScheme,
			MatchScheme:   c.Nginx.MatchScheme,
		},
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(raw)
}
