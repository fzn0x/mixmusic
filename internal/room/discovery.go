package room

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/fzn0x/mixmusic/internal/dnssd"
)

// Service is the DNS-SD service the computers that host a room announce,
// apart from remote control's.
const Service = "_mixmusic-room._tcp.local."

// Discovery finds the rooms on the network, and announces this one's:
// MDNS on the network, roomtest's fake in tests.
type Discovery interface {
	// Announce has the others find the room, until stop.
	Announce(a Announcement) (stop func(), err error)
	// Browse finds the rooms announced now.
	Browse(ctx context.Context) ([]Found, error)
}

// Announcement is what a room tells of itself: the ID of its host's key,
// the host's name in rooms, and the port its server listens on. It tells
// neither its code nor what plays: those are for its guests.
type Announcement struct {
	ID, Name string
	Port     int
}

// Found is a room found: its host's ID and name, as it tells them, and the
// address of its server. What discovery tells can't be trusted: the code,
// or the link's key, decides who answers there.
type Found struct {
	ID, Name, Addr string
}

// MDNS is Discovery on the network, by DNS-SD over mDNS.
type MDNS struct{}

// Announce answers the questions about the room on mDNS's port, on each of
// the computer's networks, until stop, on a socket it keeps working: one
// opened before macOS let MixMusic use the local network is put back once
// it may (#725). Its instance and its host are named after its host's ID,
// which no other computer has.
func (MDNS) Announce(a Announcement) (func(), error) {
	return responderOf(a).Keep(func() (net.PacketConn, error) {
		conn, err := dnssd.ListenMulticast()
		if err != nil {
			return nil, err
		}
		return conn, nil
	})
}

// responderOf is what answers the questions about the room a announces.
func responderOf(a Announcement) *dnssd.Responder {
	return &dnssd.Responder{Service: Service, Instance: "MixMusic room " + a.ID, Host: "mixmusic-room-" + a.ID + ".local.", Port: a.Port,
		Text: []string{"id=" + a.ID, "name=" + truncate(CleanName(a.Name), 200), "v=" + strconv.Itoa(version)}}
}

// Browse asks the network for the rooms, for a second.
func (MDNS) Browse(ctx context.Context) ([]Found, error) {
	insts, err := dnssd.Browse(ctx, Service, dnssd.BrowseOptions{})
	if err != nil {
		return nil, err
	}
	return foundOf(insts), nil
}

// foundOf returns the rooms of the instances found that speak this
// version of the protocol.
func foundOf(insts []dnssd.Instance) []Found {
	var out []Found
	for _, in := range insts {
		id := cleanEntry(in.Text["id"])
		if id == "" || len(id) > 32 || in.Text["v"] != strconv.Itoa(version) {
			continue
		}
		out = append(out, Found{ID: strings.ToLower(id), Name: CleanName(in.Text["name"]), Addr: in.Addr()})
	}
	return out
}

// truncate is s in at most n bytes, cut between characters.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}
