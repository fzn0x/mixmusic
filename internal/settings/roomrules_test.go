package settings

import "testing"

// TestRoomRulesDefaults has the user accept each song guests add first,
// lets guests control nothing, and lets computers join unasked.
func TestRoomRulesDefaults(t *testing.T) {
	d := Default()
	if d.RoomSongs != RoomSongsAsk || d.RoomGuestControl || d.RoomAskJoin {
		t.Fatalf("defaults %q, control %v, ask %v", d.RoomSongs, d.RoomGuestControl, d.RoomAskJoin)
	}
}

// TestRoomSongsClean keeps the choices there are, and asks first for any
// other.
func TestRoomSongsClean(t *testing.T) {
	for in, want := range map[string]string{"add": "add", "ask": "ask", "off": "off", "": "ask", "always": "ask"} {
		s := Default()
		s.RoomSongs = in
		if got := s.Clean().RoomSongs; got != want {
			t.Errorf("%q cleans to %q, want %q", in, got, want)
		}
	}
}
