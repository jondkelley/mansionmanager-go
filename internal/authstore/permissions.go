package authstore

import (
	"fmt"
	"sort"
	"strings"
)

// Palace permission keys for subaccounts (delegated by a tenant).
const (
	PermControl  = "control"
	PermLogs     = "logs"
	PermUsers    = "users"
	PermBans     = "bans"
	PermMedia    = "media"
	PermFiles    = "files"
	PermSettings = "settings"
	PermProps    = "props"
	PermPages    = "pages"
	PermBackups  = "backups"
)

// AllPalacePerms is the canonical list for validation and UI.
var AllPalacePerms = []string{
	PermControl, PermLogs, PermUsers, PermBans, PermMedia, PermFiles,
	PermSettings, PermProps, PermPages, PermBackups,
}

var permSet map[string]struct{}

func init() {
	permSet = make(map[string]struct{}, len(AllPalacePerms))
	for _, p := range AllPalacePerms {
		permSet[p] = struct{}{}
	}
}

func IsValidPalacePerm(p string) bool {
	_, ok := permSet[strings.ToLower(strings.TrimSpace(p))]
	return ok
}

// NormalizePalacePerms trims keys, lowercases permission strings, dedupes, drops invalid perms.
func NormalizePalacePerms(m map[string][]string) map[string][]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string][]string)
	for palace, perms := range m {
		pn := strings.TrimSpace(palace)
		if pn == "" {
			continue
		}
		seen := make(map[string]struct{})
		var list []string
		for _, raw := range perms {
			p := strings.ToLower(strings.TrimSpace(raw))
			if !IsValidPalacePerm(p) {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			list = append(list, p)
		}
		if len(list) > 0 {
			sort.Strings(list)
			out[pn] = list
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// HasPalacePerm reports whether perms for palace include perm (exact key match on palaceName).
func HasPalacePerm(m map[string][]string, palaceName, perm string) bool {
	if m == nil {
		return false
	}
	list, ok := m[palaceName]
	if !ok {
		return false
	}
	for _, p := range list {
		if p == perm {
			return true
		}
	}
	return false
}

// SubaccountPalaceKeys returns sorted palace names that have at least one permission.
func SubaccountPalaceKeys(m map[string][]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if k != "" && len(v) > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// ValidateSubaccountPalacePerms ensures every palace exists in parentPalaces and at least one palace has perms.
func ValidateSubaccountPalacePerms(parentPalaces []string, perms map[string][]string) error {
	norm := NormalizePalacePerms(perms)
	if len(norm) == 0 {
		return ErrSubaccountPalaces
	}
	parentSet := make(map[string]struct{}, len(parentPalaces))
	for _, p := range parentPalaces {
		p = strings.TrimSpace(p)
		if p != "" {
			parentSet[p] = struct{}{}
		}
	}
	for palace := range norm {
		if _, ok := parentSet[palace]; !ok {
			return fmt.Errorf("palace %q is not assigned to parent tenant", palace)
		}
	}
	return nil
}
