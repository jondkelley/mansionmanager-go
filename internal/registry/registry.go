package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

const DefaultPath = "/etc/palace-manager/registry.json"

type Palace struct {
	Name          string    `json:"name"`
	User          string    `json:"user"`
	TCPPort       int       `json:"tcpPort"`
	HTTPPort      int       `json:"httpPort"`
	DataDir       string    `json:"dataDir"`
	ProvisionedAt time.Time `json:"provisionedAt"`
	// PserverVersion is a pinned archived semver (e.g. "0.3.5") or empty for the shared "latest" install path.
	PserverVersion string `json:"pserverVersion,omitempty"`
	// YPHost / YPPort are written to pserver.prefs as YPMYEXTADDR / YPMYEXTPORT for directory registration.
	YPHost string `json:"ypHost,omitempty"`
	YPPort int    `json:"ypPort,omitempty"`
	// QuotaBytesMax is a hard cap on total logical size of regular files under the service user's
	// Unix home directory. Zero or omitted means unlimited (legacy palaces).
	QuotaBytesMax int64 `json:"quotaBytesMax,omitempty"`
}

type Registry struct {
	mu      sync.RWMutex
	path    string
	Palaces []Palace `json:"palaces"`
}

func Load(path string) (*Registry, error) {
	r := &Registry{path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) Save() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll("/etc/palace-manager", 0755); err != nil {
		return err
	}

	return os.WriteFile(r.path, data, 0644)
}

func (r *Registry) Add(p Palace) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, existing := range r.Palaces {
		if existing.Name == p.Name {
			r.Palaces[i] = p
			return r.saveUnlocked()
		}
	}
	r.Palaces = append(r.Palaces, p)
	return r.saveUnlocked()
}

// UpdatePserverVersion sets the pserver binary pin for a palace (empty = use shared "latest" at installPath).
func (r *Registry) UpdatePserverVersion(name, pserverVersion string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Palaces {
		if r.Palaces[i].Name == name {
			r.Palaces[i].PserverVersion = pserverVersion
			return r.saveUnlocked()
		}
	}
	return fmt.Errorf("palace %q not found in registry", name)
}

func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	filtered := r.Palaces[:0]
	for _, p := range r.Palaces {
		if p.Name != name {
			filtered = append(filtered, p)
		}
	}
	r.Palaces = filtered
	return r.saveUnlocked()
}

func (r *Registry) Get(name string) (Palace, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.Palaces {
		if p.Name == name {
			return p, true
		}
	}
	return Palace{}, false
}

func (r *Registry) All() []Palace {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Palace, len(r.Palaces))
	copy(out, r.Palaces)
	return out
}

func (r *Registry) PortInUse(tcp, http int) bool {
	return r.portInUseLocked(tcp, http, "")
}

// PortInUseExcept returns true if tcp/http conflicts with another palace's ports, ignoring exceptName.
func (r *Registry) PortInUseExcept(tcp, http int, exceptName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.portInUseLocked(tcp, http, exceptName)
}

func (r *Registry) portInUseLocked(tcp, http int, exceptName string) bool {
	for _, p := range r.Palaces {
		if exceptName != "" && p.Name == exceptName {
			continue
		}
		if p.TCPPort == tcp || p.HTTPPort == http || p.TCPPort == http || p.HTTPPort == tcp {
			return true
		}
	}
	return false
}

// PutPalace removes the registry row keyed by oldKey and inserts palace p (p.Name is the new canonical name).
// Use oldKey == p.Name to update ports/quota in place without renaming the systemd unit.
func (r *Registry) PutPalace(oldKey string, p Palace) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, x := range r.Palaces {
		if x.Name != oldKey && x.Name == p.Name {
			return fmt.Errorf("palace name %q is already in use", p.Name)
		}
	}

	out := make([]Palace, 0, len(r.Palaces))
	found := false
	for _, x := range r.Palaces {
		if x.Name == oldKey {
			found = true
			continue
		}
		out = append(out, x)
	}
	if !found {
		return fmt.Errorf("palace %q not found in registry", oldKey)
	}
	out = append(out, p)
	r.Palaces = out
	return r.saveUnlocked()
}

func (r *Registry) saveUnlocked() error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/palace-manager", 0755); err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0644)
}
