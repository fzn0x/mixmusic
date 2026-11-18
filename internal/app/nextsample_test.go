package app

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// TestRoomHostJoinsNoTracks starts no next track on the sample after the
// one before while a guest listens to the room this computer hosts: its
// track changes wait for the guests.
func TestRoomHostJoinsNoTracks(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	s := roomStream{a: desk.App}
	if !s.Joins() {
		t.Error("it joins no tracks without a room")
	}
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	if !s.Joins() {
		t.Error("it joins no tracks in a room no guest listens to")
	}
	joinRoom(t, lap, desk)
	if s.Joins() {
		t.Error("it joins tracks while a guest listens")
	}
}
