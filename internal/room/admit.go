package room

// Admission: what a room takes of the computers that proved nothing yet.
// The code is the only thing between someone on the network and the room,
// and SPAKE2 has each try check one code, online: so the tries are what the
// room counts. Each source, an IPv4 address or the /56 of an IPv6 one,
// which one home changes within as it likes, tries Tries codes, then
// waits; the whole room takes RoomTries wrong ones each Wait, then takes no
// code until the wait is over: as many from its own network apart, so that
// wrong codes from the internet lock no code out on the network (#562),
// nor the other way. A try counts as it starts, before SPAKE2
// runs, so that tries at once count too. Links, of a secret too long to
// guess, and guests that come back by their keys, aren't counted.
//
// At 30 codes a minute, the room tries 43,200 a day: the 1.29 billion
// codes would take 82 years, half of them 41, or 41 and 20 with the
// network's 30 beside the internet's. A room lasts an evening.
//
// Before they prove anything, the room holds a few connections at once,
// from all and from each source, a few seconds in all, and takes messages
// of a handshake's size from them. A new one takes the place of the oldest,
// so that connections held open keep no one out; a guest that comes back by
// its key gives its place back once TLS tells its key.

import (
	"net"
	"net/netip"
	"time"
)

const (
	// RoomTries bounds the wrong codes the whole room takes each Wait.
	RoomTries = 30
	// maxSources bounds the sources whose tries the room counts.
	maxSources = 4096
	// maxUnproven bounds the connections that proved nothing yet, room
	// wide, and maxUnprovenFrom those of one source.
	maxUnproven     = 32
	maxUnprovenFrom = 4
	// handshakeMessage bounds a message before its sender proved the code,
	// the link or its key: a hello is a few hundred bytes. guestMessage
	// bounds one after: a song a guest adds is the largest, a few KiB.
	handshakeMessage = 4 << 10
	guestMessage     = 64 << 10
)

// sourceOf is the source of the address addr ("host:port", or a host): its
// IPv4 address, or the /56 of its IPv6 address, as a home's router has
// from its provider, 256 networks of /64 (#562); or a loopback address
// whole, this computer's own.
func sourceOf(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	ip = ip.Unmap().WithZone("")
	if ip.Is4() || ip.IsLoopback() {
		return ip.String()
	}
	p, err := ip.Prefix(56)
	if err != nil {
		return ip.String()
	}
	return p.String()
}

// caps are the bounds of admission, which tests change.
type caps struct {
	unproven, unprovenFrom int
	tries, roomTries       int
	// rate bounds the messages read of a connection each second
	// (limits.go): maxRate when 0.
	rate int
	// accepts bounds the connections a source opens each second, after a
	// burst of acceptBurst (limits.go): none when 0, as in tests of other
	// bounds.
	accepts int
	// previews and previewsFrom bound the computers that proved the code
	// or the link and wait to join, room-wide and of one source
	// (previews.go): when 0, maxPreviews and maxPreviewsFrom, or the caps
	// of the unproven when more.
	previews, previewsFrom int
}

func (c *caps) orDefault() caps {
	if c == nil {
		return caps{unproven: maxUnproven, unprovenFrom: maxUnprovenFrom, tries: Tries, roomTries: RoomTries, accepts: acceptRate}
	}
	return *c
}

// tries are the codes a source, or the room, tried since since: those
// found wrong, and those under way, which may be. It waits until until once
// it tried as many wrong ones as it may; last is when it last tried.
type tries struct {
	wrong, running     int
	since, until, last time.Time
}

// try is what a try of a code may do: run, or not, as its source tried too
// many wrong ones (tryWaits), or the room did (tryLocked), or as so many
// run that, all wrong, they would be too many (tryBusy).
type try int

const (
	tryRuns try = iota
	tryWaits
	tryBusy
	tryLocked
)

// admission counts the codes tried, and the connections that proved
// nothing yet, oldest first. Server.mu guards it.
type admission struct {
	caps     caps
	sources  map[string]*tries
	room     tries
	local    tries // of the room's own network, apart (localSource)
	unproven []*unproven
}

// unproven is a connection of src that proved nothing yet, which close
// closes.
type unproven struct {
	src   string
	close func()
	gone  bool
}

// letIn takes a connection of src that proves nothing yet, which close
// closes, and returns those it takes the place of: the oldest of src, when
// the room holds as many of src as it takes, and, when it holds as many as
// it takes in all, the oldest of the source that holds the most, so that
// sources that each flood the room take each other's places, not those of
// a guest on its way in. The caller closes them.
func (a *admission) letIn(src string, close func()) (*unproven, []*unproven) {
	var out []*unproven
	n := 0
	for _, u := range a.unproven {
		if u.src == src {
			n++
		}
	}
	if n >= a.caps.unprovenFrom {
		for _, u := range a.unproven {
			if u.src == src {
				out = append(out, u)
				a.proven(u)
				break
			}
		}
	}
	if len(a.unproven) >= a.caps.unproven && len(a.unproven) > 0 {
		u := a.crowding()
		out = append(out, u)
		a.proven(u)
	}
	u := &unproven{src: src, close: close}
	a.unproven = append(a.unproven, u)
	return u, out
}

// crowding is the oldest unproven connection of the source that holds the
// most of them, the oldest of those that hold as many.
func (a *admission) crowding() *unproven {
	held := map[string]int{}
	most := 0
	for _, u := range a.unproven {
		held[u.src]++
		most = max(most, held[u.src])
	}
	for _, u := range a.unproven {
		if held[u.src] == most {
			return u
		}
	}
	return a.unproven[0]
}

// proven lets go of a connection that proved the code, the link or its
// key, or ended, or gave its place to another.
func (a *admission) proven(u *unproven) {
	if u.gone {
		return
	}
	u.gone = true
	for i, o := range a.unproven {
		if o == u {
			a.unproven = append(a.unproven[:i:i], a.unproven[i+1:]...)
			return
		}
	}
}

// check is what t, which may try limit wrong codes each wait, lets a try
// do at now.
func (t *tries) check(limit int, now time.Time, wait time.Duration) try {
	if !t.until.IsZero() && !now.Before(t.until) || t.until.IsZero() && now.Sub(t.since) >= wait {
		// The wait, or the window, is over; the tries under way go on.
		*t = tries{since: now, last: t.last, running: t.running}
	}
	switch {
	case now.Before(t.until):
		return tryWaits
	case t.wrong >= limit:
		t.until = now.Add(wait)
		return tryWaits
	case t.wrong+t.running >= limit:
		return tryBusy
	}
	return tryRuns
}

// tryCode starts a try of a code from src, at now, before it runs, unless
// src, or the room, waits after too many wrong ones, or so many run that,
// all wrong, they would be. endTry ends it.
func (a *admission) tryCode(src string, now time.Time, wait time.Duration) try {
	room := a.roomOf(src)
	switch room.check(a.caps.roomTries, now, wait) {
	case tryWaits:
		return tryLocked
	case tryBusy:
		return tryBusy
	}
	t := a.sources[src]
	if t == nil {
		if len(a.sources) >= maxSources {
			a.sweep(now, wait)
		}
		if len(a.sources) >= maxSources {
			// So many sources try at once that the room counts no more:
			// none tries until some are let go.
			return tryBusy
		}
		if a.sources == nil {
			a.sources = map[string]*tries{}
		}
		t = &tries{since: now}
		a.sources[src] = t
	}
	if v := t.check(a.caps.tries, now, wait); v != tryRuns {
		return v
	}
	t.running++
	t.last = now
	room.running++
	return tryRuns
}

// end ends a try of t, wrong or not, at now: once it tried limit wrong
// ones, it waits.
func (t *tries) end(wrong bool, limit int, now time.Time, wait time.Duration) {
	t.running = max(t.running-1, 0)
	if !wrong {
		return
	}
	t.wrong++
	if t.wrong >= limit && t.until.IsZero() {
		t.until = now.Add(wait)
	}
}

// endTry ends the try of a code from src that tryCode started: wrong, or
// the code, or none at all, as one that ended before it was checked.
func (a *admission) endTry(src string, wrong bool, now time.Time, wait time.Duration) {
	a.roomOf(src).end(wrong, a.caps.roomTries, now, wait)
	if t := a.sources[src]; t != nil {
		t.end(wrong, a.caps.tries, now, wait)
	}
}

// locked returns until when the room takes no code, after too many wrong
// ones, or the zero time.
func (a *admission) locked(now time.Time) time.Time {
	for _, t := range []*tries{&a.room, &a.local} {
		if now.Before(t.until) {
			return t.until
		}
	}
	return time.Time{}
}

// sweep forgets the sources that tried longer than wait ago, and neither
// wait nor try now.
func (a *admission) sweep(now time.Time, wait time.Duration) {
	for src, t := range a.sources {
		if t.running == 0 && now.Sub(t.last) >= wait && !now.Before(t.until) {
			delete(a.sources, src)
		}
	}
}
