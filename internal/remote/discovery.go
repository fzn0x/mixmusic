package remote

import (
	"context"
	"strconv"

	"github.com/fzn0x/mixmusic/internal/dnssd"
)

// Service is the DNS-SD service the computers with remote control on
// announce.
const Service = "_mixmusic._tcp.local."

// Discovery finds the computers with remote control on, and announces
// this one: MDNS on the network, remotetest's fake in tests.
type Discovery interface {
	// Announce has the others find this computer, until stop.
	Announce(a Announcement) (stop func(), err error)
	// Browse finds the computers that announce themselves now.
	Browse(ctx context.Context) ([]Found, error)
}

// Announcement is what a computer tells of itself: its ID, its name, and
// the port its server listens on.
type Announcement struct {
	ID, Name string
	Port     int
}

// Found is a computer found: its ID, its name, as it tells them, and the
// address of its server. What discovery tells can't be trusted: the key
// decides who answers there.
type Found struct {
	ID, Name, Addr string
}

// MDNS is Discovery on the network, by DNS-SD over mDNS.
type MDNS struct{}

// Announce answers the questions about this computer on mDNS's port, on
// each of the computer's networks, until stop. Its instance and its host
// are named after its ID, which no other computer has.
func (MDNS) Announce(a Announcement) (func(), error) {
	conn, err := dnssd.ListenMulticast()
	if err != nil {
		return nil, err
	}
	r := &dnssd.Responder{Service: Service, Instance: "MixMusic " + a.ID, Host: "mixmusic-" + a.ID + ".local.", Port: a.Port,
		Text: []string{"id=" + a.ID, "name=" + truncate(a.Name, 200), "v=" + strconv.Itoa(version)}}
	go r.Serve(conn)
	return func() { conn.Close() }, nil
}

// Browse asks the network for the computers with remote control on, for a
// second.
func (MDNS) Browse(ctx context.Context) ([]Found, error) {
	insts, err := dnssd.Browse(ctx, Service, dnssd.BrowseOptions{})
	if err != nil {
		return nil, err
	}
	return foundOf(insts), nil
}

// foundOf returns the computers of the instances found that speak this
// version of the protocol.
func foundOf(insts []dnssd.Instance) []Found {
	var out []Found
	for _, in := range insts {
		id := in.Text["id"]
		if id == "" || in.Text["v"] != strconv.Itoa(version) {
			continue
		}
		out = append(out, Found{ID: cleanText(id, 32), Name: cleanName(in.Text["name"]), Addr: in.Addr()})
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}
