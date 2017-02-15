package utils

import "strings"

// clearName replaces non-allowed characters.
func ClearName(txt string, allowColon bool, replaceRune rune) string {
	if replaceRune == 0 {
		replaceRune = -1
	}
	i := -1
	return strings.Map(
		func(r rune) rune {
			i++
			if r == '.' {
				return '_'
			}
			if 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || r == '_' {
				return r
			}
			if allowColon && r == ':' {
				return r
			}
			if i > 0 && '0' <= r && r <= '9' {
				return r
			}
			return replaceRune
		},
		txt)
}
