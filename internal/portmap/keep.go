package portmap

import (
	"context"
	"net/netip"
	"sync"
	"time"
)

// Keeper keeps the TCP port Port of this computer reachable from the
// internet, while a room of Listen with friends is open: it asks the
// router of IPv4 to forward it (Map), and the router of IPv6 to let the
// port of this computer's public IPv6 address in (Open), renews both at
// half their lifetime, and at once when Poke, as the computer wakes, tells
// what it set up (Reach), and removes both as it stops.
type Keeper struct {
	Client *Client
	Port   uint16
	// Lifetime is how long the routers keep what is asked, unless it is
	// renewed: 30 minutes when 0, so that what a MixMusic that ended at
	// once left goes soon.
	Lifetime time.Duration
	// IPv6 returns this computer's IPv6 addresses the internet may
	// reach: PublicIPv6 when nil.
	IPv6 func() []netip.Addr
	// Changed is told, from the keeper's goroutine, as Reach changes.
	Changed func()

	mu     sync.Mutex
	reach  Reach
	v4, v6 *Mapping
	poke   chan struct{}
	stop   chan struct{}
	done   chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
}

// Reach is what the keeper set up.
type Reach struct {
	// Asking is set while the routers are asked the first time.
	Asking bool
	// Forward is where the internet reaches the port through the router
	// of IPv4, and Method how it was asked; Err tells why nothing does:
	// ErrNoRouter, a *RefusedError or a *SharedError.
	Forward netip.AddrPort
	Method  Method
	Err     error
	// IPv6 is this computer's IPv6 address the internet may reach, if it
	// has one, and Opened is set once its router said, by PCP, that it
	// lets the port in there.
	IPv6   netip.Addr
	Opened bool
}

func (r Reach) same(o Reach) bool {
	errText := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}
	return r.Asking == o.Asking && r.Forward == o.Forward && r.Method == o.Method && errText(r.Err) == errText(o.Err) &&
		r.IPv6 == o.IPv6 && r.Opened == o.Opened
}

// SharedError: the router's internet address, Addr, is one the internet
// doesn't reach (Shared): the router sits behind another.
type SharedError struct{ Addr netip.Addr }

func (e *SharedError) Error() string {
	return "the router's internet address, " + e.Addr.String() + ", is shared or private: it sits behind another router"
}

func (k *Keeper) lifetime() time.Duration {
	if k.Lifetime > 0 {
		return k.Lifetime
	}
	return 30 * time.Minute
}

func (k *Keeper) ipv6() []netip.Addr {
	if k.IPv6 != nil {
		return k.IPv6()
	}
	return PublicIPv6()
}

// Start starts asking the routers, from a goroutine of its own.
func (k *Keeper) Start() {
	k.poke = make(chan struct{}, 1)
	k.stop = make(chan struct{})
	k.done = make(chan struct{})
	k.ctx, k.cancel = context.WithCancel(context.Background())
	k.mu.Lock()
	k.reach = Reach{Asking: true}
	k.mu.Unlock()
	go k.run()
}

// Poke asks the routers again at once, as after the computer woke, or its
// network changed.
func (k *Keeper) Poke() {
	select {
	case k.poke <- struct{}{}:
	default:
	}
}

// Stop stops asking, and asks the routers to remove what they were asked,
// waiting a moment at most for them.
func (k *Keeper) Stop() {
	close(k.stop)
	k.cancel()
	<-k.done
}

// Reach returns what the keeper set up.
func (k *Keeper) Reach() Reach {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.reach
}

func (k *Keeper) run() {
	defer close(k.done)
	for {
		next := k.ask()
		t := time.NewTimer(next)
		select {
		case <-k.stop:
			t.Stop()
			k.remove()
			return
		case <-k.poke:
			t.Stop()
		case <-t.C:
		}
	}
}

// ask asks the routers once, and returns when to ask again: at half the
// shortest lifetime they gave, sooner when nothing reaches the port.
func (k *Keeper) ask() time.Duration {
	ctx, cancel := context.WithTimeout(k.ctx, 30*time.Second)
	defer cancel()
	c, life := k.Client, k.lifetime()
	k.mu.Lock()
	old4, old6 := k.v4, k.v6
	k.mu.Unlock()
	var m4 *Mapping
	var err error
	if old4 != nil {
		m4, err = c.Renew(ctx, old4, life)
	}
	if old4 == nil || err != nil {
		// None yet, or the router forgot it, or another router answers
		// now.
		m4, err = c.Map(ctx, k.Port, life)
	}
	if err == nil && !Unicast(m4.External.Addr()) {
		// A router that tells no address of the internet: what it says
		// can't go in a link.
		_ = c.Delete(ctx, m4)
		err, m4 = &RefusedError{Method: m4.Method, Reason: "it told " + m4.External.Addr().String() + " as its internet address, which isn't one"}, nil
	}
	if err == nil && Shared(m4.External.Addr()) {
		// A forward that the internet doesn't reach helps no one.
		_ = c.Delete(ctx, m4)
		err, m4 = &SharedError{Addr: m4.External.Addr()}, nil
	}
	var v6 netip.Addr
	if addrs := k.ipv6(); len(addrs) > 0 {
		v6 = addrs[0]
	}
	var m6 *Mapping
	if old6 != nil && old6.local == v6 {
		m6, _ = c.Renew(ctx, old6, life)
	} else if old6 != nil {
		_ = c.Delete(ctx, old6)
	}
	if v6.IsValid() && m6 == nil {
		m6, _ = c.Open(ctx, v6, k.Port, life)
	}
	if k.ctx.Err() != nil {
		// Stopped meanwhile: what was set up goes.
		m4, m6 = orKept(m4, old4), orKept(m6, old6)
	}
	r := Reach{Err: err, IPv6: v6, Opened: m6 != nil}
	if m4 != nil {
		r.Forward, r.Method = m4.External, m4.Method
	}
	k.mu.Lock()
	k.v4, k.v6 = m4, m6
	changed := !r.same(k.reach)
	k.reach = r
	k.mu.Unlock()
	if changed && k.Changed != nil {
		k.Changed()
	}
	next := min(15*time.Minute, life/2)
	for _, m := range []*Mapping{m4, m6} {
		if m != nil && m.Lifetime > 0 {
			next = min(next, m.Lifetime/2)
		}
	}
	return max(next, time.Second)
}

// orKept is m, or the mapping kept before, when ask was stopped before it
// told.
func orKept(m, kept *Mapping) *Mapping {
	if m != nil {
		return m
	}
	return kept
}

// remove asks the routers to remove what they keep, a moment at most.
func (k *Keeper) remove() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	k.mu.Lock()
	m4, m6 := k.v4, k.v6
	k.v4, k.v6 = nil, nil
	k.mu.Unlock()
	for _, m := range []*Mapping{m4, m6} {
		if m != nil {
			// A forward the router doesn't remove goes at the end of its
			// lifetime.
			_ = k.Client.Delete(ctx, m)
		}
	}
}
