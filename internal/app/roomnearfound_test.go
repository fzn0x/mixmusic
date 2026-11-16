package app

import (
	"context"
	"strconv"
	"testing"

	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// manyNetwork is the network n as something on it answers for each host:
// with addresses of the internet first, then its own, then a crowd more.
type manyNetwork struct{ *roomtest.Network }

func (m manyNetwork) Browse(ctx context.Context) ([]room.Found, error) {
	found, err := m.Network.Browse(ctx)
	for i := range found {
		own := found[i].Addr
		found[i].Addr = "203.0.113.7:4000"
		found[i].Others = []string{"198.51.100.1:4000", own}
		for j := range 20 {
			found[i].Others = append(found[i].Others, "127.0.0."+strconv.Itoa(j+2)+":4000")
		}
	}
	found = append(found, room.Found{ID: "elsewhere", Name: "Elsewhere", Addr: "203.0.113.9:4000"})
	return found, err
}

// TestRoomsFoundAtTheirNetworksAddresses keeps, of the rooms found, the
// addresses of this computer's networks alone, a few, its own first, and
// leaves out a room found at none.
func TestRoomsFoundAtTheirNetworksAddresses(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	lap := newRoomApp(t, n, "Lap")
	lap.opts.Room.Discovery = manyNetwork{n}
	lap.browseRooms()
	hostID := desk.opts.Remote.Identity.Fingerprint().ID()
	f, ok := lap.foundRoom(hostID, false)
	own := ""
	if found, _ := n.Browse(context.Background()); len(found) == 1 {
		own = found[0].Addr
	}
	if !ok || f.Addr != own || len(f.Others) != room.MaxAddrs || f.Others[0] != "127.0.0.2:4000" {
		t.Fatalf("Desk's room found at %+v, its own %s", f, own)
	}
	if f, ok := lap.foundRoom("elsewhere", false); ok {
		t.Fatalf("a room of the internet found: %+v", f)
	}
}
