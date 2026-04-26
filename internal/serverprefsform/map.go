package serverprefsform

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const formatVersion = 1

// LoadRawMap reads serverprefs.json into a top-level object map. Missing file → empty map.
func LoadRawMap(path string) (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]json.RawMessage{}, nil
		}
		return nil, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, fmt.Errorf("serverprefs.json: %w", err)
	}
	if top == nil {
		top = map[string]json.RawMessage{}
	}
	return top, nil
}

func cloneTop(in map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(in))
	for k, v := range in {
		cp := make([]byte, len(v))
		copy(cp, v)
		out[k] = json.RawMessage(cp)
	}
	return out
}

func getString(top map[string]json.RawMessage, key string) string {
	raw, ok := top[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func getBool(top map[string]json.RawMessage, key string, absentDefault bool) bool {
	raw, ok := top[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return absentDefault
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return absentDefault
	}
	return b
}

func getInt64(top map[string]json.RawMessage, key string) int64 {
	raw, ok := top[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0
	}
	return n
}

func normalizeRoomAnnotations(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "off", "wizards", "everyone":
		return v
	case "wizard":
		return "wizards"
	case "":
		return "everyone"
	default:
		return "everyone"
	}
}

func normalizeWizAuthoring(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "on", "off", "bless", "godonly":
		return v
	case "":
		return "on"
	default:
		return "on"
	}
}

func normalizeMediaManagerRank(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "wizards", "wizard":
		return "wizards"
	case "gods", "god":
		return "gods"
	default:
		return "owners"
	}
}

func normalizeMediaUploadRank(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off":
		return "off"
	case "wizards", "wizard":
		return "wizards"
	case "gods", "god":
		return "gods"
	default:
		return "owners"
	}
}

func intRoomSetFromMapObj(top map[string]json.RawMessage, key string) []int {
	raw, ok := top[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var obj map[string]bool
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	var out []int
	for ks, v := range obj {
		if !v {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(ks))
		if err != nil || n < -32768 || n > 32767 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func overflowFromTop(top map[string]json.RawMessage) []int {
	raw, ok := top["overflow_rooms"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var arr []int64
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	out := make([]int, 0, len(arr))
	for _, v := range arr {
		if v < -32768 || v > 32767 {
			continue
		}
		out = append(out, int(v))
	}
	return out
}

// MapToForm builds a DTO from the on-disk top map (only guided keys are read).
func MapToForm(top map[string]json.RawMessage) ServerPrefsFormDTO {
	fk := floodKillFromTop(top)
	sl := soundLimitFromTop(top)
	ps := passwordSecurityFromTop(top)
	return ServerPrefsFormDTO{
		Website:                getString(top, "website"),
		YpLanguage:             getString(top, "yp_language"),
		YpCategory:             getString(top, "yp_category"),
		YpDescription:          getString(top, "yp_description"),
		TimeoutRoomID:          int(getInt64(top, "timeout_room_id")),
		AutopurgeBanlistDays:   getInt64(top, "autopurgebanlist_days"),
		UnicodeNames:           getBool(top, "unicode_names", false),
		UnicodeFull:            getBool(top, "unicode", true),
		AltNames:               getBool(top, "alt_names", true),
		NoLoosePropsNonOps:     getBool(top, "nolooseprops_non_ops", false),
		EspEnabled:             getBool(top, "esp_enabled", true),
		RoomAnnotations:        normalizeRoomAnnotations(getString(top, "room_annotations")),
		WizAuthoring:           normalizeWizAuthoring(getString(top, "wiz_authoring")),
		WizAuthoringAnnotation: getBool(top, "wiz_authoring_annotation", true),
		NotifyLogon:            getBool(top, "notify_logon", false),
		NotifyLogoff:           getBool(top, "notify_logoff", false),
		PublicMedia:            getBool(top, "public_media", true),
		SecureProps:            getBool(top, "secure_props", false),
		MediaManagerEnabled:    getBool(top, "media_manager_enabled", true),
		MediaManagerRank:       normalizeMediaManagerRank(getString(top, "media_manager_rank")),
		MediaUploadConfigRank:  normalizeMediaUploadRank(getString(top, "media_upload_config_rank")),
		LegacyClientsBlock:     getBool(top, "legacyclients", false),
		OverflowRoomIDs:        overflowFromTop(top),
		PropFreezeRoomIDs:      intRoomSetFromMapObj(top, "propfreeze_rooms"),
		RatbotsAllowedRoomIDs:  intRoomSetFromMapObj(top, "ratbots_allowed_rooms"),
		FloodKill:              fk,
		SoundLimit:             sl,
		PasswordSecurity:       ps,
	}
}

func floodKillFromTop(top map[string]json.RawMessage) FloodKillDTO {
	raw, ok := top["floodkill_limits"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return FloodKillDTO{}
	}
	var fk FloodKillDTO
	if err := json.Unmarshal(raw, &fk); err != nil {
		return FloodKillDTO{}
	}
	return fk
}

func soundLimitFromTop(top map[string]json.RawMessage) SoundLimitDTO {
	raw, ok := top["sound_limit"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return SoundLimitDTO{Enabled: true, Times: 10, Timeframe: 60}
	}
	var sl SoundLimitDTO
	if err := json.Unmarshal(raw, &sl); err != nil {
		return SoundLimitDTO{Enabled: true, Times: 10, Timeframe: 60}
	}
	if sl.Times <= 0 {
		sl.Times = 10
	}
	if sl.Timeframe <= 0 {
		sl.Timeframe = 60
	}
	return sl
}

func isDefaultPasswordSecurity(p PasswordSecurityDTO) bool {
	return p.MinLength == 8 && p.RequireNumber && !p.RequireSymbol && !p.RequireUpper && !p.RequireLower
}

func passwordSecurityFromTop(top map[string]json.RawMessage) PasswordSecurityDTO {
	raw, ok := top["password_security"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return PasswordSecurityDTO{
			MinLength:     8,
			RequireNumber: true,
		}
	}
	var inner struct {
		MinLength     int  `json:"min_length"`
		RequireNumber bool `json:"require_number"`
		RequireSymbol bool `json:"require_symbol"`
		RequireUpper  bool `json:"require_upper"`
		RequireLower  bool `json:"require_lower"`
	}
	if err := json.Unmarshal(raw, &inner); err != nil {
		return PasswordSecurityDTO{MinLength: 8, RequireNumber: true}
	}
	return PasswordSecurityDTO{
		MinLength:     inner.MinLength,
		RequireNumber: inner.RequireNumber,
		RequireSymbol: inner.RequireSymbol,
		RequireUpper:  inner.RequireUpper,
		RequireLower:  inner.RequireLower,
	}
}

func setJSON(top map[string]json.RawMessage, key string, v interface{}) error {
	if v == nil {
		delete(top, key)
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	top[key] = b
	return nil
}

// ApplyFormToMap merges guided fields onto a clone of orig, always restoring preserved keys
// from orig and writing format version.
func ApplyFormToMap(orig map[string]json.RawMessage, f ServerPrefsFormDTO) (map[string]json.RawMessage, error) {
	top := cloneTop(orig)
	for _, k := range preservedKeys {
		if v, ok := orig[k]; ok {
			top[k] = v
		} else {
			delete(top, k)
		}
	}

	// Strings: empty → delete
	setStringKey := func(key, s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			delete(top, key)
			return
		}
		b, err := json.Marshal(s)
		if err != nil {
			return
		}
		top[key] = b
	}
	setStringKey("website", f.Website)
	setStringKey("yp_language", f.YpLanguage)
	setStringKey("yp_category", f.YpCategory)
	setStringKey("yp_description", f.YpDescription)

	if f.TimeoutRoomID == 0 {
		delete(top, "timeout_room_id")
	} else {
		if err := setJSON(top, "timeout_room_id", int64(f.TimeoutRoomID)); err != nil {
			return nil, err
		}
	}
	if f.AutopurgeBanlistDays <= 0 {
		delete(top, "autopurgebanlist_days")
	} else {
		if err := setJSON(top, "autopurgebanlist_days", f.AutopurgeBanlistDays); err != nil {
			return nil, err
		}
	}

	// unicode_names: false removes key; true writes true (mansionsource SaveUnicodeUserNames)
	if !f.UnicodeNames {
		delete(top, "unicode_names")
	} else {
		if err := setJSON(top, "unicode_names", true); err != nil {
			return nil, err
		}
	}
	// unicode: true default → omit when true
	if f.UnicodeFull {
		delete(top, "unicode")
	} else {
		if err := setJSON(top, "unicode", false); err != nil {
			return nil, err
		}
	}
	if f.AltNames {
		delete(top, "alt_names")
	} else {
		if err := setJSON(top, "alt_names", false); err != nil {
			return nil, err
		}
	}
	if !f.NoLoosePropsNonOps {
		delete(top, "nolooseprops_non_ops")
	} else {
		if err := setJSON(top, "nolooseprops_non_ops", true); err != nil {
			return nil, err
		}
	}
	if f.EspEnabled {
		delete(top, "esp_enabled")
	} else {
		if err := setJSON(top, "esp_enabled", false); err != nil {
			return nil, err
		}
	}
	ra := normalizeRoomAnnotations(f.RoomAnnotations)
	if ra == "everyone" {
		delete(top, "room_annotations")
	} else {
		if err := setJSON(top, "room_annotations", ra); err != nil {
			return nil, err
		}
	}
	wa := normalizeWizAuthoring(f.WizAuthoring)
	if wa == "on" {
		delete(top, "wiz_authoring")
	} else {
		if err := setJSON(top, "wiz_authoring", wa); err != nil {
			return nil, err
		}
	}
	if f.WizAuthoringAnnotation {
		delete(top, "wiz_authoring_annotation")
	} else {
		if err := setJSON(top, "wiz_authoring_annotation", false); err != nil {
			return nil, err
		}
	}
	if !f.NotifyLogon {
		delete(top, "notify_logon")
	} else {
		if err := setJSON(top, "notify_logon", true); err != nil {
			return nil, err
		}
	}
	if !f.NotifyLogoff {
		delete(top, "notify_logoff")
	} else {
		if err := setJSON(top, "notify_logoff", true); err != nil {
			return nil, err
		}
	}
	if f.PublicMedia {
		delete(top, "public_media")
	} else {
		if err := setJSON(top, "public_media", false); err != nil {
			return nil, err
		}
	}
	if !f.SecureProps {
		delete(top, "secure_props")
	} else {
		if err := setJSON(top, "secure_props", true); err != nil {
			return nil, err
		}
	}
	if f.MediaManagerEnabled {
		delete(top, "media_manager_enabled")
	} else {
		if err := setJSON(top, "media_manager_enabled", false); err != nil {
			return nil, err
		}
	}
	mmr := normalizeMediaManagerRank(f.MediaManagerRank)
	if mmr == "owners" {
		delete(top, "media_manager_rank")
	} else {
		if err := setJSON(top, "media_manager_rank", mmr); err != nil {
			return nil, err
		}
	}
	mur := normalizeMediaUploadRank(f.MediaUploadConfigRank)
	switch mur {
	case "owners":
		delete(top, "media_upload_config_rank")
	case "off":
		if err := setJSON(top, "media_upload_config_rank", "off"); err != nil {
			return nil, err
		}
	default:
		if err := setJSON(top, "media_upload_config_rank", mur); err != nil {
			return nil, err
		}
	}
	if !f.LegacyClientsBlock {
		delete(top, "legacyclients")
	} else {
		if err := setJSON(top, "legacyclients", true); err != nil {
			return nil, err
		}
	}

	if len(f.OverflowRoomIDs) == 0 {
		delete(top, "overflow_rooms")
	} else {
		arr := make([]int64, 0, len(f.OverflowRoomIDs))
		for _, id := range f.OverflowRoomIDs {
			if id < -32768 || id > 32767 {
				continue
			}
			arr = append(arr, int64(id))
		}
		if len(arr) == 0 {
			delete(top, "overflow_rooms")
		} else {
			if err := setJSON(top, "overflow_rooms", arr); err != nil {
				return nil, err
			}
		}
	}

	if err := setRoomBoolMap(top, "propfreeze_rooms", f.PropFreezeRoomIDs); err != nil {
		return nil, err
	}
	if err := setRoomBoolMap(top, "ratbots_allowed_rooms", f.RatbotsAllowedRoomIDs); err != nil {
		return nil, err
	}

	// floodkill_limits
	fk := f.FloodKill
	if !fk.Enabled && fk.Move == 0 && fk.Chat == 0 && fk.Whisper == 0 && fk.ESP == 0 && fk.Page == 0 &&
		fk.Prop == 0 && fk.PropDrop == 0 && fk.Draw == 0 && fk.Username == 0 && fk.Time == 0 {
		delete(top, "floodkill_limits")
	} else {
		type fkWire struct {
			Enabled  bool `json:"enabled"`
			Time     int  `json:"time"`
			Move     int  `json:"move"`
			Chat     int  `json:"chat"`
			Whisper  int  `json:"whisper"`
			ESP      int  `json:"esp"`
			Page     int  `json:"page"`
			Prop     int  `json:"prop"`
			PropDrop int  `json:"propdrop"`
			Draw     int  `json:"draw"`
			Username int  `json:"username"`
		}
		if err := setJSON(top, "floodkill_limits", fkWire{
			Enabled: fk.Enabled, Time: fk.Time, Move: fk.Move, Chat: fk.Chat, Whisper: fk.Whisper,
			ESP: fk.ESP, Page: fk.Page, Prop: fk.Prop, PropDrop: fk.PropDrop, Draw: fk.Draw, Username: fk.Username,
		}); err != nil {
			return nil, err
		}
	}

	sl := f.SoundLimit
	if sl.Times <= 0 {
		sl.Times = 10
	}
	if sl.Timeframe <= 0 {
		sl.Timeframe = 60
	}
	if sl.Enabled && sl.Times == 10 && sl.Timeframe == 60 {
		delete(top, "sound_limit")
	} else {
		type slWire struct {
			Enabled   bool `json:"enabled"`
			Times     int  `json:"times"`
			Timeframe int  `json:"timeframe"`
		}
		if err := setJSON(top, "sound_limit", slWire{Enabled: sl.Enabled, Times: sl.Times, Timeframe: sl.Timeframe}); err != nil {
			return nil, err
		}
	}

	ps := f.PasswordSecurity
	if ps.MinLength <= 0 {
		ps.MinLength = 8
	}
	if isDefaultPasswordSecurity(ps) {
		delete(top, "password_security")
	} else {
		type psWire struct {
			MinLength     int  `json:"min_length"`
			RequireNumber bool `json:"require_number"`
			RequireSymbol bool `json:"require_symbol"`
			RequireUpper  bool `json:"require_upper"`
			RequireLower  bool `json:"require_lower"`
		}
		if err := setJSON(top, "password_security", psWire{
			MinLength: ps.MinLength, RequireNumber: ps.RequireNumber, RequireSymbol: ps.RequireSymbol,
			RequireUpper: ps.RequireUpper, RequireLower: ps.RequireLower,
		}); err != nil {
			return nil, err
		}
	}

	if err := setJSON(top, "version", formatVersion); err != nil {
		return nil, err
	}
	return top, nil
}

func setRoomBoolMap(top map[string]json.RawMessage, key string, roomIDs []int) error {
	if len(roomIDs) == 0 {
		delete(top, key)
		return nil
	}
	obj := make(map[string]bool, len(roomIDs))
	for _, id := range roomIDs {
		if id < -32768 || id > 32767 {
			continue
		}
		obj[strconv.Itoa(id)] = true
	}
	if len(obj) == 0 {
		delete(top, key)
		return nil
	}
	return setJSON(top, key, obj)
}
