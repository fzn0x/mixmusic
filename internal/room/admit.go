package room

// Admission: what a room takes of the computers that proved nothing yet.
// The code is the only thing between someone on the network and the room,
// and SPAKE2 has each try check one code, online: so the tries are what the
// room counts. Each source, an IPv4 address or the /64 of an IPv6 one,
// which one computer changes within as it likes, tries Tries codes, then
// waits; the whole room takes RoomTries wrong ones each Wait, then takes no
// code until the wait is over. A try counts as it starts, before SPAKE2
// runs, so that tries at once count too. Links, of a secret too long to
// guess, and guests that come back by their keys, aren't counted.
//
// At 30 codes a minute, the room tries 43,200 a day: the 1.29 billion
// codes would take 82 years, half of them 41. A room lasts an evening.
//
// Before they prove anything, the room holds a few connections at once,
// from all and from each source, and takes messages of a handshake's size
// from them.

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
	// the link or its key: a hello is a few hundred bytes.
	handshakeMessage = 4 << 10
)

// sourceOf is the source of the address addr ("host:port", or a host): its
// IPv4 address, or the /64 of its IPv6 address.
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
	if ip.Is4() {
		return ip.String()
	}
	p, err := ip.Prefix(64)
	if err != nil {
		return ip.String()
	}
	return p.String()
}

// caps are the bounds of admission, which tests change.
type caps struct {
	unproven, unprovenFrom int
	tries, roomTries       int
}

func (c *caps) orDefault() caps {
	if c == nil {
		return caps{unproven: maxUnproven, unprovenFrom: maxUnprovenFrom, tries: Tries, roomTries: RoomTries}
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

// try is what a try of a code may do: run, or not, as too many wrong ones
// were tried (tryWaits), or as so many run that, all wrong, they would be
// too many (tryBusy).
type try int

const (
	tryRuns try = iota
	tryWaits
	tryBusy
)

// admission counts the codes tried, and the connections that proved
// nothing yet. Server.mu guards it.
type admission struct {
	caps     caps
	sources  map[string]*tries
	room     tries
	unproven int
	from     map[string]int
}

// letIn takes a connection of src that proves nothing yet, unless the room
// holds as many as it takes, from all or from src.
func (a *admission) letIn(src string) bool {
	if a.unproven >= a.caps.unproven || a.from[src] >= a.caps.unprovenFrom {
		return false
	}
	if a.from == nil {
		a.from = map[string]int{}
	}
	a.unproven++
	a.from[src]++
	return true
}

// proven lets go of a connection of src that proved the code, the link
// or its key, or ended.
func (a *admission) proven(src string) {
	a.unproven = max(a.unproven-1, 0)
	if a.from[src] <= 1 {
		delete(a.from, src)
	} else {
		a.from[src]--
	}
}

// check is what t, which may try limit wrong codes each wait, lets a try
// do at now.
func (t *tries) check(limit int, now time.Time, wait time.Duration) try {
	if !t.until.IsZero() && !now.Before(t.until) || t.until.IsZero() && t.running == 0 && now.Sub(t.since) >= wait {
		// The wait, or the window, is over.
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
	if v := a.room.check(a.caps.roomTries, now, wait); v != tryRuns {
		return v
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
	a.room.running++
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
	a.room.end(wrong, a.caps.roomTries, now, wait)
	if t := a.sources[src]; t != nil {
		t.end(wrong, a.caps.tries, now, wait)
	}
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
