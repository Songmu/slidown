package pptx

import "strconv"

func itoa(i int) string     { return strconv.Itoa(i) }
func itoa64(i int64) string { return strconv.FormatInt(i, 10) }

// escapeXML escapes the five XML predefined entities for use in text content
// and attribute values.
func escapeXML(s string) string {
	needs := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>', '"', '\'':
			needs = true
		}
		if needs {
			break
		}
	}
	if !needs {
		return s
	}
	var b []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			b = append(b, "&amp;"...)
		case '<':
			b = append(b, "&lt;"...)
		case '>':
			b = append(b, "&gt;"...)
		case '"':
			b = append(b, "&quot;"...)
		case '\'':
			b = append(b, "&apos;"...)
		default:
			b = append(b, s[i])
		}
	}
	return string(b)
}
