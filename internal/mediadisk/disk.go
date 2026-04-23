// Package mediadisk reads and writes Palace media folders on disk (same layout as pserver -m).
package mediadisk

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MaxUploadBytes = 32 << 20

var ErrUnsafePath = errors.New("unsafe path")

// SkipProps matches mediaserver: hide props/ and legacy_props/ trees.
func SkipProps(rel string) bool {
	rel = filepath.ToSlash(rel)
	if rel == "props" || strings.HasPrefix(rel, "props/") {
		return true
	}
	if rel == "legacy_props" || strings.HasPrefix(rel, "legacy_props/") {
		return true
	}
	return false
}

// ResolveSafe joins picRoot with a web-style relative path.
func ResolveSafe(picRoot, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = filepath.FromSlash(strings.TrimPrefix(rel, "/"))
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" {
		return "", ErrUnsafePath
	}
	if strings.HasPrefix(rel, "..") {
		return "", ErrUnsafePath
	}
	if SkipProps(filepath.ToSlash(rel)) {
		return "", ErrUnsafePath
	}
	full := filepath.Join(picRoot, rel)
	base := filepath.Clean(picRoot)
	cf := filepath.Clean(full)
	if cf != base && !strings.HasPrefix(cf, base+string(filepath.Separator)) {
		return "", ErrUnsafePath
	}
	return cf, nil
}

// FileRow mirrors mediamanager JSON rows (room/door refs left empty — no live palace state).
type FileRow struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	IsDir        bool   `json:"is_dir"`
	ModTime      int64  `json:"mod_time"`
	FileType     string `json:"file_type"`
	UsedInRoom   string `json:"used_in_room"`
	UsedInDoor   string `json:"used_in_door"`
}

// List walks picRoot and optionally filters rows by case-insensitive substring match on paths.
func List(picRoot, query string) (rows []FileRow, totalBytes int64, nFiles int, err error) {
	query = strings.TrimSpace(query)
	err = filepath.WalkDir(picRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, e := filepath.Rel(picRoot, path)
		if e != nil || rel == "." {
			return nil
		}
		if SkipProps(filepath.ToSlash(rel)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, e2 := d.Info()
		if e2 != nil {
			return nil
		}
		webPath := filepath.ToSlash(rel)
		name := filepath.Base(rel)
		ft := "Folder"
		if !d.IsDir() {
			nFiles++
			totalBytes += info.Size()
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
			if ext != "" {
				ft = strings.ToUpper(ext)
			} else {
				ft = "File"
			}
		}
		rows = append(rows, FileRow{
			Name:       webPath,
			Size:       info.Size(),
			IsDir:      d.IsDir(),
			ModTime:    info.ModTime().Unix(),
			FileType:   ft,
			UsedInRoom: "",
			UsedInDoor: "",
		})
		return nil
	})
	if err != nil {
		return nil, 0, 0, err
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	if query != "" {
		q := strings.ToLower(query)
		var filt []FileRow
		var tb int64
		var nf int
		for _, r := range rows {
			if strings.Contains(strings.ToLower(r.Name), q) {
				filt = append(filt, r)
				if !r.IsDir {
					nf++
					tb += r.Size
				}
			}
		}
		return filt, tb, nf, nil
	}
	return rows, totalBytes, nFiles, nil
}

// NormalizeRenameDestination mirrors mediamanager behavior.
func NormalizeRenameDestination(fromWeb, toInput string) (string, error) {
	fromWeb = filepath.ToSlash(strings.TrimSpace(fromWeb))
	fromWeb = strings.TrimPrefix(fromWeb, "/")
	if fromWeb == "" || fromWeb == "." {
		return "", ErrUnsafePath
	}
	toInput = strings.TrimSpace(toInput)
	if toInput == "" {
		return "", fmt.Errorf("empty destination")
	}
	toSlash := filepath.ToSlash(toInput)
	toSlash = strings.TrimPrefix(toSlash, "/")
	if strings.Contains(toSlash, "..") {
		return "", ErrUnsafePath
	}
	if !strings.Contains(toSlash, "/") {
		if i := strings.LastIndex(fromWeb, "/"); i >= 0 {
			return filepath.Clean(fromWeb[:i+1] + filepath.Base(toSlash)), nil
		}
		return filepath.Clean(filepath.Base(toSlash)), nil
	}
	return filepath.Clean(toSlash), nil
}

// SaveUploaded writes streaming body to dest via atomic rename from .partial.
func SaveUploaded(dest string, src io.Reader) error {
	tmp := dest + ".partial"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, src)
	_ = f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
