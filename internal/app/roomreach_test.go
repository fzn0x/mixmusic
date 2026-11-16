package app

import (
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/portmap"
	"github.com/fzn0x/mixmusic/internal/portmap/portmaptest"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// fakeRouter starts a fake router of the internet address external, on
// the loopback, which ra's rooms ask: never a real one.
func fakeRouter(t *testing.T, ra *roomApp, external string) *portmaptest.Router {
	t.Helper()
	r, err := portmaptest.New(external)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	ra.opts.Room.Router = &portmap.Client{
		Gateways:   func() ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil },
		ServerPort: r.Port(), SSDP: r.SSDP(), Retry: 20 * time.Millisecond,
	}
	return r
}

func setRooms(t *testing.T, a *App, fn func(s *settings.Settings)) {
	t.Helper()
	if _, err := (&Preferences{a}).Update(fn); err != nil {
		t.Fatal(err)
	}
}

// untilReach waits for the reach of the room a hosts to show what ok
// wants.
func untilReach(t *testing.T, a *App, what string, ok func(RoomReach) bool) RoomReach {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		v, _ := a.Rooms().Reach()
		if ok(v) {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s; the room's reach is %+v", what, v)
		}
	}
}

// listenPort is the port the room of ra listens on.
func listenPort(ra *roomApp) int { return ra.ln.Addr().(*net.TCPAddr).Port }

// TestRoomReachedThroughTheRouter hosts a room that friends outside join:
// the router forwards its port, the invite link carries the router's
// address, and as the switch goes off, or the room ends, the router
// forwards it no more.
func TestRoomReachedThroughTheRouter(t *testing.T) {
	desk := newRoomApp(t, roomtest.New(), "Desk")
	router := fakeRouter(t, desk, "203.0.113.7")
	setRooms(t, desk.App, func(s *settings.Settings) { s.RoomAnywhere = true })
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	port := listenPort(desk)
	v := untilReach(t, desk.App, "the forward", func(v RoomReach) bool { return v.On && !v.Asking && len(v.Addrs) > 0 })
	want := "203.0.113.7:" + strconv.Itoa(port)
	if v.Addrs[0] != want || v.Problem != "" || len(v.Done) == 0 || !strings.Contains(v.Done[0], "203.0.113.7") || !strings.Contains(v.Done[0], "PCP") {
		t.Fatalf("reach %+v", v)
	}
	hosted, _ := desk.Rooms().Hosted()
	in, err := room.ParseLink(hosted.Link)
	if err != nil || len(in.Addrs) != 1 || in.Addrs[0] != want {
		t.Fatalf("the link %q carries %+v, %v", hosted.Link, in.Addrs, err)
	}
	if f := router.Forwards(); len(f) != 1 || int(f[0].Internal) != port || f[0].Client != netip.MustParseAddr("127.0.0.1") {
		t.Fatalf("the router keeps %+v", f)
	}
	desk.Rooms().SetAnywhere(false)
	untilReached(t, "the forward gone", func() bool { return len(router.Forwards()) == 0 })
	if hosted, _ := desk.Rooms().Hosted(); strings.Contains(hosted.Link, "@") {
		t.Fatalf("the link of the network alone: %q", hosted.Link)
	}
	desk.Rooms().SetAnywhere(true)
	untilReached(t, "the forward again", func() bool { return len(router.Forwards()) == 1 })
	desk.Rooms().End()
	untilReached(t, "the forward gone as the room ended", func() bool { return len(router.Forwards()) == 0 })
	if got := desk.settings.Get().RoomAnywhere; !got {
		t.Fatal("the switch wasn't kept")
	}
}

// untilReached polls cond until it holds, for up to 10 s.
func untilReached(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !cond(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s", what)
		}
	}
}

// TestRoomBehindAnotherRouter hosts a room whose router's internet address
// is the provider's shared one: the page says why friends outside can't
// reach it, and the way that's left.
func TestRoomBehindAnotherRouter(t *testing.T) {
	desk := newRoomApp(t, roomtest.New(), "Desk")
	router := fakeRouter(t, desk, "100.70.1.2")
	setRooms(t, desk.App, func(s *settings.Settings) { s.RoomAnywhere = true })
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	v := untilReach(t, desk.App, "the answer", func(v RoomReach) bool { return v.On && !v.Asking })
	if !strings.Contains(v.Problem, "100.70.1.2") || !strings.Contains(v.Problem, "VPN") || len(v.Addrs) != 0 {
		t.Fatalf("reach %+v", v)
	}
	if len(router.Forwards()) != 0 {
		t.Fatalf("the router keeps %+v", router.Forwards())
	}
}

// TestRoomTellsWhenItsAddressMoves hosts a room whose router's internet
// address changes: the host is told to send the new link, and copying it
// takes the change in.
func TestRoomTellsWhenItsAddressMoves(t *testing.T) {
	desk := newRoomApp(t, roomtest.New(), "Desk")
	router := fakeRouter(t, desk, "203.0.113.7")
	var moved atomic.Int32
	t.Cleanup(desk.OnRooms(func(note string) {
		if note == roomMoved {
			moved.Add(1)
		}
	}))
	setRooms(t, desk.App, func(s *settings.Settings) { s.RoomAnywhere = true })
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	untilReach(t, desk.App, "the forward", func(v RoomReach) bool { return len(v.Addrs) > 0 && !v.Moved })
	router.Set(func(r *portmaptest.Router) { r.External = netip.MustParseAddr("203.0.113.99") })
	desk.Rooms().ReachAgain()
	v := untilReach(t, desk.App, "the move", func(v RoomReach) bool { return v.Moved })
	if !strings.HasPrefix(v.Addrs[0], "203.0.113.99:") || moved.Load() != 1 {
		t.Fatalf("reach %+v; told %d times", v, moved.Load())
	}
	desk.Rooms().LinkCopied()
	if v, _ := desk.Rooms().Reach(); v.Moved {
		t.Fatalf("after copying, reach %+v", v)
	}
}
