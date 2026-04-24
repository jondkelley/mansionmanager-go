package pserverprefs

import (
	"fmt"
	"strings"
)

// RenderPrefState writes pserver.prefs text from state. It does not emit YPMYEXTADDR/YPMYEXTPORT;
// use MergeYPAnnounce afterward when applying registry directory settings.
func RenderPrefState(st PrefState) string {
	var w strings.Builder
	w.WriteString("; Server Prefs\n;\n")

	if st.ServerName != "" {
		fmt.Fprintf(&w, "SERVERNAME %s\n", palaceQuoted(st.ServerName))
	}
	if st.WizardPlain != "" {
		fmt.Fprintf(&w, "WIZARDPASSWORD %s\n", palaceQuoted(st.WizardPlain))
	} else if strings.TrimSpace(st.WizardHash) != "" {
		fmt.Fprintf(&w, "WIZARDPASSWORD_HASH %s\n", palaceQuoted(strings.TrimSpace(st.WizardHash)))
	}
	if st.GodPlain != "" {
		fmt.Fprintf(&w, "GODPASSWORD %s\n", palaceQuoted(st.GodPlain))
	} else if strings.TrimSpace(st.GodHash) != "" {
		fmt.Fprintf(&w, "GODPASSWORD_HASH %s\n", palaceQuoted(strings.TrimSpace(st.GodHash)))
	}
	if strings.TrimSpace(st.HostPasswordHash) != "" {
		fmt.Fprintf(&w, "HOSTPASSWORD_HASH %s\n", palaceQuoted(strings.TrimSpace(st.HostPasswordHash)))
	}

	fmt.Fprintf(&w, "PERMISSIONS 0x%-8X\n", uint32(st.Permissions))
	if st.DeathPenaltyMinutes != 0 {
		fmt.Fprintf(&w, "DEATHPENALTY %d\n", st.DeathPenaltyMinutes)
	}
	if st.MaxOccupancy != 0 {
		fmt.Fprintf(&w, "MAXOCCUPANCY %d\n", st.MaxOccupancy)
	}
	if st.RoomOccupancy != 0 {
		fmt.Fprintf(&w, "ROOMOCCUPANCY %d\n", st.RoomOccupancy)
	}
	if st.MinFloodEvents != 0 {
		fmt.Fprintf(&w, "MINFLOODEVENTS %d\n", st.MinFloodEvents)
	}
	if st.PurgePropDays != 0 {
		fmt.Fprintf(&w, "PURGEPROPDAYS %d\n", st.PurgePropDays)
	}
	if st.AutoPurge {
		w.WriteString("AUTOPURGE ON\n")
	}
	if st.RecycleLimit != 0 {
		fmt.Fprintf(&w, "MAXSESSIONID %d\n", st.RecycleLimit)
	}
	if st.ServerOptions != 0 {
		fmt.Fprintf(&w, "SERVEROPTIONS 0x%-8X\n", uint32(st.ServerOptions))
	}
	if st.SaveSessionKeys {
		w.WriteString("SAVESESSIONKEYS\n")
	}
	if st.PicFolder != "" {
		fmt.Fprintf(&w, "PICFOLDER %s\n", palaceQuoted(st.PicFolder))
	}
	if st.Sysop != "" {
		fmt.Fprintf(&w, "SYSOP %s\n", palaceQuoted(st.Sysop))
	}
	if st.URL != "" {
		fmt.Fprintf(&w, "URL %s\n", palaceQuoted(st.URL))
	}
	if st.Website != "" {
		fmt.Fprintf(&w, "WEBSITE %s\n", palaceQuoted(st.Website))
	}
	if strings.TrimSpace(st.MOTD) != "" {
		fmt.Fprintf(&w, "MOTD %s\n", palaceQuoted(strings.TrimSpace(st.MOTD)))
	}
	if st.Description != "" {
		fmt.Fprintf(&w, "BLURB %s\n", palaceQuoted(st.Description))
	}
	if st.Announcement != "" {
		fmt.Fprintf(&w, "ANNOUNCEMENT %s\n", palaceQuoted(st.Announcement))
	}
	if st.HTTPServer != "" {
		fmt.Fprintf(&w, "HTTP_URL %s\n", palaceQuoted(st.HTTPServer))
	}
	if strings.TrimSpace(st.ChatLogTypes) != "" {
		fmt.Fprintf(&w, "CHATLOG %s\n", palaceQuoted(strings.TrimSpace(st.ChatLogTypes)))
	}
	if strings.TrimSpace(st.ChatLogFile) != "" {
		fmt.Fprintf(&w, "CHATLOG_FILE %s\n", palaceQuoted(strings.TrimSpace(st.ChatLogFile)))
	}
	if st.ChatLogNoWarn {
		w.WriteString("CHATLOG_NOWARN\n")
	}
	if strings.TrimSpace(st.ChatLogTypes) != "" && strings.TrimSpace(st.ChatLogFormat) != "" {
		f := strings.TrimSpace(st.ChatLogFormat)
		if strings.EqualFold(f, "csv") {
			fmt.Fprintf(&w, "CHATLOG_FORMAT %s\n", palaceQuoted("csv"))
		} else {
			fmt.Fprintf(&w, "CHATLOG_FORMAT %s\n", palaceQuoted("json"))
		}
	}
	if st.AutoAnnounce != "" {
		fmt.Fprintf(&w, "AUTOANNOUNCE %s\n", palaceQuoted(st.AutoAnnounce))
	}
	if st.NoAutoRegister {
		w.WriteString("NOAUTOREGISTER\n")
	}

	out := w.String()
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}
