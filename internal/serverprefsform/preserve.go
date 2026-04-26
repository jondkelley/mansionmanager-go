package serverprefsform

import "encoding/json"

// Keys we never read or write through the guided form (copied verbatim from the
// previous serverprefs.json on each save). Form-managed keys such as
// wiz_authoring and wiz_authoring_annotation are merged in map.go, not listed here.
var preservedKeys = []string{
	"moderation_records",
	"moderation_schema",
	"promotion_passwords",
	"promotion_password_schema",
	"command_ranks",
	"operator_hold",
	"name_gag",
	"bless_config",
	"bless_config_schema",
	"nickregistration",
	"registernick",
	"ratbot_avatar",
}

func isPreservedKey(k string) bool {
	for _, p := range preservedKeys {
		if p == k {
			return true
		}
	}
	return false
}

// PreservedKeysPresent returns which preserved keys exist in top (for UI hints).
func PreservedKeysPresent(top map[string]json.RawMessage) []string {
	var out []string
	for _, k := range preservedKeys {
		if raw, ok := top[k]; ok && len(raw) > 0 && string(raw) != "null" {
			out = append(out, k)
		}
	}
	return out
}
