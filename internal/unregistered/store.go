package unregistered

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"palace-manager/internal/registry"
)

// DefaultPath is persisted metadata for unregister-only removals so instances stay recoverable
// even if systemd discovery fails or the palman unit is missing from list-units.
const DefaultPath = "/etc/palace-manager/unregistered-palaces.json"

// Record is a snapshot from the registry at unregister time (not in registry while present here).
type Record struct {
	Name           string    `json:"name"`
	User           string    `json:"user"`
	TCPPort        int       `json:"tcpPort"`
	HTTPPort       int       `json:"httpPort"`
	DataDir        string    `json:"dataDir"`
	PserverVersion string    `json:"pserverVersion,omitempty"`
	YPHost         string    `json:"ypHost,omitempty"`
	YPPort         int       `json:"ypPort,omitempty"`
	QuotaBytesMax  int64     `json:"quotaBytesMax,omitempty"`
	UnregisteredAt time.Time `json:"unregisteredAt"`
}

type Store struct {
	mu      sync.RWMutex
	path    string
	Records []Record `json:"records"`
}

func Load(path string) (*Store, error) {
	st := &Store{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	var raw struct {
		Records []Record `json:"records"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	st.Records = raw.Records
	return st, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) UpsertFromPalace(p registry.Palace, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := Record{
		Name:           p.Name,
		User:           p.User,
		TCPPort:        p.TCPPort,
		HTTPPort:       p.HTTPPort,
		DataDir:        p.DataDir,
		PserverVersion: p.PserverVersion,
		YPHost:         p.YPHost,
		YPPort:         p.YPPort,
		QuotaBytesMax:  p.QuotaBytesMax,
		UnregisteredAt: at.UTC(),
	}
	found := false
	for i := range s.Records {
		if s.Records[i].Name == p.Name {
			s.Records[i] = rec
			found = true
			break
		}
	}
	if !found {
		s.Records = append(s.Records, rec)
	}
	return s.saveLocked()
}

func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.Records[:0]
	for _, r := range s.Records {
		if r.Name != name {
			filtered = append(filtered, r)
		}
	}
	s.Records = filtered
	return s.saveLocked()
}

func (s *Store) Get(name string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.Records {
		if r.Name == name {
			return r, true
		}
	}
	return Record{}, false
}

func (s *Store) All() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Record, len(s.Records))
	copy(out, s.Records)
	return out
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(struct {
		Records []Record `json:"records"`
	}{Records: s.Records}, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
