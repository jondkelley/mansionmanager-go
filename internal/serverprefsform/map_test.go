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
		RoomAnnotations: "everyone", WizAuthoring: "on", WizAuthoringAnnotation: true,
		SoundLimit: SoundLimitDTO{Enabled: true, Times: 10, Timeframe: 60}}
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
		"moderation_schema":  json.RawMessage(`"palaceserver-go.moderation/v1"`),
		"moderation_records": json.RawMessage(`[{"id":"x","source":"ban"}]`),
		"website":            json.RawMessage(`"https://old.example"`),
	}
	form := ServerPrefsFormDTO{
		Website:                "https://new.example",
		UnicodeFull:            true,
		EspEnabled:             true,
		PublicMedia:            true,
		AltNames:               true,
		RoomAnnotations:        "everyone",
		WizAuthoring:           "on",
		WizAuthoringAnnotation: true,
		SoundLimit:             SoundLimitDTO{Enabled: true, Times: 10, Timeframe: 60},
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

func TestWizAuthoringMapRoundTrip(t *testing.T) {
	orig := map[string]json.RawMessage{
		"wiz_authoring":            json.RawMessage(`"bless"`),
		"wiz_authoring_annotation": json.RawMessage(`false`),
	}
	form := MapToForm(orig)
	if form.WizAuthoring != "bless" || form.WizAuthoringAnnotation {
		t.Fatalf("MapToForm: wizAuthoring=%q annotation=%v", form.WizAuthoring, form.WizAuthoringAnnotation)
	}
	base := ServerPrefsFormDTO{
		UnicodeFull:            true,
		EspEnabled:             true,
		AltNames:               true,
		PublicMedia:            true,
		RoomAnnotations:        "everyone",
		WizAuthoring:           "godonly",
		WizAuthoringAnnotation: true,
		SoundLimit:             SoundLimitDTO{Enabled: true, Times: 10, Timeframe: 60},
	}
	out, err := ApplyFormToMap(orig, base)
	if err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := json.Unmarshal(out["wiz_authoring"], &mode); err != nil || mode != "godonly" {
		t.Fatalf("wiz_authoring = %q err=%v", mode, err)
	}
	if _, ok := out["wiz_authoring_annotation"]; ok {
		t.Fatal("expected wiz_authoring_annotation key omitted when annotation enabled (default)")
	}

	out2, err := ApplyFormToMap(out, ServerPrefsFormDTO{
		UnicodeFull:            true,
		EspEnabled:             true,
		AltNames:               true,
		PublicMedia:            true,
		RoomAnnotations:        "everyone",
		WizAuthoring:           "on",
		WizAuthoringAnnotation: false,
		SoundLimit:             SoundLimitDTO{Enabled: true, Times: 10, Timeframe: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out2["wiz_authoring"]; ok {
		t.Fatal("expected wiz_authoring omitted when on")
	}
	var ann bool
	if err := json.Unmarshal(out2["wiz_authoring_annotation"], &ann); err != nil || ann {
		t.Fatalf("wiz_authoring_annotation = %v err=%v", ann, err)
	}
}
