package patparse

import (
	"path/filepath"
	"strconv"
	"strings"
)

// MediaRef is the first place a media path appears in the PAT (room + optional spot context).
type MediaRef struct {
	RoomName string
	RoomID   int
	SpotName string // empty for room background only
	SpotID   int    // picture layer: PicID; hotspot: hotspot ID
	Kind     string // "background" | "picture" | "spot"
}

// FirstRefMap maps lowercase lookup keys (basename and optional full path) to first match.
type FirstRefMap map[string]MediaRef

func up(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// pictureKeys matches mediaserver pictureLookupKeys: basename + path when path has /.
func pictureKeys(fname string) []string {
	fname = strings.TrimSpace(fname)
	if fname == "" {
		return nil
	}
	slash := filepath.ToSlash(fname)
	keys := []string{strings.ToLower(filepath.Base(fname))}
	if strings.Contains(slash, "/") {
		keys = append(keys, strings.ToLower(slash))
	}
	return keys
}

func (m FirstRefMap) addFirst(fname string, ref MediaRef) {
	for _, k := range pictureKeys(fname) {
		if k == "" {
			continue
		}
		if _, ok := m[k]; ok {
			continue
		}
		m[k] = ref
	}
}

// ParsePatFirstRefs parses pserver.pat text and returns first-match refs for media filenames.
func ParsePatFirstRefs(pat []byte) FirstRefMap {
	out := make(FirstRefMap)
	data := append(append([]byte(nil), pat...), 0)
	t := &tokenizer{data: data}

	for t.getToken() {
		switch up(t.token) {
		case "ROOM":
			parseRoom(t, out)
		case "ENTRANCE":
			_ = t.parseInt()
		case "BANREC":
			_ = t.parseLong()
			_ = t.parseLong()
			_ = t.parseLong()
		case "BANREC2":
			skipBanRec2(t)
		case "END":
			return out
		}
	}
	return out
}

func skipBanRec2(t *tokenizer) {
	if !t.getToken() {
		return
	}
	_ = t.token
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_ = t.parseLong()
	_, _ = t.getPString()
	_, _ = t.getPString()
	_, _ = t.getPString()
}

func parseRoom(t *tokenizer, out FirstRefMap) {
	var roomID int
	var roomName string
	picByID := map[int16]string{}
	picOrdinal := 0
	hotOrdinal := 0

	for t.getToken() {
		switch up(t.token) {
		case "ENDROOM":
			return
		case "ID":
			roomID = t.parseInt()
		case "NAME":
			ps, ok := t.getPString()
			if ok {
				roomName = pascalToString(ps)
			}
		case "MAXMEMBERS", "MAXGUESTS":
			_ = t.parseShort()
		case "FACES":
			_ = t.parseLong()
		case "ARTIST":
			ps, ok := t.getPString()
			_ = ps
			_ = ok
		case "LOCKED":
			ps, ok := t.getPString()
			_ = ps
			_ = ok
		case "PRIVATE", "NOPAINTING", "NOCYBORGS", "NOLOOSEPROPS", "LOOSEPROPSOPSONLY",
			"HIDDEN", "NOGUESTS", "WIZARDSONLY", "DROPZONE":
			// flags only
		case "PICT":
			ps, ok := t.getPString()
			if ok {
				fn := pascalToString(ps)
				if fn != "" {
					out.addFirst(fn, MediaRef{RoomName: nn(roomName), RoomID: roomID, Kind: "background"})
				}
			}
		case "PICTURE":
			parsePicture(t, picByID, roomID, roomName, &picOrdinal, out)
		case "DOOR", "BOLT", "NAVAREA", "HOTSPOT", "SPOT":
			parseHotspot(t, picByID, roomID, roomName, &hotOrdinal, out)
		case "PROP":
			skipProp(t)
		}
	}
}

func nn(s string) string {
	if strings.TrimSpace(s) == "" {
		return "?"
	}
	return s
}

func parsePicture(t *tokenizer, picByID map[int16]string, roomID int, roomName string, picOrdinal *int, out FirstRefMap) {
	var picID int16
	var fname string
	picID = int16(*picOrdinal + 1)

	for t.getToken() {
		switch up(t.token) {
		case "ENDPICTURE":
			*picOrdinal++
			if fname != "" {
				picByID[picID] = fname
				out.addFirst(fname, MediaRef{
					RoomName: nn(roomName),
					RoomID:   roomID,
					SpotName: "Picture layer",
					SpotID:   int(picID),
					Kind:     "picture",
				})
			}
			return
		case "ID":
			picID = t.parseShort()
		case "NAME":
			ps, ok := t.getPString()
			if ok {
				fname = pascalToString(ps)
			}
		case "TRANSCOLOR":
			_ = t.parseShort()
		}
	}
}

func parseHotspot(t *tokenizer, picByID map[int16]string, roomID int, roomName string, hotOrdinal *int, out FirstRefMap) {
	spotID := int16(*hotOrdinal + 1)
	var spotName string
	var pictIDs []int16

	for t.getToken() {
		tok := up(t.token)
		switch tok {
		case "ENDDOOR", "ENDBOLT", "ENDSPOT", "ENDHOTSPOT":
			*hotOrdinal++
			sid := int(spotID)
			for _, pid := range pictIDs {
				fn := picByID[pid]
				if fn == "" {
					continue
				}
				out.addFirst(fn, MediaRef{
					RoomName: nn(roomName),
					RoomID:   roomID,
					SpotName: nn(spotName),
					SpotID:   sid,
					Kind:     "spot",
				})
			}
			return
		case "NAME":
			ps, ok := t.getPString()
			if ok {
				spotName = pascalToString(ps)
			}
		case "ID":
			spotID = t.parseShort()
		case "PICT", "PICTID":
			pictIDs = append(pictIDs, t.parseShort())
		case "PICTS", "PICTIDS":
			for t.getToken() {
				st := up(t.token)
				if st == "ENDPICTS" || st == "ENDPICTIDS" {
					break
				}
				v, err := strconv.Atoi(t.token)
				if err != nil || v < -32768 || v > 32767 {
					continue
				}
				pid := int16(v)
				if !t.getToken() {
					break
				}
				if t.token == "," {
					t.getToken()
				} else {
					t.ungetToken()
				}
				_, _ = t.parsePoint()
				pictIDs = append(pictIDs, pid)
			}
		case "DEST":
			_ = t.parseShort()
		case "DOOR":
			_ = t.parseShort()
		case "LOC":
			_, _ = t.parsePoint()
		case "LOCKABLE", "SHUTABLE":
			// multi-state door — pict count handled elsewhere
		case "SCRIPT":
			skipScriptBlock(t)
		case "OUTLINE":
			skipOutline(t)
		default:
			if strings.HasPrefix(tok, "DONTMOVE") || strings.HasPrefix(tok, "DRAG") ||
				strings.HasPrefix(tok, "SHOW") || strings.HasPrefix(tok, "INV") ||
				strings.HasPrefix(tok, "FORB") || strings.HasPrefix(tok, "MAND") ||
				strings.HasPrefix(tok, "LAND") || strings.HasPrefix(tok, "FILL") ||
				strings.HasPrefix(tok, "SHADOW") {
				// hotspot flags
			}
		}
	}
}

func skipOutline(t *tokenizer) {
	xFlag := false
	for t.getToken() {
		tok := t.token
		if len(tok) == 0 {
			continue
		}
		if isDigitChar(tok[0]) || (tok[0] == '-' && len(tok) > 1 && isDigitChar(tok[1])) {
			if xFlag {
				xFlag = false
			} else {
				xFlag = true
			}
		} else if tok != "," {
			t.ungetToken()
			break
		}
	}
}

func skipScriptBlock(t *tokenizer) {
	for t.getToken() {
		if strings.EqualFold(t.token, "ENDSCRIPT") {
			break
		}
	}
}

func skipProp(t *tokenizer) {
	for t.getToken() {
		if up(t.token) == "ENDPROP" {
			return
		}
	}
}
