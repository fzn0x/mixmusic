package settings

import (
	"strings"
	"unicode"
	"unicode/utf16"
)

// The lines of the Discord status of a new install, as they were before
// they could be chosen: the title, then the artists.
const (
	DefaultDiscordFirstLine  = "{title}"
	DefaultDiscordSecondLine = "{artist}"
)

// DiscordLineMax is the longest line of the Discord status, as Discord
// counts characters, in UTF-16 code units: it shows no more.
const DiscordLineMax = 128

// cleanDiscordLine is a line of the Discord status on one line, without the
// spaces around it, cut to DiscordLineMax.
func cleanDiscordLine(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	n := 0
	for i, r := range s {
		if n += utf16.RuneLen(r); n > DiscordLineMax {
			return strings.TrimRightFunc(s[:i], unicode.IsSpace)
		}
	}
	return s
}
