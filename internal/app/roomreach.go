package app

// Friends anywhere (#562): the room this computer hosts listens at a port
// kept from room to room, and, while the user lets friends outside the
// network join, MixMusic asks the router to forward it (internal/portmap),
// and the invite link carries where friends outside reach the room: the
// router's internet address, this computer's public IPv6 address, or the
// address the user typed for invites. There is no server: guests reach
// this computer directly. MixMusic can't test the port from outside, so
// the room's page tells what it set up, and why nothing works when it
// knows.

import (
	"errors"
	"log"
	"math/rand/v2"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/portmap"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// roomFar is what rooms keep of friends outside the network, on both
// sides.
type roomFar struct {
	// reach is the listener of the room this computer hosts, while it
	// listens.
	reach atomic.Pointer[roomReach]
	// stopping waits for the routers to remove their forwards as rooms
	// end, which MixMusic waits for as it quits (flushing).
	stopping sync.WaitGroup
	flushing sync.Once
	// nets are the networks of this computer's interfaces, as read at
	// netsAt, which a room takes connections of alone while friends
	// outside may not join (reachListener).
	netsMu sync.Mutex
	nets   []netip.Prefix
	netsAt time.Time
	// link is where the room this computer knocks at, or is a guest of,
	// is reached from outside the network (roomfar.go). a.room.mu guards
	// it.
	link farLink
}

// roomPort is the port rooms listen on: the one kept in the settings, or
// one picked at random from 1024 to 65535, which is kept from then on.
func (a *App) roomPort() int {
	if p := a.settings.Get().Clean().RoomPort; p != 0 {
		return p
	}
	pick := 1024 + rand.IntN(65536-1024)
	s, err := (&Preferences{a}).Update(func(s *settings.Settings) {
		if s.RoomPort == 0 {
			s.RoomPort = pick
		}
	})
	if err != nil || s.RoomPort == 0 {
		return pick
	}
	return s.RoomPort
}

// listenRoom listens for the guests' connections on every network of the
// computer, at the port kept for rooms, or, when another program has it,
// at one the system picks, which a forward made by hand doesn't reach.
func (a *App) listenRoom() (net.Listener, error) {
	port := a.roomPort()
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err == nil {
		return ln, nil
	}
	log.Printf("mixmusic: room: port %d is taken (%v): listening at another", port, err)
	return net.Listen("tcp", ":0")
}

// reachable wraps listen, so that the room it listens for is reached from
// outside the network while the user lets friends outside join, until it
// stops listening.
func (a *App) reachable(listen func() (net.Listener, error)) func() (net.Listener, error) {
	return func() (net.Listener, error) {
		ln, err := listen()
		if err != nil {
			return nil, err
		}
		far := &a.room.far
		o := a.opts.Room
		far.flushing.Do(func() {
			a.OnFlush(a.waitForRouters)
			if o == nil {
				// A computer that woke may have another address, and its
				// router may have forgotten the forward.
				mygo.Power.OnResume(func() { a.Rooms().ReachAgain() })
			}
		})
		r := &roomReach{a: a, kept: a.settings.Get().Clean().RoomPort}
		if addr, ok := ln.Addr().(*net.TCPAddr); ok {
			r.port = addr.Port
		}
		switch {
		case o == nil:
			// The network's routers, which only MixMusic itself asks:
			// tests that start rooms always give RoomOptions.
			r.router, r.ipv6 = &portmap.Client{}, portmap.PublicIPv6
		case o.Router != nil:
			r.router, r.ipv6 = o.Router, o.IPv6
			if r.ipv6 == nil {
				r.ipv6 = func() []netip.Addr { return nil }
			}
		}
		far.reach.Store(r)
		if a.settings.Get().Clean().RoomAnywhere {
			r.start()
		}
		return &reachListener{Listener: ln, r: r}, nil
	}
}

// waitForRouters waits, a moment at most, for the routers to remove the
// forwards of the rooms that ended, as MixMusic quits.
func (a *App) waitForRouters() {
	done := make(chan struct{})
	go func() {
		a.room.far.stopping.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

// reachListener is the room's listener, whose closing ends its reach.
// While friends outside the network may not join, it takes connections of
// the network alone, before TLS: the kept port listens on every network,
// which, without a router that turns away what comes in unasked, as some
// of IPv6 have none, the internet may reach.
type reachListener struct {
	net.Listener
	r    *roomReach
	once sync.Once
}

func (l *reachListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		l.r.mu.Lock()
		on := l.r.on
		l.r.mu.Unlock()
		if on || l.r.a.nearby(c.RemoteAddr()) {
			return c, nil
		}
		c.Close()
	}
}

// nearby reports whether addr is of this computer's own networks: a
// private, loopback or link-local address, or one of the networks of its
// interfaces, which it reads again each half minute.
func (a *App) nearby(addr net.Addr) bool {
	ap, err := netip.ParseAddrPort(addr.String())
	if err != nil {
		return false
	}
	far := &a.room.far
	far.netsMu.Lock()
	if time.Since(far.netsAt) > 30*time.Second {
		far.nets, far.netsAt = interfaceNets(), time.Now()
	}
	nets := far.nets
	far.netsMu.Unlock()
	return nearbyOf(ap.Addr(), nets)
}

// nearbyOf reports whether ip is private, loopback or link-local, or of
// one of nets.
func nearbyOf(ip netip.Addr, nets []netip.Prefix) bool {
	ip = ip.Unmap().WithZone("")
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	for _, p := range nets {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// interfaceNets are the networks of this computer's interfaces, but those
// of one address alone, as a VPN's of /32, which tell nothing of who else
// is on them.
func interfaceNets() []netip.Prefix {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []netip.Prefix
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(ipn.IP)
		ones, bits := ipn.Mask.Size()
		if !ok || ones == bits || bits == 0 {
			continue
		}
		ip = ip.Unmap()
		if ip.Is4() && bits == 128 {
			ones -= 96
		}
		out = append(out, netip.PrefixFrom(ip, ones).Masked())
	}
	return out
}

func (l *reachListener) Close() error {
	l.once.Do(func() {
		r := l.r
		r.a.room.far.reach.CompareAndSwap(r, nil)
		r.stop()
	})
	return l.Listener.Close()
}

// roomReach is how friends outside the network reach the room this
// computer hosts: the port it listens on, and what the routers were asked.
type roomReach struct {
	a *App
	// port is the port the room listens on, and kept the one kept for
	// rooms, which another program may have had.
	port, kept int
	// router asks the routers, and ipv6 tells this computer's public IPv6
	// addresses; neither when nothing is to be asked, as in tests that set
	// no router.
	router *portmap.Client
	ipv6   func() []netip.Addr

	mu     sync.Mutex
	on     bool
	keeper *portmap.Keeper
	// base are the addresses of the link the user copied last, or that
	// the routers' first answer gave; moved is set once they differ.
	base    []string
	hasBase bool
	told    bool
}

// start starts reaching out: the routers are asked, unless an address for
// invites is typed, which invites carry in place of theirs.
func (r *roomReach) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.on {
		return
	}
	r.on, r.hasBase, r.base, r.told = true, false, nil, false
	if r.typed() != "" || r.router == nil || r.port <= 0 || r.port > 65535 {
		return
	}
	k := &portmap.Keeper{Client: r.router, Port: uint16(r.port), IPv6: r.ipv6, Changed: r.changed}
	r.keeper = k
	k.Start()
	log.Printf("mixmusic: room: asking the router to forward port %d", r.port)
}

// stop stops reaching out, and has the routers remove their forwards,
// beside the caller: MixMusic waits for them as it quits.
func (r *roomReach) stop() {
	r.mu.Lock()
	k := r.keeper
	r.on, r.keeper = false, nil
	r.mu.Unlock()
	if k == nil {
		return
	}
	far := &r.a.room.far
	far.stopping.Add(1)
	go func() {
		defer far.stopping.Done()
		k.Stop()
		log.Print("mixmusic: room: the router no longer forwards the room's port")
	}()
}

// typed is the address for invites the user typed, or "".
func (r *roomReach) typed() string { return r.a.settings.Get().Clean().RoomAddress }

// addrs are where friends outside reach the room, as the invite link
// carries them: none while they may not join.
func (r *roomReach) addrs() []string {
	r.mu.Lock()
	on, k := r.on, r.keeper
	r.mu.Unlock()
	if !on {
		return nil
	}
	if t := r.typed(); t != "" {
		if a, ok := room.InviteAddr(t, r.port); ok {
			return []string{a}
		}
		return nil
	}
	if k == nil {
		return nil
	}
	var out []string
	reach := k.Reach()
	if reach.Forward.IsValid() {
		out = append(out, reach.Forward.String())
	}
	if reach.IPv6.IsValid() {
		out = append(out, netip.AddrPortFrom(reach.IPv6, uint16(r.port)).String())
	}
	return out
}

// changed takes in what the routers answered: the room's addresses, once
// they first answered, are those of its link, and the user is told when
// they change, so that they send friends the new link.
func (r *roomReach) changed() {
	addrs := r.addrs()
	r.mu.Lock()
	k := r.keeper
	asking := k != nil && k.Reach().Asking
	tell := false
	if !asking {
		if !r.hasBase {
			r.base, r.hasBase = addrs, true
		} else if !slices.Equal(addrs, r.base) && !r.told {
			r.told, tell = true, len(addrs) > 0
		}
	}
	r.mu.Unlock()
	if tell {
		log.Printf("mixmusic: room: reached at %v now", addrs)
		r.a.notifyRooms(roomMoved)
		return
	}
	r.a.notifyRooms("")
}

// roomMoved is what the host is told as the room's addresses change.
const roomMoved = "Your room's internet address changed: send friends outside your network the new invite link."

// copied takes in that the user copied the room's link, of the room's
// addresses now.
func (r *roomReach) copied() {
	addrs := r.addrs()
	r.mu.Lock()
	r.base, r.hasBase, r.told = addrs, true, false
	r.mu.Unlock()
}

// RoomReach is how friends outside this network reach the room this
// computer hosts, for its page.
type RoomReach struct {
	// On is set while friends outside this network may join, and Asking
	// while MixMusic asks the router the first time.
	On, Asking bool
	// Addrs are where they reach the room, which the invite link
	// carries.
	Addrs []string
	// Done tells what MixMusic set up, a sentence each.
	Done []string
	// Problem tells why friends outside may not reach the room, as far
	// as MixMusic can tell.
	Problem string
	// Moved is set once the addresses changed since the link was copied,
	// or since they were first found: friends outside need the new link.
	Moved bool
	// Port is the port the room listens on.
	Port int
}

// Reach returns how friends outside this network reach the room this
// computer hosts, and whether it hosts one.
func (rs *Rooms) Reach() (RoomReach, bool) {
	r := rs.a.room.far.reach.Load()
	if r == nil || rs.a.hostedRoom() == nil {
		return RoomReach{}, false
	}
	return r.view(), true
}

func (r *roomReach) view() RoomReach {
	addrs := r.addrs()
	r.mu.Lock()
	on, k := r.on, r.keeper
	moved := r.hasBase && !slices.Equal(addrs, r.base)
	r.mu.Unlock()
	v := RoomReach{On: on, Addrs: addrs, Moved: moved && len(addrs) > 0, Port: r.port}
	if !on {
		return v
	}
	if r.kept != 0 && r.port != r.kept {
		v.Done = append(v.Done, "Port "+strconv.Itoa(r.kept)+", kept for rooms, is taken by another program: the room listens at port "+
			strconv.Itoa(r.port)+" instead, which a port forwarded by hand doesn't reach.")
	}
	if t := r.typed(); t != "" {
		if len(addrs) == 0 {
			v.Problem = "The address for invites, " + t + ", isn't an address: change it in Settings › Desktop › Listen with friends."
		} else {
			v.Done = append(v.Done, "Invites carry the address you typed for them, "+addrs[0]+". It must reach this computer's port "+strconv.Itoa(r.port)+".")
		}
		return v
	}
	if k == nil {
		v.Problem = "MixMusic asks no router here."
		return v
	}
	reach := k.Reach()
	if reach.Asking {
		v.Asking = true
		return v
	}
	if reach.Forward.IsValid() {
		v.Done = append(v.Done, "Your router forwards port "+strconv.Itoa(int(reach.Forward.Port()))+" of its internet address, "+
			reach.Forward.Addr().String()+", to this computer ("+string(reach.Method)+").")
	}
	if reach.IPv6.IsValid() {
		if reach.Opened {
			v.Done = append(v.Done, "Friends with IPv6 reach this computer at "+reach.IPv6.String()+", where your router lets the port in (PCP).")
		} else {
			v.Done = append(v.Done, "Friends with IPv6 may reach this computer at "+reach.IPv6.String()+", if your router lets port "+strconv.Itoa(r.port)+" in.")
		}
	}
	if reach.Err != nil {
		v.Problem = reachProblem(reach.Err, r.port)
	}
	return v
}

// reachProblem tells why the router of IPv4 forwards no port, for the
// room's port, in the interface's words.
func reachProblem(err error, port int) string {
	p := strconv.Itoa(port)
	vpn := " A VPN both of you run, such as Tailscale, reaches the room anyway: put this computer's address on it in Address for invites, in Settings › Desktop › Listen with friends."
	var shared *portmap.SharedError
	var refused *portmap.RefusedError
	switch {
	case errors.As(err, &shared):
		return "Your router's internet address, " + shared.Addr.String() + ", is shared with your provider's other customers, or private: the router sits behind another, most often the provider's, out of reach from outside." + vpn
	case errors.As(err, &refused):
		return "Your router refused to forward port " + p + ": " + refused.Reason + ". Forward the port to this computer by hand in the router's settings, or turn on UPnP there." + vpn
	}
	return "Your router didn't answer: it may not take forwards from computers, or have UPnP, NAT-PMP and PCP off. Turn one on in its settings, or forward port " + p + " to this computer by hand." + vpn
}

// SetAnywhere lets friends outside this network join the rooms this
// computer hosts, or not: the room open now asks the router at once, or
// has it remove the forward.
func (rs *Rooms) SetAnywhere(on bool) {
	a := rs.a
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.RoomAnywhere = on }); err != nil {
		log.Printf("mixmusic: room: %v", err)
	}
	if r := a.room.far.reach.Load(); r != nil {
		if on {
			r.start()
		} else {
			r.stop()
		}
	}
	a.notifyRooms("")
}

// ReachAgain asks the routers again for the room open now, as the address
// for invites changed, or the computer woke.
func (rs *Rooms) ReachAgain() {
	r := rs.a.room.far.reach.Load()
	if r == nil {
		return
	}
	r.mu.Lock()
	on, k := r.on, r.keeper
	r.mu.Unlock()
	if !on {
		return
	}
	if (r.typed() != "") != (k == nil) {
		// An address typed, or no longer: the routers are asked, or not.
		r.stop()
		r.start()
	} else if k != nil {
		k.Poke()
	}
	rs.a.notifyRooms("")
}

// LinkCopied takes in that the user copied the invite link of the room
// this computer hosts, of the addresses it has now.
func (rs *Rooms) LinkCopied() {
	if r := rs.a.room.far.reach.Load(); r != nil {
		r.copied()
	}
	rs.a.notifyRooms("")
}

// reachLink is link, the invite link of the room this computer hosts,
// with the addresses that reach it from outside, while friends outside
// may join.
func (a *App) reachLink(link string) string {
	r := a.room.far.reach.Load()
	if r == nil {
		return link
	}
	in, err := room.ParseLink(link)
	if err != nil {
		return link
	}
	in.Addrs = r.addrs()
	return in.Link()
}
