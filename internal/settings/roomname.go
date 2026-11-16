package settings

import (
	"strings"
	"unicode"
)

// maxRoomName bounds a name in rooms, in characters.
const maxRoomName = 50

// cleanRoomName is a name in rooms without control characters or the
// spaces around it, of at most maxRoomName characters: "" stays "", the
// computer's name.
func cleanRoomName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == unicode.ReplacementChar {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxRoomName {
		s = strings.TrimSpace(string(r[:maxRoomName]))
	}
	return s
}
