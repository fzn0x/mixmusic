package room

import (
	"slices"
	"testing"
)

// TestResponderOfTheRoom answers for the room's instance, of its host's
// ID, its port, and a TXT record of its ID, its host's name and the
// protocol's version, which MDNS.Announce keeps answering (Keep).
func TestResponderOfTheRoom(t *testing.T) {
	r := responderOf(Announcement{ID: "abcdef", Name: " Desk\x07 ", Port: 4000})
	if r.Service != Service || r.Instance != "MixMusic room abcdef" || r.Host != "mixmusic-room-abcdef.local." || r.Port != 4000 {
		t.Fatalf("responder %+v", r)
	}
	if want := []string{"id=abcdef", "name=Desk", "v=1"}; !slices.Equal(r.Text, want) {
		t.Fatalf("TXT %q, want %q", r.Text, want)
	}
}
