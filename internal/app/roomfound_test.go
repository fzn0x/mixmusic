package app

import (
	"context"
	"testing"

	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// refuser takes connections and closes them at once, before TLS, as a
// room of the network does those of addresses outside it, as a VPN's.
func refuser(t *testing.T) string {
	t.Helper()
	ln, err := roomtest.Listen()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return ln.Addr().String()
}

// vpnNetwork is the network n as a computer on a VPN too finds it: each
// room first at an address that refuses, as its host's address on the VPN
// does, then at its own, unless others is false.
type vpnNetwork struct {
	*roomtest.Network
	refuses string
	others  bool
}

func (v vpnNetwork) Browse(ctx context.Context) ([]room.Found, error) {
	found, err := v.Network.Browse(ctx)
	for i := range found {
		if v.others {
			found[i].Others = []string{found[i].Addr}
		}
		found[i].Addr = v.refuses
	}
	return found, err
}

// onVPN has a find the rooms of n as a computer on a VPN too does.
func onVPN(t *testing.T, a *roomApp, n *roomtest.Network, others bool) {
	t.Helper()
	a.opts.Room.Discovery = vpnNetwork{Network: n, refuses: refuser(t), others: others}
}

// TestRoomFoundAtEveryAddress joins a room found at an address that
// refuses first, as its host's on a VPN, and at its own after: by its code
// from the devices menu, and by its link, each address is tried, and the
// guest gets in.
func TestRoomFoundAtEveryAddress(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	lap := newRoomApp(t, n, "Lap")
	onVPN(t, lap, n, true)
	joinRoom(t, lap, desk)
	lap.Rooms().Leave()
	until(t, "Lap gone", func() bool { h, _ := desk.Rooms().Hosted(); return len(h.Guests) == 0 })
	hosted, _ := desk.Rooms().Hosted()
	if err := lap.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	untilGuest(t, lap.App, "the preview by the link", func(v GuestView) bool { return v.Step == RoomPreview })
}
