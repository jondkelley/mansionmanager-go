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

func TestMergeServerNameSysop(t *testing.T) {
	base := "SERVERNAME \"Old\"\nSYSOP \"OldOp\"\nMAXOCCUPANCY 50\n"
	got := MergeServerNameSysop(base, "My Mansion", "Joe Sysop")
	if strings.Count(got, "SERVERNAME") != 1 || strings.Count(got, "SYSOP") != 1 {
		t.Fatalf("expected single SERVERNAME and SYSOP, got:\n%s", got)
	}
	if !strings.Contains(got, `SERVERNAME "My Mansion"`) {
		t.Fatalf("server name: %q", got)
	}
	if !strings.Contains(got, `SYSOP "Joe Sysop"`) {
		t.Fatalf("sysop: %q", got)
	}
	if !strings.Contains(got, "MAXOCCUPANCY") {
		t.Fatalf("should keep other prefs: %q", got)
	}
}

func TestMergeServerNameSysop_replacesOnlyIdentity(t *testing.T) {
	base := "SERVERNAME \"x\"\nYPMYEXTADDR \"h\"\n"
	got := MergeServerNameSysop(base, "N", "S")
	if strings.Count(got, "YPMYEXTADDR") != 1 {
		t.Fatalf("YP line should remain: %q", got)
	}
}
