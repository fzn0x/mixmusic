package app

import (
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestGuestJoinsAtTheLinksAddress joins, from another network, a room by
// the address its link carries, the one its host typed for invites; drops
// off, and comes back there.
func TestGuestJoinsAtTheLinksAddress(t *testing.T) {
	// Set before the apps, so that it is put back once they closed.
	was := roomRetry
	roomRetry = 20 * time.Millisecond
	t.Cleanup(func() { roomRetry = was })
	desk := newRoomApp(t, roomtest.New(), "Desk")
	// Another network: the guest finds no room on it.
	lap := newRoomApp(t, roomtest.New(), "Lap")
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	addr := "127.0.0.1:" + strconv.Itoa(listenPort(desk))
	setRooms(t, desk.App, func(s *settings.Settings) { s.RoomAddress = "127.0.0.1" })
	desk.Rooms().SetAnywhere(true)
	v := untilReach(t, desk.App, "the address typed", func(v RoomReach) bool { return len(v.Addrs) == 1 })
	if v.Addrs[0] != addr || len(v.Done) == 0 || !strings.Contains(v.Done[0], addr) {
		t.Fatalf("reach %+v", v)
	}
	hosted, _ := desk.Rooms().Hosted()
	if err := lap.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	untilGuest(t, lap.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview })
	lap.Rooms().Join()
	untilGuest(t, lap.App, "the room", func(v GuestView) bool { return v.Step == RoomIn && v.Host == "Desk" })
	desk.ln.kill()
	untilGuest(t, lap.App, "away", func(v GuestView) bool { return v.Step == RoomAway })
	untilGuest(t, lap.App, "back", func(v GuestView) bool { return v.Step == RoomIn })
}

// TestKnockAtAnAddress joins a room at an address typed, with its code.
func TestKnockAtAnAddress(t *testing.T) {
	desk := newRoomApp(t, roomtest.New(), "Desk")
	lap := newRoomApp(t, roomtest.New(), "Lap")
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	hosted, _ := desk.Rooms().Hosted()
	lap.Rooms().KnockAt("127.0.0.1", hosted.Code)
	if v := untilGuest(t, lap.App, "the problem", func(v GuestView) bool { return v.Step == RoomEnded }); !strings.Contains(v.Problem, "port") {
		t.Fatalf("an address without its port: %+v", v)
	}
	lap.Rooms().KnockAt(" 127.0.0.1:"+strconv.Itoa(listenPort(desk))+" ", strings.ToLower(hosted.Code))
	untilGuest(t, lap.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview && v.Host == "Desk" })
	lap.Rooms().Join()
	untilGuest(t, lap.App, "the room", func(v GuestView) bool { return v.Step == RoomIn })
}

// TestNewLinkBringsAGuestBack has a guest drop off where its host's
// address is no longer: the host's new link brings it back at once.
func TestNewLinkBringsAGuestBack(t *testing.T) {
	// Set before the apps, so that it is put back once they closed.
	was := roomRetry
	roomRetry = 20 * time.Millisecond
	t.Cleanup(func() { roomRetry = was })
	desk := newRoomApp(t, roomtest.New(), "Desk")
	lap := newRoomApp(t, roomtest.New(), "Lap")
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	setRooms(t, desk.App, func(s *settings.Settings) { s.RoomAddress = "127.0.0.1" })
	desk.Rooms().SetAnywhere(true)
	untilReach(t, desk.App, "the address typed", func(v RoomReach) bool { return len(v.Addrs) == 1 })
	hosted, _ := desk.Rooms().Hosted()
	if err := lap.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	untilGuest(t, lap.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview })
	lap.Rooms().Join()
	untilGuest(t, lap.App, "the room", func(v GuestView) bool { return v.Step == RoomIn })
	// The host's address went: the guest knows only one where no one
	// answers.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := ln.Addr().String()
	ln.Close()
	lap.room.mu.Lock()
	lap.room.far.link.addrs = []string{dead}
	lap.room.found = nil
	lap.room.mu.Unlock()
	desk.ln.kill()
	untilGuest(t, lap.App, "away", func(v GuestView) bool { return v.Step == RoomAway })
	if err := lap.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	untilGuest(t, lap.App, "back", func(v GuestView) bool { return v.Step == RoomIn })
}

// TestFarProblem tells a guest that couldn't reach a room outside the
// network the likely causes, and one on the network as before.
func TestFarProblem(t *testing.T) {
	far := &room.UnreachableError{Addrs: []string{"203.0.113.7:41234"}, Err: net.ErrClosed}
	if got := roomProblem("Desk", far); !strings.Contains(got, "Couldn't reach Desk over the internet") || !strings.Contains(got, "VPN") {
		t.Fatalf("far: %q", got)
	}
	near := &room.UnreachableError{Addrs: []string{"192.168.1.20:41234"}, Err: &net.OpError{Op: "dial", Err: net.ErrClosed}}
	if got := roomProblem("Desk", near); strings.Contains(got, "internet") {
		t.Fatalf("near: %q", got)
	}
}

// TestPreviewEndedProblem tells a guest whose preview ended what to do.
func TestPreviewEndedProblem(t *testing.T) {
	if got := roomProblem("Desk", room.ErrPreviewEnded); !strings.Contains(got, "Desk's room") || !strings.Contains(got, "again") {
		t.Fatalf("problem %q", got)
	}
}

// TestMadeUpLinkMovesNoGuest has a guest of a room outside its network
// drop off, then open links made of the host's key alone, of another
// secret, one with an address of no room and one with none: its addresses
// stay, and it comes back there. Another host's link changes nothing
// either, as the guest stays in its room.
func TestMadeUpLinkMovesNoGuest(t *testing.T) {
	// Set before the apps, so that it is put back once they closed.
	was := roomRetry
	roomRetry = 20 * time.Millisecond
	t.Cleanup(func() { roomRetry = was })
	desk := newRoomApp(t, roomtest.New(), "Desk")
	lap := newRoomApp(t, roomtest.New(), "Lap")
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	setRooms(t, desk.App, func(s *settings.Settings) { s.RoomAddress = "127.0.0.1" })
	desk.Rooms().SetAnywhere(true)
	untilReach(t, desk.App, "the address typed", func(v RoomReach) bool { return len(v.Addrs) == 1 })
	hosted, _ := desk.Rooms().Hosted()
	if err := lap.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	untilGuest(t, lap.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview })
	lap.Rooms().Join()
	untilGuest(t, lap.App, "the room", func(v GuestView) bool { return v.Step == RoomIn })
	real := lap.linkAddrs(lap.guestRoom().id)
	in, err := room.ParseLink(hosted.Link)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := room.NewSecret()
	for _, addrs := range [][]string{{"203.0.113.66:41234"}, nil} {
		madeUp := room.Invite{Code: in.Code, Host: in.Host, Secret: other, Addrs: addrs}
		if err := lap.Rooms().OpenLink(madeUp.Link()); err != nil {
			t.Fatal(err)
		}
		if got := lap.linkAddrs(lap.guestRoom().id); !slices.Equal(got, real) {
			t.Fatalf("a made-up link of %v left %v, want %v", addrs, got, real)
		}
	}
	// The same secret, no address: the addresses stay too.
	same := in
	same.Addrs = nil
	if err := lap.Rooms().OpenLink(same.Link()); err != nil {
		t.Fatal(err)
	}
	if got := lap.linkAddrs(lap.guestRoom().id); !slices.Equal(got, real) {
		t.Fatalf("a link of no address left %v", got)
	}
	// Another host's link, while in the room.
	stranger := newRoomApp(t, roomtest.New(), "Stranger")
	if err := stranger.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	sh, _ := stranger.Rooms().Hosted()
	if err := lap.Rooms().OpenLink(sh.Link); err != nil {
		t.Fatal(err)
	}
	if got := lap.linkAddrs(lap.guestRoom().id); !slices.Equal(got, real) {
		t.Fatalf("another host's link left %v", got)
	}
	desk.ln.kill()
	untilGuest(t, lap.App, "back", func(v GuestView) bool { return v.Step == RoomIn && v.Host == "Desk" })
}
