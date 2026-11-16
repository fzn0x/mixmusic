package settings

import (
	"strings"
	"unicode"
)

// cleanRoomPort is port, the port rooms listen on, when it is one of 1024
// to 65535, or 0, to be picked again.
func cleanRoomPort(port int) int {
	if port < 1024 || port > 65535 {
		return 0
	}
	return port
}

// maxRoomAddress bounds the address for invites: a name of the DNS and a
// port.
const maxRoomAddress = 260

// cleanRoomAddress is the address for invites without spaces or control
// characters, of at most maxRoomAddress bytes, or "" for one that's
// longer. The app reads what's left as an address.
func cleanRoomAddress(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == unicode.ReplacementChar {
			return -1
		}
		return r
	}, s)
	if len(s) > maxRoomAddress {
		return ""
	}
	return s
}
