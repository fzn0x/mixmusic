package settings

import "testing"

// TestRoomAnywhereCleans keeps a port for rooms of 1024 to 65535 alone, and
// an address for invites without spaces.
func TestRoomAnywhereCleans(t *testing.T) {
	for in, want := range map[int]int{0: 0, 80: 0, 1023: 0, 1024: 1024, 41234: 41234, 65535: 65535, 65536: 0, -5: 0} {
		if got := cleanRoomPort(in); got != want {
			t.Errorf("cleanRoomPort(%d) = %d, want %d", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"":                        "",
		" desk.tail1234.ts.net ":  "desk.tail1234.ts.net",
		"203.0.113.7:41234\n":     "203.0.113.7:41234",
		"[2001:db8::5]:41234":     "[2001:db8::5]:41234",
		string(make([]byte, 300)): "",
	} {
		if got := cleanRoomAddress(in); got != want {
			t.Errorf("cleanRoomAddress(%q) = %q, want %q", in, got, want)
		}
	}
}
