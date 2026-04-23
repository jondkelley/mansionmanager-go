package nginx

import (
	"bytes"
	"context"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"

	"palace-manager/internal/config"
)

// RegenStatus holds the result of the most recent nginx regen run.
type RegenStatus struct {
	LastRun  time.Time `json:"lastRun"`
	ExitCode int       `json:"exitCode"`
	Output   string    `json:"output"`
	NextRun  time.Time `json:"nextRun"`
}

// Manager runs gen-media-nginx.sh on a ticker and on-demand triggers.
type Manager struct {
	cfg     *config.Nginx
	mu      sync.RWMutex
	status  RegenStatus
	trigger chan struct{}
}

func NewManager(cfg *config.Nginx) *Manager {
	return &Manager{
		cfg:     cfg,
		trigger: make(chan struct{}, 1),
	}
}

// Start runs the background regen loop until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.RegenInterval)
	defer ticker.Stop()

	m.mu.Lock()
	m.status.NextRun = time.Now().Add(m.cfg.RegenInterval)
	m.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.regen(nil)
			m.mu.Lock()
			m.status.NextRun = time.Now().Add(m.cfg.RegenInterval)
			m.mu.Unlock()
		case <-m.trigger:
			m.regen(nil)
			// Reset ticker so we don't double-fire right after a manual trigger
			ticker.Reset(m.cfg.RegenInterval)
			m.mu.Lock()
			m.status.NextRun = time.Now().Add(m.cfg.RegenInterval)
			m.mu.Unlock()
		}
	}
}

// Trigger causes an immediate regen outside the normal tick schedule.
// If a regen is already queued, this is a no-op.
func (m *Manager) Trigger() {
	select {
	case m.trigger <- struct{}{}:
	default:
	}
}

// RegenWithWriter runs gen-media-nginx.sh immediately and streams output to w.
// Used by the API handler for on-demand regen requests.
func (m *Manager) RegenWithWriter(w io.Writer) error {
	return m.regen(w)
}

// Status returns a snapshot of the last regen result.
func (m *Manager) Status() RegenStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) regen(w io.Writer) error {
	args := []string{
		"--scan-homes",
		"--match-scheme", m.cfg.MatchScheme,
		"--edge-scheme", m.cfg.EdgeScheme,
		"--reload",
	}
	if m.cfg.CertDir != "" {
		args = append(args, "--cert-dir", m.cfg.CertDir)
	}
	if m.cfg.MediaHost != "" {
		args = append(args, "--media-host", m.cfg.MediaHost)
		args = append(args, "--nginx-conf", MediaProxySiteConf)
	}

	cmd := exec.Command(m.cfg.GenScript, args...)

	var buf bytes.Buffer
	var out io.Writer = &buf
	if w != nil {
		out = io.MultiWriter(&buf, w)
	}
	cmd.Stdout = out
	cmd.Stderr = out

	runAt := time.Now()
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		log.Printf("nginx regen failed (exit %d): %v", exitCode, err)
	}

	output := buf.String()
	if len(output) > 4096 {
		output = output[len(output)-4096:]
	}

	m.mu.Lock()
	m.status = RegenStatus{
		LastRun:  runAt,
		ExitCode: exitCode,
		Output:   output,
		NextRun:  m.status.NextRun,
	}
	m.mu.Unlock()

	return err
}
