package pserverprefs

import "strings"

// PrefsFormDTO is the JSON shape for the structured editor (wizard/god passwords are not edited here).
type PrefsFormDTO struct {
	ServerName    string `json:"serverName"`
	Sysop         string `json:"sysop"`
	URL           string `json:"url"`
	Website       string `json:"website"`
	MOTD          string `json:"motd"`
	Blurb         string `json:"blurb"`
	Announcement  string `json:"announcement"`
	DeathPenalty  int    `json:"deathPenalty"`
	MaxOccupancy  int    `json:"maxOccupancy"`
	RoomOccupancy int    `json:"roomOccupancy"`
	MinFlood      int    `json:"minFloodEvents"`
	PurgePropDays int    `json:"purgePropDays"`
	RecycleLimit  int    `json:"recycleLimit"`
	ChatLogTypes  string `json:"chatLogTypes"`
	ChatLogFile   string `json:"chatLogFile"`
	ChatLogFormat string `json:"chatLogFormat"` // json | csv
	ChatLogNoWarn bool   `json:"chatLogNoWarn"`
}

// StateToDTO converts internal parse state to an API-safe DTO.
func StateToDTO(st PrefState) PrefsFormDTO {
	return PrefsFormDTO{
		ServerName:    st.ServerName,
		Sysop:         st.Sysop,
		URL:           st.URL,
		Website:       st.Website,
		MOTD:          st.MOTD,
		Blurb:         st.Description,
		Announcement:  st.Announcement,
		DeathPenalty:  int(st.DeathPenaltyMinutes),
		MaxOccupancy:  int(st.MaxOccupancy),
		RoomOccupancy: int(st.RoomOccupancy),
		MinFlood:      int(st.MinFloodEvents),
		PurgePropDays: int(st.PurgePropDays),
		RecycleLimit:  int(st.RecycleLimit),
		ChatLogTypes:  st.ChatLogTypes,
		ChatLogFile:   st.ChatLogFile,
		ChatLogFormat: st.ChatLogFormat,
		ChatLogNoWarn: st.ChatLogNoWarn,
	}
}

// MergeDTO applies form submission onto existing disk state, preserving wizard/god/host password
// material and directives not exposed in the DTO (PERMISSIONS, SERVEROPTIONS, PICFOLDER, AUTOPURGE,
// SAVESESSIONKEYS, NOAUTOREGISTER, HTTP_URL, AUTOANNOUNCE) from the previous file.
func MergeDTO(d PrefsFormDTO, old PrefState) PrefState {
	st := PrefState{
		ServerName:    d.ServerName,
		Sysop:         d.Sysop,
		URL:           d.URL,
		Website:       d.Website,
		MOTD:          d.MOTD,
		Description:   d.Blurb,
		Announcement:  d.Announcement,
		DeathPenaltyMinutes: int16(clampShort(d.DeathPenalty)),
		MaxOccupancy:        int16(clampShort(d.MaxOccupancy)),
		RoomOccupancy:       int16(clampShort(d.RoomOccupancy)),
		MinFloodEvents:      int16(clampShort(d.MinFlood)),
		PurgePropDays:       int16(clampShort(d.PurgePropDays)),
		RecycleLimit:        int32(d.RecycleLimit),
		ChatLogTypes:        d.ChatLogTypes,
		ChatLogFile:         d.ChatLogFile,
		ChatLogFormat:       d.ChatLogFormat,
		ChatLogNoWarn:       d.ChatLogNoWarn,

		HTTPServer:    old.HTTPServer,
		AutoAnnounce:  old.AutoAnnounce,
		Permissions:         old.Permissions,
		ServerOptions:       old.ServerOptions,
		PicFolder:           old.PicFolder,
		AutoPurge:           old.AutoPurge,
		SaveSessionKeys:     old.SaveSessionKeys,
		NoAutoRegister:      old.NoAutoRegister,
		WizardPlain:         old.WizardPlain,
		WizardHash:          old.WizardHash,
		GodPlain:            old.GodPlain,
		GodHash:             old.GodHash,
		HostPasswordHash:    old.HostPasswordHash,
	}
	return st
}

func clampShort(v int) int {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return v
}

// RenderWithUnknown appends preserved comment / unknown directive lines after the generated prefs block.
func RenderWithUnknown(st PrefState, unknownTail string) string {
	base := RenderPrefState(st)
	unknownTail = strings.TrimRight(unknownTail, "\n")
	if unknownTail == "" {
		return base
	}
	if base != "" && !strings.HasSuffix(base, "\n") {
		base += "\n"
	}
	return base + unknownTail + "\n"
}
