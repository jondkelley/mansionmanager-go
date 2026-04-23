package patparse

import (
	"strconv"
	"strings"
)

// tokenizer is a minimal copy of mansionsource script tokenizer for .pat text.
type tokenizer struct {
	data  []byte
	pos   int
	token string
	ungot bool
}

func (t *tokenizer) getToken() bool {
	if t.ungot {
		t.ungot = false
		return true
	}
again:
	if t.pos >= len(t.data) || t.data[t.pos] == 0 {
		return false
	}
	b := t.data[t.pos]
	if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
		t.pos++
		goto again
	}
	if b == '#' || b == ';' {
		for t.pos < len(t.data) && t.data[t.pos] != '\r' && t.data[t.pos] != '\n' {
			t.pos++
		}
		goto again
	}
	if isAlnumByte(b) || b == '_' || b == '.' ||
		(b == '-' && t.pos+1 < len(t.data) && isDigitByte(t.data[t.pos+1])) {
		var buf []byte
		if b == '-' {
			buf = append(buf, b)
			t.pos++
		}
		for t.pos < len(t.data) && (isAlnumByte(t.data[t.pos]) || t.data[t.pos] == '_' || t.data[t.pos] == '.') {
			buf = append(buf, t.data[t.pos])
			t.pos++
		}
		t.token = string(buf)
		return true
	}
	if b == '"' {
		var buf []byte
		buf = append(buf, b)
		t.pos++
		for t.pos < len(t.data) && t.data[t.pos] != '"' {
			if t.data[t.pos] == '\\' {
				t.pos++
				c := t.data[t.pos]
				if c == '\\' || c == '"' {
					buf = append(buf, c)
					t.pos++
				} else if t.pos+1 < len(t.data) {
					hi := hexNibble(t.data[t.pos])
					lo := hexNibble(t.data[t.pos+1])
					buf = append(buf, byte(hi<<4|lo))
					t.pos += 2
				}
			} else {
				buf = append(buf, t.data[t.pos])
				t.pos++
			}
		}
		if t.pos < len(t.data) && t.data[t.pos] == '"' {
			buf = append(buf, '"')
			t.pos++
		}
		t.token = string(buf)
		return true
	}
	if strings.ContainsRune("{}[](),", rune(b)) {
		t.token = string(b)
		t.pos++
		return true
	}
	if isPunctByte(b) {
		var buf []byte
		for t.pos < len(t.data) && (isPunctByte(t.data[t.pos]) || t.data[t.pos] == '_') {
			buf = append(buf, t.data[t.pos])
			t.pos++
		}
		t.token = string(buf)
		return true
	}
	t.pos++
	goto again
}

func (t *tokenizer) ungetToken() { t.ungot = true }

func (t *tokenizer) getPString() ([]byte, bool) {
again:
	if t.pos >= len(t.data) || t.data[t.pos] == 0 {
		return nil, false
	}
	b := t.data[t.pos]
	if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
		t.pos++
		goto again
	}
	if b == '#' || b == ';' {
		for t.pos < len(t.data) && t.data[t.pos] != '\r' && t.data[t.pos] != '\n' {
			t.pos++
		}
		goto again
	}
	if isAlnumByte(b) || b == '_' {
		var content []byte
		for t.pos < len(t.data) && (isAlnumByte(t.data[t.pos]) || t.data[t.pos] == '_') {
			content = append(content, t.data[t.pos])
			t.pos++
		}
		result := make([]byte, 1+len(content))
		result[0] = byte(len(content))
		copy(result[1:], content)
		return result, true
	}
	if b == '"' {
		var content []byte
		t.pos++
		for t.pos < len(t.data) && t.data[t.pos] != '"' {
			if t.data[t.pos] == '\\' {
				t.pos++
				c := t.data[t.pos]
				if c == '\\' || c == '"' {
					content = append(content, c)
					t.pos++
				} else if t.pos+1 < len(t.data) {
					hi := hexNibble(t.data[t.pos])
					lo := hexNibble(t.data[t.pos+1])
					content = append(content, byte(hi<<4|lo))
					t.pos += 2
				}
			} else {
				content = append(content, t.data[t.pos])
				t.pos++
			}
		}
		if t.pos < len(t.data) && t.data[t.pos] == '"' {
			t.pos++
		}
		result := make([]byte, 1+len(content))
		result[0] = byte(len(content))
		copy(result[1:], content)
		return result, true
	}
	return nil, false
}

func (t *tokenizer) parseInt() int {
	if !t.getToken() {
		return 0
	}
	tok := t.token
	if len(tok) > 2 && tok[0] == '0' && (tok[1] == 'x' || tok[1] == 'X') {
		v, _ := strconv.ParseUint(tok[2:], 16, 32)
		return int(v)
	}
	v, _ := strconv.Atoi(tok)
	return v
}

func (t *tokenizer) parseShort() int16 { return int16(t.parseInt()) }

func (t *tokenizer) parseLong() int32 { return int32(t.parseInt()) }

func (t *tokenizer) skipPointLike() {
	h, v := t.parsePoint()
	_ = h
	_ = v
}

// parsePoint reads "H , V" or a single coordinate (mansionsource-compatible).
func (t *tokenizer) parsePoint() (h, v int16) {
	if !t.getToken() {
		return 0, 0
	}
	tok := t.token
	if len(tok) == 0 {
		return 0, 0
	}
	if isDigitChar(tok[0]) || tok[0] == '-' {
		v1, _ := strconv.Atoi(tok)
		h = int16(v1)
		if t.getToken() {
			if t.token == "," {
				t.getToken()
			} else {
				t.ungetToken()
				return h, 0
			}
			tok2 := t.token
			if len(tok2) > 0 && (isDigitChar(tok2[0]) || tok2[0] == '-') {
				v2, _ := strconv.Atoi(tok2)
				v = int16(v2)
			} else {
				t.ungetToken()
			}
		}
	}
	return h, v
}

func pascalToString(ps []byte) string {
	if len(ps) < 1 {
		return ""
	}
	n := int(ps[0])
	if n > len(ps)-1 {
		n = len(ps) - 1
	}
	return string(ps[1 : 1+n])
}

func isAlnumByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }

func isDigitChar(b byte) bool { return b >= '0' && b <= '9' }

func isPunctByte(b byte) bool {
	if b >= 0x21 && b <= 0x7E {
		if isAlnumByte(b) || b == '_' || b == '"' {
			return false
		}
		if strings.ContainsRune("{}[](),", rune(b)) {
			return false
		}
		return true
	}
	return false
}

func hexNibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
