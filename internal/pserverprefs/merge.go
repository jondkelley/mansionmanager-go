// Package pserverprefs rewrites YPMYEXTADDR / YPMYEXTPORT lines in pserver.prefs text
// to match Palace directory (YP) registration, using the same quoted-string encoding as mansionsource-go.
package pserverprefs

import (
	"fmt"
	"strings"
)

// MergeYPAnnounce removes existing YPMYEXTADDR / YPMYEXTPORT lines and appends new ones
// derived from host and port. Empty host and port<=0 remove those directives entirely.
func MergeYPAnnounce(content, host string, port int) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines)+2)
	for _, line := range lines {
		tl := strings.TrimSpace(line)
		if tl == "" {
			out = append(out, line)
			continue
		}
		u := strings.ToUpper(tl)
		if strings.HasPrefix(u, "YPMYEXTADDR") || strings.HasPrefix(u, "YPMYEXTPORT") {
			continue
		}
		out = append(out, line)
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	var b strings.Builder
	if len(out) > 0 {
		b.WriteString(strings.Join(out, "\n"))
	}
	host = strings.TrimSpace(host)
	if host != "" {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("YPMYEXTADDR ")
		b.WriteString(palaceQuoted(host))
	}
	if port > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "YPMYEXTPORT %d", port)
	}
	s := b.String()
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

// MergeServerNameSysop removes existing SERVERNAME / SYSOP directives and appends new ones
// using Palace quoted-string encoding. Empty serverName or sysop omits that directive.
func MergeServerNameSysop(content, serverName, sysop string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines)+2)
	for _, line := range lines {
		tl := strings.TrimSpace(line)
		if tl == "" {
			out = append(out, line)
			continue
		}
		u := strings.ToUpper(tl)
		if strings.HasPrefix(u, "SERVERNAME") || strings.HasPrefix(u, "SYSOP") {
			continue
		}
		out = append(out, line)
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	var b strings.Builder
	if len(out) > 0 {
		b.WriteString(strings.Join(out, "\n"))
	}
	serverName = strings.TrimSpace(serverName)
	sysop = strings.TrimSpace(sysop)
	if serverName != "" {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("SERVERNAME ")
		b.WriteString(palaceQuoted(serverName))
	}
	if sysop != "" {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("SYSOP ")
		b.WriteString(palaceQuoted(sysop))
	}
	s := b.String()
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

// palaceQuoted formats s as a Palace script double-quoted string (p-string style).
func palaceQuoted(s string) string {
	var w strings.Builder
	w.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= ' ' && c <= '~' && c != '\\' && c != '"' {
			w.WriteByte(c)
		} else {
			switch c {
			case '\\', '"':
				w.WriteByte('\\')
				w.WriteByte(c)
			default:
				fmt.Fprintf(&w, "\\%02X", c)
			}
		}
	}
	w.WriteByte('"')
	return w.String()
}
