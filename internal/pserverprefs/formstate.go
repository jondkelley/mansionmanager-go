package pserverprefs

// PrefState holds all directives understood from pserver.prefs (mansionsource-go script/prefs.go).
// YPMYEXTADDR/YPMYEXTPORT are omitted when rendering from the manager; registry merge adds them.
type PrefState struct {
	ServerName string

	WizardPlain      string
	WizardHash       string
	GodPlain         string
	GodHash          string
	HostPasswordHash string

	Permissions         uint32
	DeathPenaltyMinutes int16
	MaxOccupancy        int16
	RoomOccupancy       int16
	MinFloodEvents      int16
	PurgePropDays       int16
	AutoPurge           bool
	RecycleLimit        int32
	ServerOptions       uint32
	SaveSessionKeys     bool

	PicFolder string

	Sysop        string
	URL          string
	Website      string
	MOTD         string
	Description  string // BLURB
	HTTPServer   string // HTTP_URL
	AutoAnnounce string

	ChatLogTypes   string
	ChatLogFile    string
	ChatLogFormat  string // "json" or "csv"
	ChatLogNoWarn  bool
	NoAutoRegister bool

	// Ignored directives consumed without storing (no-op); not re-emitted.
}
