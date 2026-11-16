package app

// The guest's side of friends anywhere (#562): an invite link carries where
// its room is reached from outside the host's network, which a knock, and
// a guest that comes back, dial beside the address found on the network
// (room.DialFirst); Join a room… knocks at an address typed, with the
// room's code; and a guest that dropped off takes a new link of its host's
// to come back at the host's new address.

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/room"
)

// maxLinks bounds the rooms of invite links whose addresses are kept.
const maxLinks = 16

// keepLinkAddrs keeps addrs, where the room of the host id is reached from
// outside its network, as its invite link carries them: none forgets them.
func (a *App) keepLinkAddrs(id string, addrs []string) {
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	far := &r.far
	if len(addrs) == 0 {
		delete(far.links, id)
		return
	}
	if far.links == nil || len(far.links) >= maxLinks {
		far.links = map[string][]string{}
	}
	far.links[id] = slices.Clone(addrs)
}

// linkAddrs are the addresses of the room of the host id that its invite
// link carried.
func (a *App) linkAddrs(id string) []string {
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.far.links[id])
}

// linkRoom is the room of the host id as its invite link tells it, when it
// carried addresses: found where the network didn't.
func (a *App) linkRoom(id string) (room.Found, bool) {
	addrs := a.linkAddrs(id)
	if len(addrs) == 0 {
		return room.Found{}, false
	}
	return room.Found{ID: id, Name: "The room", Addr: addrs[0]}, true
}

// roomDial connects to the room this computer knocks at, or comes back
// to, at addr, and at the addresses of its invite link beside it, keeping
// the first that answers. A room of the network alone is dialed as
// before.
func (a *App) roomDial(ctx context.Context, network, addr string) (net.Conn, error) {
	var link []string
	if g := a.guestRoom(); g != nil {
		link = a.linkAddrs(g.id)
	}
	if len(link) == 0 {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	addrs := []string{addr}
	for _, x := range link {
		if !slices.Contains(addrs, x) {
			addrs = append(addrs, x)
		}
	}
	return room.DialFirst(ctx, nil, addrs)
}

// KnockAt knocks at the room at addr, an address typed with its port, as
// a VPN's or the host's router's, with the code the user typed.
func (rs *Rooms) KnockAt(addr, code string) {
	a := rs.a
	at, ok := room.InviteAddr(addr, 0)
	if !ok {
		a.startKnock(strings.TrimSpace(addr), nil, knockBy{}, errors.New("A room's address has its port, as 203.0.113.7:41234."))
		return
	}
	parsed, ok := room.ParseCode(code)
	if !ok {
		a.startKnock(at, nil, knockBy{}, errors.New("A room's code is 6 letters and digits."))
		return
	}
	a.keepLinkAddrs(at, []string{at})
	a.startKnock(at, nil, knockBy{password: []byte(parsed)}, nil)
}

// backByInvite takes a new invite link of the host this computer is a
// guest of: it keeps the link's addresses, and a guest that dropped off
// tries to come back there at once, as the host's address changed. It
// reports whether the link was the host's.
func (a *App) backByInvite(in room.Invite) bool {
	r := &a.room
	r.mu.Lock()
	g := r.guest
	mine := g != nil && g.hasKey && g.key == in.Host && g.step != RoomEnded && g.step != RoomKnocking && g.step != RoomPreview
	away := mine && g.step == RoomAway
	if away && g.retry != nil {
		g.retry.Stop()
	}
	var id string
	if mine {
		id = g.id
	}
	r.mu.Unlock()
	if !mine {
		return false
	}
	a.keepLinkAddrs(id, in.Addrs)
	if away {
		go a.comeBack(g)
	}
	return true
}

// farUnreachable reports whether err tells that a room's addresses outside
// the network didn't answer.
func farUnreachable(err error) bool {
	var ue *room.UnreachableError
	if !errors.As(err, &ue) {
		return false
	}
	for _, addr := range ue.Addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		ip, err := netip.ParseAddr(host)
		if err != nil || !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// farProblem tells why a room outside the network didn't answer, of the
// host named name.
func farProblem(name string) string {
	if name == "" || name == "The room" {
		name = "the room's host"
	}
	return "Couldn't reach " + name + " over the internet: their router may not forward the room's port, a firewall may turn MixMusic away, or the room ended. A VPN both of you run, such as Tailscale, reaches it where the router doesn't."
}
