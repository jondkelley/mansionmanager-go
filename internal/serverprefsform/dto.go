package serverprefsform

// ServerPrefsFormDTO is the guided-editor JSON shape (camelCase for the web UI).
// Semantics match mansionsource-go internal/serverprefs LoadIfPresent / Save* helpers.
type ServerPrefsFormDTO struct {
	Website       string `json:"website"`
	YpLanguage    string `json:"ypLanguage"`
	YpCategory    string `json:"ypCategory"`
	YpDescription string `json:"ypDescription"`

	TimeoutRoomID        int   `json:"timeoutRoomId"`        // 0 = omit key
	AutopurgeBanlistDays int64 `json:"autopurgeBanlistDays"` // 0 = omit

	UnicodeNames       bool `json:"unicodeNames"`
	UnicodeFull        bool `json:"unicodeFull"`        // serverprefs "unicode"
	AltNames           bool `json:"altNames"`           // serverprefs "alt_names"
	NoLoosePropsNonOps bool `json:"noLoosePropsNonOps"` // nolooseprops_non_ops
	EspEnabled         bool `json:"espEnabled"`
	RoomAnnotations    string `json:"roomAnnotations"` // everyone | wizards | off
	NotifyLogon        bool   `json:"notifyLogon"`
	NotifyLogoff       bool   `json:"notifyLogoff"`

	PublicMedia           bool   `json:"publicMedia"`
	SecureProps           bool   `json:"secureProps"`
	MediaManagerEnabled   bool   `json:"mediaManagerEnabled"`
	MediaManagerRank      string `json:"mediaManagerRank"`      // owners | gods | wizards
	MediaUploadConfigRank string `json:"mediaUploadConfigRank"` // owners | gods | wizards | off

	LegacyClientsBlock bool `json:"legacyClientsBlock"` // legacyclients in file

	OverflowRoomIDs         []int `json:"overflowRoomIds"`
	PropFreezeRoomIDs       []int `json:"propFreezeRoomIds"`
	RatbotsAllowedRoomIDs   []int `json:"ratbotsAllowedRoomIds"`

	FloodKill        FloodKillDTO        `json:"floodKill"`
	SoundLimit       SoundLimitDTO       `json:"soundLimit"`
	PasswordSecurity PasswordSecurityDTO `json:"passwordSecurity"`
}

// FloodKillDTO mirrors serverprefs.json "floodkill_limits".
type FloodKillDTO struct {
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

// SoundLimitDTO mirrors serverprefs.json "sound_limit".
type SoundLimitDTO struct {
	Enabled   bool `json:"enabled"`
	Times     int  `json:"times"`
	Timeframe int  `json:"timeframe"`
}

// PasswordSecurityDTO mirrors top-level "password_security".
type PasswordSecurityDTO struct {
	MinLength     int  `json:"minLength"`
	RequireNumber bool `json:"requireNumber"`
	RequireSymbol bool `json:"requireSymbol"`
	RequireUpper  bool `json:"requireUpper"`
	RequireLower  bool `json:"requireLower"`
}
