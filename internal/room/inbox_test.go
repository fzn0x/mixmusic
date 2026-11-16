package room

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// TestSentKeepsAGuestsFields reads what a guest's messages tell, and
// builds none of the fields of the host's that a guest's message has.
func TestSentKeepsAGuestsFields(t *testing.T) {
	track := &remote.QueueTrack{Source: "youtube", ID: "yt:aaaaaaaaaaa", Title: "A song"}
	want := message{Type: "add", Seq: 7, Track: track, Sent: time.Second, Do: Next, Port: 4000, Reason: "can't", OK: true}
	with := want
	with.Seats = make([]Seat, 1000)
	with.Members = []string{strings.Repeat("x", 1000)}
	with.State = &State{Entry: "e1"}
	data, err := json.Marshal(with)
	if err != nil {
		t.Fatal(err)
	}
	var got sent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	m := got.message()
	if m.Type != want.Type || m.Seq != want.Seq || m.Track == nil || m.Track.ID != track.ID || m.Track.Title != track.Title || m.Sent != want.Sent || m.Do != want.Do ||
		m.Port != want.Port || m.Reason != want.Reason || m.OK != want.OK {
		t.Fatalf("read %+v", m)
	}
	if m.Seats != nil || m.Members != nil || m.State != nil {
		t.Fatalf("built the host's fields: %+v", m)
	}
}
