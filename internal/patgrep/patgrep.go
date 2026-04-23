// Package patgrep maps media filenames to PAT references using a lightweight parse of pserver.pat.
package patgrep

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"palace-manager/internal/mediadisk"
	"palace-manager/internal/patparse"
)

const maxPatBytes = 64 << 20

// AnnotateMediaRows sets UsedInRoom / UsedInDoor from the first structured match in pserver.pat.
func AnnotateMediaRows(patPath string, rows []mediadisk.FileRow) (patReadErr string) {
	b, err := os.ReadFile(patPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		return err.Error()
	}
	if len(b) > maxPatBytes {
		b = b[:maxPatBytes]
	}

	refs := patparse.ParsePatFirstRefs(b)

	for i := range rows {
		if rows[i].IsDir {
			continue
		}
		rel := filepath.ToSlash(strings.TrimSpace(rows[i].Name))
		if rel == "" {
			continue
		}
		base := strings.ToLower(filepath.Base(rel))
		ref, ok := refs[base]
		if !ok && strings.Contains(rel, "/") {
			ref, ok = refs[strings.ToLower(rel)]
		}
		if !ok {
			continue
		}
		rows[i].UsedInRoom = formatRoom(ref)
		rows[i].UsedInDoor = formatDoor(ref)
	}
	return ""
}

func formatRoom(ref patparse.MediaRef) string {
	return fmt.Sprintf("%s · room %d", ref.RoomName, ref.RoomID)
}

func formatDoor(ref patparse.MediaRef) string {
	switch ref.Kind {
	case "background":
		return ""
	case "picture":
		return fmt.Sprintf("%s · id %d", ref.SpotName, ref.SpotID)
	case "spot":
		return fmt.Sprintf("%s · id %d", ref.SpotName, ref.SpotID)
	default:
		return ""
	}
}
