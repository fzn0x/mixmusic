package settings

import (
	"strings"
	"testing"
)

// TestRoomNameIsCleaned keeps a name in rooms to 50 characters, without
// control characters or the spaces around it, and leaves none as none.
func TestRoomNameIsCleaned(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "",
		"  Ana  ":               "Ana",
		"Ana\x07's\nlaptop":     "Ana'slaptop",
		strings.Repeat("é", 60): strings.Repeat("é", 50),
		strings.Repeat("a", 50): strings.Repeat("a", 50),
	} {
		s := Settings{Desktop: Desktop{RoomName: in}}.Clean()
		if s.RoomName != want {
			t.Errorf("%q cleans to %q, want %q", in, s.RoomName, want)
		}
	}
	if Default().RoomFollowVolume {
		t.Error("a guest follows the host's volume at first")
	}
}
