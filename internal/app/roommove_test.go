package app

import (
	"net"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// answerHost has the user of g answer the host's ask that it host the
// room, once it asks.
func answerHost(t *testing.T, g *roomApp, yes bool) {
	t.Helper()
	until(t, g.opts.Remote.Name+" asked to host", func() bool { _, ok := g.Rooms().HostAsk(); return ok })
	g.Rooms().AnswerHost(yes)
}

// TestRoomMakeHost moves the room to a guest the other reaches: it hosts
// it, its player playing the room's queue, and the other guest and the old
// host are guests of it, in step with it.
func TestRoomMakeHost(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	tab := newRoomApp(t, n, "Tab")
	deskNotes, lapNotes := roomNotes(t, desk.App), roomNotes(t, lap.App)
	dp := desk.GoPlayer()
	dp.PlayTracks(tracksOf("a", "b", "c"), 0)
	waitPlayer(t, dp, "a playing", func(s player.State) bool { return s.Playing })
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	tab.browseRooms()
	hosted, _ := desk.Rooms().Hosted()
	if err := tab.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	untilGuest(t, tab.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview })
	tab.Rooms().Join()
	untilGuest(t, tab.App, "the room", func(v GuestView) bool { return v.Step == RoomIn && v.HasState })
	lapKey := lap.opts.Remote.Identity.Fingerprint().String()
	if !desk.Rooms().CanHost(lapKey) {
		t.Fatal("Lap can't host")
	}
	if err := desk.Rooms().MakeHost(lapKey); err != nil {
		t.Fatal(err)
	}
	answerHost(t, lap, true)
	toldNote(t, lapNotes, "You host the room now")
	toldNote(t, deskNotes, "The room moved to Lap's computer: you're a guest in it now")
	if _, ok := lap.Rooms().Hosted(); !ok {
		t.Fatal("Lap hosts no room")
	}
	if _, ok := desk.Rooms().Hosted(); ok {
		t.Fatal("Desk still hosts the room")
	}
	lp := lap.GoPlayer()
	waitPlayer(t, lp, "the room's queue on Lap", func(s player.State) bool {
		cur, ok := s.Current()
		return ok && cur.Track.ID == vid("a") && len(s.Queue) == 3 && s.Playing
	})
	for _, g := range []*roomApp{desk, tab} {
		untilGuest(t, g.App, g.opts.Remote.Name+" in Lap's room", func(v GuestView) bool {
			return v.Step == RoomIn && v.Host == "Lap" && v.HasState && v.State.Entry != ""
		})
	}
	until(t, "the old host in step with Lap", func() bool { return off(lap, desk) < room.InStep })
	until(t, "Tab in step with Lap", func() bool { return off(lap, tab) < room.InStep })
	h, _ := lap.Rooms().Hosted()
	if len(h.Guests) != 2 {
		t.Fatalf("Lap's guests %+v", h.Guests)
	}
	// The old host's own queue, the room's as it moved, comes back as it
	// leaves.
	desk.Rooms().Leave()
	waitPlayer(t, dp, "the old host's queue back", func(s player.State) bool { return len(s.Queue) == 3 })
}

// TestRoomProblemMoving tells a computer that knocks as the room moves to
// try again.
func TestRoomProblemMoving(t *testing.T) {
	if got := roomProblem("Desk", room.ErrRoomMoves); got != "Desk's room is moving to another computer: try again in a moment." {
		t.Fatalf("problem %q", got)
	}
}

// shut listens as a room does, and shuts each connection at once.
type shut struct{ net.Listener }

func (s shut) Accept() (net.Conn, error) {
	for {
		c, err := s.Listener.Accept()
		if err != nil {
			return nil, err
		}
		c.Close()
	}
}

// TestRoomMakeHostUnreached keeps the room where it is when the host can't
// reach the one it would move to: it is told so.
func TestRoomMakeHostUnreached(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	// Lap's rooms turn every connection away, as a firewall does.
	listen := lap.opts.Room.Listen
	lap.opts.Room.Listen = func() (net.Listener, error) {
		ln, err := listen()
		if err != nil {
			return nil, err
		}
		return shut{ln}, nil
	}
	deskNotes := roomNotes(t, desk.App)
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	if err := desk.Rooms().MakeHost(lap.opts.Remote.Identity.Fingerprint().String()); err != nil {
		t.Fatal(err)
	}
	answerHost(t, lap, true)
	until(t, "the move refused", func() bool {
		for _, note := range deskNotes() {
			if strings.HasPrefix(note, "The room stays here: this computer can't reach Lap's") {
				return true
			}
		}
		return false
	})
	if _, ok := desk.Rooms().Hosted(); !ok {
		t.Fatal("the room moved")
	}
	if _, ok := lap.Rooms().Hosted(); ok {
		t.Fatal("Lap hosts a room")
	}
	if v, _ := lap.Rooms().Guest(); v.Step != RoomIn {
		t.Fatalf("Lap is %+v", v)
	}
}
