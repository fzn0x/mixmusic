package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// roomTiming is the rooms' timing in these tests.
var roomTiming = room.Timing{Ping: 100 * time.Millisecond, Silence: 3 * time.Second, Gate: time.Second, Lead: 50 * time.Millisecond,
	Wait: time.Second, Changes: 10 * time.Millisecond}

// newRoomShell is a test shell of the computer named name, whose rooms are
// on the network n.
func newRoomShell(t *testing.T, n *roomtest.Network, name string) *shell {
	t.Helper()
	s := newRemoteShellOf(t, remotetest.New(), name, func(o *app.Options) {
		o.Room = &app.RoomOptions{Discovery: n, Listen: roomtest.Listen, Timing: roomTiming}
	})
	t.Cleanup(func() {
		s.a.Rooms().End()
		s.a.Rooms().Leave()
	})
	return s
}

// foundRoom waits for the room of the host named name.
func foundRoom(t *testing.T, s *shell, name string) app.FoundRoom {
	t.Helper()
	s.a.Rooms().Find()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		for _, f := range s.a.Rooms().Found() {
			if f.Name == name {
				return f
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s's room wasn't found: %+v", name, s.a.Rooms().Found())
		}
	}
}

// TestRoomJoinFromDevicesMenu lists a room in the devices menu, whose page
// takes its code, shows who hosts it, and joins it; the host's page lists
// the guest, and removes it, which the guest's page tells.
func TestRoomJoinFromDevicesMenu(t *testing.T) {
	n := roomtest.New()
	desk := newRoomShell(t, n, "Desk")
	lap := newRoomShell(t, n, "Lap")
	dt, lt := tester(t, desk), tester(t, lap)
	if _, ok := lt.Find("Cast to a device"); ok {
		t.Fatal("a devices button with nothing on the network")
	}
	if err := desk.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	hosted, _ := desk.a.Rooms().Hosted()
	f := foundRoom(t, lap, "Desk")
	lt.Frame()
	click(t, lap, lt, "Cast to a device")
	want := []string{"This computer", "-", "Start a room", "Desk's room", "Join a room…"}
	if got := lt.Menu(); !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
	if err := lt.ChooseMenuItem("Desk's room"); err != nil {
		t.Fatal(err)
	}
	lap.drain()
	lt.Frame()
	lt.Advance(time.Second)
	if got, want := lap.router.Path(), roomPath(f.ID); got != want {
		t.Errorf("went to %q, want %q", got, want)
	}
	until(t, lap, lt, "the code asked", func() bool { return lt.HasText("Type the code Desk's room shows, of 6 letters and digits.") })
	lt.Type(strings.ToLower(hosted.Code))
	lap.drain()
	lt.Frame()
	click(t, lap, lt, "join")
	untilBoth(t, desk, dt, lap, lt, "the preview", func() bool { return lt.HasText("Desk hosts this room.") })
	if !lt.HasText("0 listening. Joining plays what Desk plays, here, in step; your queue comes back as you leave.") {
		t.Errorf("the preview shows %q", lt.Texts())
	}
	click(t, lap, lt, "join")
	untilBoth(t, desk, dt, lap, lt, "in the room", func() bool { return lt.HasText("In Desk's room") && lt.HasText("Listening: Desk, Lap") })

	// The host's page shows the code, and lists the guest.
	show(desk, dt, roomsPath)
	untilBoth(t, desk, dt, lap, lt, "the guest", func() bool {
		return dt.HasText(room.ShowCode(hosted.Code)) && dt.HasText("Lap") && dt.HasText("Lap joined · "+time.Now().Format("15:04"))
	})
	click(t, desk, dt, "Remove Lap")
	untilBoth(t, desk, dt, lap, lt, "removed", func() bool { return lt.HasText("Desk removed you from the room.") })
	if g := desk.a.Rooms(); len(func() []app.RoomMember { h, _ := g.Hosted(); return h.Guests }()) != 0 {
		t.Error("the host keeps the guest it removed")
	}
	click(t, lap, lt, "close")
	untilBoth(t, desk, dt, lap, lt, "the code asked again", func() bool { return lt.HasText("Type the code Desk's room shows, of 6 letters and digits.") })
	click(t, desk, dt, "end room")
	untilBoth(t, desk, dt, lap, lt, "the room ended", func() bool { return n.Announced() == 0 && dt.HasText("start a room") })
}

// TestRoomSettingsStartARoom starts a room from Settings › Desktop, which
// shows its page with its code, and keeps the name in rooms.
func TestRoomSettingsStartARoom(t *testing.T) {
	n := roomtest.New()
	s := newRoomShell(t, n, "Desk")
	tt := tester(t, s)
	show(s, tt, "/settings/desktop")
	for _, text := range []string{"listen with friends", "Your name in rooms", "Follow the host's volume"} {
		if !tt.HasText(text) {
			t.Fatalf("Settings › Desktop doesn't show %q: %q", text, tt.Texts())
		}
	}
	click(t, s, tt, "start a room")
	until(t, s, tt, "the room's page", func() bool {
		h, ok := s.a.Rooms().Hosted()
		return ok && s.router.Path() == roomsPath && tt.HasText(room.ShowCode(h.Code)) && tt.HasText("No one joined yet.")
	})
	if n.Announced() != 1 {
		t.Fatal("the room isn't announced")
	}
	was := copyText
	var copied string
	copyText = func(text string) { copied = text }
	t.Cleanup(func() { copyText = was })
	click(t, s, tt, "copy invite link")
	if h, _ := s.a.Rooms().Hosted(); copied != h.Link || !strings.HasPrefix(copied, "mixmusic://join/"+h.Code+"#") {
		t.Errorf("copied %q", copied)
	}
	show(s, tt, "/settings/desktop")
	until(t, s, tt, "the room open", func() bool { return tt.HasText("Your room is open") })
}

// TestRoomPasteInviteLink opens a room from its invite link, pasted on the
// rooms' page.
func TestRoomPasteInviteLink(t *testing.T) {
	n := roomtest.New()
	desk := newRoomShell(t, n, "Desk")
	lap := newRoomShell(t, n, "Lap")
	dt, lt := tester(t, desk), tester(t, lap)
	if err := desk.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	hosted, _ := desk.a.Rooms().Hosted()
	foundRoom(t, lap, "Desk")
	show(lap, lt, roomsPath)
	if !lt.HasText("Desk's room") {
		t.Fatalf("the rooms on the network don't show: %q", lt.Texts())
	}
	click(t, lap, lt, "Invite link")
	lt.Type(hosted.Link)
	lap.drain()
	lt.Frame()
	click(t, lap, lt, "open the link")
	untilBoth(t, desk, dt, lap, lt, "the preview", func() bool { return lt.HasText("Desk hosts this room.") })
}

// TestRoomNewCodeButton gives the room a new code from its page, which
// shows it.
func TestRoomNewCodeButton(t *testing.T) {
	n := roomtest.New()
	s := newRoomShell(t, n, "Desk")
	tt := tester(t, s)
	if err := s.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	old, _ := s.a.Rooms().Hosted()
	show(s, tt, roomsPath)
	click(t, s, tt, "new code")
	until(t, s, tt, "the new code", func() bool {
		h, _ := s.a.Rooms().Hosted()
		return h.Code != old.Code && tt.HasText(room.ShowCode(h.Code)) && !tt.HasText(room.ShowCode(old.Code))
	})
}
