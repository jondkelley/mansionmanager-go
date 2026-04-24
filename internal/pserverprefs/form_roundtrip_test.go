package pserverprefs

import (
	"strings"
	"testing"
)

func TestParseRenderRoundtrip(t *testing.T) {
	sample := `; Server Prefs
;
SERVERNAME "Test Palace"
PERMISSIONS 0x00001FFF
MAXOCCUPANCY 50
SYSOP "SysOp Name"
BLURB "A nice place"
HTTP_URL "http://example.com:8080"
`
	st, unk, _ := ParsePrefState(sample)
	if unk != "" {
		t.Fatalf("unexpected unknown: %q", unk)
	}
	out := RenderPrefState(st)
	st2, unk2, _ := ParsePrefState(out)
	if unk2 != "" {
		t.Fatalf("unknown after render: %q", unk2)
	}
	if st2.ServerName != st.ServerName || st2.Sysop != st.Sysop {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", st, st2)
	}
	if !strings.Contains(out, "SERVERNAME") || !strings.Contains(out, "PERMISSIONS") {
		t.Fatal(out)
	}
}
