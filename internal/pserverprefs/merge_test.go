package pserverprefs

import (
	"strings"
	"testing"
)

func TestMergeYPAnnounce(t *testing.T) {
	base := "; hello\nSERVERNAME \"x\"\nYPMYEXTADDR \"old\"\nYPMYEXTPORT 1\n"
	got := MergeYPAnnounce(base, "new.host", 9998)
	if strings.Count(got, "YPMYEXTADDR") != 1 {
		t.Fatalf("expected one YPMYEXTADDR, got:\n%s", got)
	}
	if !strings.Contains(got, `YPMYEXTADDR "new.host"`) {
		t.Fatalf("missing new host: %q", got)
	}
	if !strings.Contains(got, "YPMYEXTPORT 9998") {
		t.Fatalf("missing new port: %q", got)
	}
	if strings.Contains(got, "old") {
		t.Fatalf("old host should be removed: %q", got)
	}
}

func TestMergeYPAnnounce_clear(t *testing.T) {
	base := "YPMYEXTADDR \"x\"\nYPMYEXTPORT 1\n"
	got := MergeYPAnnounce(base, "", 0)
	if strings.Contains(got, "YPMYEXT") {
		t.Fatalf("expected YP lines cleared, got %q", got)
	}
}
