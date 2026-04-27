// Package auditlog appends JSON-lines audit entries for manager mutations.
package auditlog

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

const DefaultPath = "/etc/palace-manager/audit.jsonl"

// Entry is one line in the audit JSONL file.
type Entry struct {
	TS          string            `json:"ts"`
	Actor       string            `json:"actor"`
	ActorRole   string            `json:"actorRole"`
	ScopeTenant string            `json:"scopeTenant,omitempty"`
	Palace      string            `json:"palace,omitempty"`
	Action      string            `json:"action"`
	Detail      map[string]string `json:"detail,omitempty"`
}

type Store struct {
	mu   sync.Mutex
	path string
}

func New(path string) *Store {
	if path == "" {
		path = DefaultPath
	}
	return &Store{path: path}
}

func (s *Store) Append(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

const maxTailScan = 8 << 20 // read at most last 8 MiB when file is large

// ReadRecent returns parsed entries from the end of the file (newest lines last in file = append order).
// Entries are returned oldest-first within the scanned window (natural read order).
func (s *Store) ReadRecent() ([]Entry, error) {
	s.mu.Lock()
	path := s.path
	s.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size == 0 {
		return nil, nil
	}
	start := int64(0)
	if size > maxTailScan {
		start = size - maxTailScan
		if _, err := f.Seek(start, 0); err != nil {
			return nil, err
		}
		br := bufio.NewReader(f)
		if start > 0 {
			_, _ = br.ReadBytes('\n') // drop partial first line
		}
		return scanEntries(br)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	return scanEntries(bufio.NewReader(f))
}

func scanEntries(br *bufio.Reader) ([]Entry, error) {
	var out []Entry
	for {
		line, err := br.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			break
		}
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		if len(line) == 0 {
			if err != nil {
				break
			}
			continue
		}
		var e Entry
		if json.Unmarshal(line, &e) != nil {
			if err != nil {
				break
			}
			continue
		}
		out = append(out, e)
		if err != nil {
			break
		}
	}
	return out, nil
}
