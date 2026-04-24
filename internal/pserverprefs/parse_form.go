package pserverprefs

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsePrefState parses pserver.prefs text into PrefState and preserves unrecognized lines
// (including comments) in unknownTail. YP-related lines are consumed but not stored — the
// manager applies directory host/port from the registry separately.
func ParsePrefState(content string) (st PrefState, unknownTail string, warnings []string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	var unknown []string
	lines := strings.Split(content, "\n")
	bannerDone := false
	for ln, line := range lines {
		raw := line
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !bannerDone {
			if line == "; Server Prefs" || line == ";" {
				continue
			}
			bannerDone = true
		}
		if strings.HasPrefix(line, ";") {
			unknown = append(unknown, raw)
			continue
		}
		dir, rest := splitFirstToken(line)
		if dir == "" {
			warnings = append(warnings, fmt.Sprintf("line %d: could not parse directive", ln+1))
			unknown = append(unknown, raw)
			continue
		}
		u := strings.ToUpper(dir)
		switch u {
		case "SERVERNAME":
			if s, ok := parseCStringRest(rest); ok {
				st.ServerName = s
			} else {
				warnings = append(warnings, fmt.Sprintf("line %d: bad SERVERNAME", ln+1))
			}
		case "WIZARDPASSWORD", "OPERATORPASSWORD":
			if s, ok := parseCStringRest(rest); ok {
				st.WizardPlain = s
			}
		case "WIZARDPASSWORD_HASH":
			if s, ok := parseCStringRest(rest); ok {
				st.WizardHash = strings.TrimSpace(s)
			}
		case "GODPASSWORD", "OWNERPASSWORD":
			if s, ok := parseCStringRest(rest); ok {
				st.GodPlain = s
			}
		case "GODPASSWORD_HASH":
			if s, ok := parseCStringRest(rest); ok {
				st.GodHash = strings.TrimSpace(s)
			}
		case "HOSTPASSWORD_HASH":
			if s, ok := parseCStringRest(rest); ok {
				st.HostPasswordHash = strings.TrimSpace(s)
			}
		case "PICFOLDER":
			if s, ok := parseCStringRest(rest); ok {
				st.PicFolder = s
			}
		case "MAXSESSIONID":
			st.RecycleLimit = int32(parseInt(rest))
		case "SERVEROPTIONS":
			st.ServerOptions = uint32(parseHexOrInt(rest))
		case "SAVESESSIONKEYS":
			st.SaveSessionKeys = true
		case "PERMISSIONS":
			st.Permissions = uint32(parseHexOrInt(rest))
		case "DEATHPENALTY":
			st.DeathPenaltyMinutes = int16(parseInt(rest))
		case "MAXOCCUPANCY":
			st.MaxOccupancy = int16(parseInt(rest))
		case "ROOMOCCUPANCY":
			st.RoomOccupancy = int16(parseInt(rest))
		case "MINFLOODEVENTS":
			st.MinFloodEvents = int16(parseInt(rest))
		case "PURGEPROPDAYS":
			st.PurgePropDays = int16(parseInt(rest))
		case "AUTOPURGE":
			tok := strings.ToLower(strings.TrimSpace(firstToken(rest)))
			st.AutoPurge = tok == "on" || tok == "1" || tok == "true" || tok == "yes"
		case "SYSOP":
			if s, ok := parseCStringRest(rest); ok {
				st.Sysop = s
			}
		case "URL":
			if s, ok := parseCStringRest(rest); ok {
				st.URL = s
			}
		case "WEBSITE":
			if s, ok := parseCStringRest(rest); ok {
				st.Website = s
			}
		case "MOTD":
			if s, ok := parseCStringRest(rest); ok {
				st.MOTD = s
			}
		case "MACHINETYPE":
			_, _ = parseCStringRest(rest)
		case "BLURB":
			if s, ok := parseCStringRest(rest); ok {
				st.Description = s
			}
		case "ANNOUNCEMENT":
			if s, ok := parseCStringRest(rest); ok {
				st.Announcement = s
			}
		case "HTTP_URL":
			if s, ok := parseCStringRest(rest); ok {
				st.HTTPServer = s
			}
		case "AUTOANNOUNCE":
			if s, ok := parseCStringRest(rest); ok {
				st.AutoAnnounce = s
			}
		case "YPMYEXTADDR", "YPMYEXTPORT":
			// Owned by registry merge.
		case "AUTOREGISTER":
			// Legacy no-op in mansionsource-go.
		case "YPDIRECTORYLIST":
			_, _ = firstToken(rest), true
		case "YPADDR":
			_, _ = parseCStringRest(rest)
		case "CHATLOG":
			if s, ok := parseCStringRest(rest); ok {
				st.ChatLogTypes = strings.TrimSpace(s)
			}
		case "CHATLOG_FILE":
			if s, ok := parseCStringRest(rest); ok {
				st.ChatLogFile = strings.TrimSpace(s)
			}
		case "CHATLOG_NOWARN":
			st.ChatLogNoWarn = true
		case "CHATLOG_FORMAT":
			if s, ok := parseCStringRest(rest); ok {
				st.ChatLogFormat = strings.TrimSpace(s)
			}
		case "NOAUTOREGISTER":
			st.NoAutoRegister = true
		default:
			unknown = append(unknown, raw)
		}
	}
	return st, strings.Join(unknown, "\n"), warnings
}

func splitFirstToken(line string) (token, rest string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	i := strings.IndexAny(line, " \t")
	if i < 0 {
		return strings.ToUpper(line), ""
	}
	return line[:i], strings.TrimSpace(line[i:])
}

func firstToken(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if len(s) > 2 && (s[0] == '0' && (s[1] == 'x' || s[1] == 'X')) {
		v, _ := strconv.ParseUint(s[2:], 16, 32)
		return int(v)
	}
	v, _ := strconv.Atoi(s)
	return v
}

func parseHexOrInt(s string) int {
	return parseInt(s)
}

// parseCStringRest reads a Palace double-quoted C string (pascal-style length in getPString).
func parseCStringRest(rest string) (string, bool) {
	rest = strings.TrimSpace(rest)
	return parsePalaceQuoted(rest)
}

func parsePalaceQuoted(s string) (string, bool) {
	if len(s) == 0 || s[0] != '"' {
		return "", false
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		if c == '"' {
			return b.String(), true
		}
		if c == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case '\\', '"':
				b.WriteByte(s[i])
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				if i+1 < len(s) && isHex(s[i]) && isHex(s[i+1]) {
					v := parseHexByte(s[i : i+2])
					b.WriteByte(v)
					i++
				} else {
					b.WriteByte(s[i])
				}
			}
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	return "", false
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func parseHexByte(two string) byte {
	v, _ := strconv.ParseUint(two, 16, 8)
	return byte(v)
}
