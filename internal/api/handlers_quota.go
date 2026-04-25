package api

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"palace-manager/internal/instance"
	"palace-manager/internal/palacequota"
)

var errPalaceQuotaExceeded = errors.New("home directory quota exceeded; delete or shrink files before uploading")

func (s *Server) enrichQuota(inst *instance.Instance) {
	p, ok := s.reg.Get(inst.Name)
	if !ok {
		return
	}
	max := p.QuotaBytesMax
	inst.QuotaBytesMax = max
	if max <= 0 {
		inst.HomeUsedBytes = 0
		inst.QuotaExceeded = false
		return
	}
	home, err := s.resolvePalaceUnixHome(inst.Name)
	if err != nil {
		inst.HomeUsedBytes = 0
		inst.QuotaExceeded = false
		return
	}
	used, err := palacequota.HomeTreeUsage(home)
	if err != nil {
		inst.HomeUsedBytes = 0
		inst.QuotaExceeded = false
		return
	}
	inst.HomeUsedBytes = used
	inst.QuotaExceeded = used > max
}

func (s *Server) quotaMaxBytes(palaceName string) int64 {
	p, ok := s.reg.Get(palaceName)
	if !ok {
		return 0
	}
	return p.QuotaBytesMax
}

func (s *Server) quotaUsedHome(palaceName string) (used int64, ok bool) {
	home, err := s.resolvePalaceUnixHome(palaceName)
	if err != nil {
		return 0, false
	}
	u, err := palacequota.HomeTreeUsage(home)
	return u, err == nil
}

// quotaRejectAfterChange blocks a write when the projected home tree size would exceed the palace quota.
// oldBytes/newBytes refer to the logical sizes of the affected file(s) only (zero oldBytes if creating).
func (s *Server) quotaRejectAfterChange(palaceName string, oldBytes, newBytes int64) error {
	max := s.quotaMaxBytes(palaceName)
	if max <= 0 {
		return nil
	}
	used, ok := s.quotaUsedHome(palaceName)
	if !ok {
		return nil
	}
	if used-oldBytes+newBytes > max {
		return errPalaceQuotaExceeded
	}
	return nil
}

func fileSizeOrZero(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if !fi.Mode().IsRegular() {
		return 0
	}
	return fi.Size()
}

// assertConfigSnapshotQuotaHeadroom rejects dated config snapshots that would push the home tree over quota.
func (s *Server) assertConfigSnapshotQuotaHeadroom(palaceName, dataDir string, nowUTC time.Time) error {
	max := s.quotaMaxBytes(palaceName)
	if max <= 0 {
		return nil
	}
	used, ok := s.quotaUsedHome(palaceName)
	if !ok {
		return nil
	}
	bdir := filepath.Join(dataDir, "backups")
	var delta int64
	for _, base := range []string{"pserver.pat", "pserver.prefs", "serverprefs.json"} {
		src := filepath.Join(dataDir, base)
		fi, err := os.Stat(src)
		if err != nil {
			continue
		}
		if !fi.Mode().IsRegular() {
			continue
		}
		dst := filepath.Join(bdir, configBackupDestName(base, nowUTC))
		delta += fi.Size() - fileSizeOrZero(dst)
	}
	if used+delta > max {
		return errPalaceQuotaExceeded
	}
	return nil
}
