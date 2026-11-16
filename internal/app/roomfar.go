package app

// The guest's side of friends anywhere (#562): an invite link carries where
// its room is reached from outside the host's network, which a knock, and
// a guest that comes back, dial beside the address found on the network
// (room.DialFirst); Join a room… knocks at an address typed, with the
// room's code; and a guest that dropped off takes a new link of its host's
// to come back at the host's new address.

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/room"
)

// farLink is where the room of the host id is reached from outside the
// network, as the link it was knocked at by, of the secret secret, or the
// address the user typed, tells: the knock that follows proves the link,
// and only a link of the same secret changes them (backByInvite), so that
// a link made up of the host's key alone moves no guest elsewhere. typed
// is set when the user typed them, which are dialled as they are
// (roomDial).
type farLink struct {
	id     string
	secret []byte
	addrs  []string
	typed  bool
}

// knock takes in the knock at the room of the host id, by by, which
// starts: its addresses are the room's from then on. a.room.mu is held.
func (f *roomFar) knock(id string, by knockBy) {
	f.link = farLink{id: id, addrs: slices.Clone(by.addrs), typed: by.typed}
	if by.link {
		f.link.secret = slices.Clone(by.password)
	}
}

// linkAddrs are the addresses of the room of the host id, outside the
// network, while it's the room this computer knocks at, or is a guest of.
func (a *App) linkAddrs(id string) []string {
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.far.link.id != id {
		return nil
	}
	return slices.Clone(r.far.link.addrs)
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

// roomElsewhere tells where else the room this computer knocks at, or
// comes back to, is reached: the addresses of its invite link, the others
// its host told discovery, as on a VPN, and where it moved, as its old
// host reached it, which a connection to its host's key tries beside the
// one found on the network, keeping the first that is the host
// (room.Guest.Elsewhere). With none, the room is of the network alone.
func (a *App) roomElsewhere() []string {
	g := a.guestRoom()
	if g == nil {
		return nil
	}
	out := a.linkAddrs(g.id)
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.found[g.id]; ok {
		out = append(out, f.found.Others...)
	}
	if g.hint != "" {
		out = append(out, g.hint)
	}
	return out
}

// roomDial connects to a room's address: as it is, where discovery found a
// room on the network, or the user typed it; by room.FarDialer, which
// connects to neither this computer nor an address of no one, where a link
// or a host told it, as anyone may make those up.
func (a *App) roomDial(ctx context.Context, network, addr string) (net.Conn, error) {
	if a.asIs(addr) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	if o := a.opts.Room; o != nil && o.DialFar != nil {
		return o.DialFar(ctx, network, addr)
	}
	return room.FarDialer().DialContext(ctx, network, addr)
}

// asIs reports whether addr is dialled as it is: where discovery found a
// room, or the address the user typed to join one at.
func (a *App) asIs(addr string) bool {
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.far.link.typed && slices.Contains(r.far.link.addrs, addr) {
		return true
	}
	for _, f := range r.found {
		if f.found.Addr == addr || slices.Contains(f.found.Others, addr) {
			return true
		}
	}
	return false
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
	a.startKnock(at, nil, knockBy{password: []byte(parsed), addrs: []string{at}, typed: true}, nil)
}

// backByInvite takes a new invite link of the host this computer is a
// guest of: a guest that dropped off tries to come back at once, at the
// link's addresses, as the host's address changed, when the link is of the
// secret of the one it joined by; one of another secret, which anyone who
// knows the host's key can make, moves it nowhere, and one with no address
// keeps those it has. It reports whether the link was the host's.
func (a *App) backByInvite(in room.Invite) bool {
	r := &a.room
	r.mu.Lock()
	g := r.guest
	mine := g != nil && g.hasKey && g.key == in.Host && g.step != RoomEnded && g.step != RoomKnocking && g.step != RoomPreview
	away := mine && g.step == RoomAway
	if away && g.retry != nil {
		g.retry.Stop()
	}
	if l := &r.far.link; mine && l.id == g.id && len(l.secret) > 0 && bytes.Equal(l.secret, in.Secret) && len(in.Addrs) > 0 {
		l.addrs = slices.Clone(in.Addrs)
	}
	r.mu.Unlock()
	if !mine {
		return false
	}
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
