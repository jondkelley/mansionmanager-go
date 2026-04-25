// Package palacequota measures service-user home usage and validates quota limits.
package palacequota

import (
	"io/fs"
	"os"
	"path/filepath"
)

const (
	MinBytes int64 = 1 << 20 // 1 MiB
	MaxBytes int64 = 1 << 40 // 1 TiB (sanity cap for custom values)
)

// HomeTreeUsage sums logical sizes of regular files under root (non-recursive into other volumes).
func HomeTreeUsage(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}

// NormalizeMax returns 0 for unlimited, or a clamped positive quota.
func NormalizeMax(v int64) int64 {
	if v <= 0 {
		return 0
	}
	if v < MinBytes {
		return MinBytes
	}
	if v > MaxBytes {
		return MaxBytes
	}
	return v
}
