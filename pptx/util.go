package pptx

import (
	"strconv"
	"strings"
)

func itoa(i int) string     { return strconv.Itoa(i) }
func itoa64(i int64) string { return strconv.FormatInt(i, 10) }

// escapeXML escapes the five XML predefined entities for use in text content
// and attribute values.
func escapeXML(s string) string {
	needs := false
	for _, r := range s {
		switch r {
		case '&', '<', '>', '"', '\'':
			needs = true
		}
		if isForbiddenXMLRune(r) {
			needs = true
		}
		if needs {
			break
		}
	}
	if !needs {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isForbiddenXMLRune(r) {
			continue
		}
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isForbiddenXMLRune(r rune) bool {
	if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
		return true
	}
	if r >= 0xD800 && r <= 0xDFFF {
		return true
	}
	return r == 0xFFFE || r == 0xFFFF
}
