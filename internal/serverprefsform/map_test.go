package serverprefsform

import (
	"encoding/json"
	"testing"
)

func TestApplyFormPreservesPromotionPasswords(t *testing.T) {
	orig := map[string]json.RawMessage{
		"promotion_passwords": json.RawMessage(`{"global":{"wizard_hash":"x"}}`),
		"unicode_names":       json.RawMessage(`true`),
	}
	form := ServerPrefsFormDTO{UnicodeNames: false, UnicodeFull: true, EspEnabled: true, AltNames: true, PublicMedia: true,
		RoomAnnotations: "everyone", SoundLimit: SoundLimitDTO{Enabled: true, Times: 10, Timeframe: 60}}
	out, err := ApplyFormToMap(orig, form)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["promotion_passwords"]; !ok {
		t.Fatal("expected promotion_passwords preserved")
	}
}

func TestApplyFormPreservesModeration(t *testing.T) {
	orig := map[string]json.RawMessage{
		"moderation_schema": json.RawMessage(`"palaceserver-go.moderation/v1"`),
		"moderation_records": json.RawMessage(`[{"id":"x","source":"ban"}]`),
		"website":            json.RawMessage(`"https://old.example"`),
	}
	form := ServerPrefsFormDTO{
		Website:       "https://new.example",
		UnicodeFull:   true,
		EspEnabled:    true,
		PublicMedia:   true,
		AltNames:      true,
		RoomAnnotations: "everyone",
		SoundLimit:    SoundLimitDTO{Enabled: true, Times: 10, Timeframe: 60},
	}
	out, err := ApplyFormToMap(orig, form)
	if err != nil {
		t.Fatal(err)
	}
	if string(out["moderation_records"]) != string(orig["moderation_records"]) {
		t.Fatalf("moderation_records changed")
	}
	var w string
	if err := json.Unmarshal(out["website"], &w); err != nil || w != "https://new.example" {
		t.Fatalf("website = %q err=%v", w, err)
	}
}
