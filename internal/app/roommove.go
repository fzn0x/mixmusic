package app

// Make host (#734): the room this computer hosts moves to one of its
// guests, if the others can reach it (internal/room's MakeHost). Asked to
// host, a guest opens a room of its own, which keeps the places of the
// others and of the host; once all reach it, it hosts the room: its player
// takes the room's queue, where it plays, and the others, the old host
// too, come back to it by their keys, as after a drop.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/room"
)

// CanHost reports whether the guest of key can host the room this
// computer hosts: it is here, and its MixMusic can.
func (rs *Rooms) CanHost(key string) bool {
	h, fp, ok := rs.member(key)
	return ok && h.server.CanHost(fp)
}

// Moving returns the name of the guest the room moves to, while it does.
func (rs *Rooms) Moving() (string, bool) {
	if h := rs.a.hostedRoom(); h != nil {
		return h.server.Moving()
	}
	return "", false
}

// MakeHost moves the room this computer hosts to the guest of key, if the
// others can reach it: this computer is then a guest of it. What became of
// it comes as a note.
func (rs *Rooms) MakeHost(key string) error {
	a := rs.a
	h, fp, ok := rs.member(key)
	if !ok {
		return errCantHost
	}
	err := h.server.MakeHost(fp, func(m room.Move, err error) {
		if err != nil {
			log.Printf("mixmusic: room: not moved: %v", err)
			a.notifyRooms("The room stays here: " + err.Error())
			return
		}
		go a.movedTo(h, m)
	})
	switch {
	case err == nil:
		a.notifyRooms("")
	case errors.Is(err, room.ErrMoving):
		return errMoving
	case errors.Is(err, room.ErrCantHost):
		return errCantHost
	}
	return err
}

// The errors of Make host, in the interface's words.
var (
	errCantHost = errors.New("That guest can't host the room now: they may be away, or their MixMusic older")
	errMoving   = errors.New("The room moves to another guest already")
)

// movedTo takes in that the room h moved to m: this computer hosts it no
// more, and comes back to it as a guest, by its key; the queue it played
// is the user's own, which comes back as they leave.
func (a *App) movedTo(h *hostedRoom, m room.Move) {
	r := &a.room
	r.mu.Lock()
	if r.hosted != h {
		r.mu.Unlock()
		return
	}
	r.hosted = nil
	r.mu.Unlock()
	r.held.release(true)
	h.close()
	g := &guestRoom{id: m.Key.ID(), name: m.Name, key: m.Key, hasKey: true, step: RoomAway, hint: m.Addr, own: a.keepOwnQueue()}
	r.mu.Lock()
	r.guest = g
	r.mu.Unlock()
	log.Printf("mixmusic: room: moved to %s", m.Name)
	a.notifyRooms("The room moved to " + m.Name + "'s computer: you're a guest in it now")
	go a.comeBack(g)
}

// hostAsked opens a room for the room of g to move to, which keeps seats,
// and tells its host where, in the session s; it closes once the room
// stays where it is, or by itself after a while.
func (a *App) hostAsked(g *guestRoom, s *room.Session, seats room.Seats) {
	r := &a.room
	r.mu.Lock()
	ok := r.guest == g && g.session == s && r.hosted == nil && g.pending == nil
	r.mu.Unlock()
	if !ok {
		_ = s.Hosting(0)
		return
	}
	h, port, err := a.openRoom(&seats)
	if err != nil {
		log.Printf("mixmusic: room: can't host: %v", err)
		_ = s.Hosting(0)
		return
	}
	r.mu.Lock()
	if r.guest != g || g.session != s || r.hosted != nil || g.pending != nil {
		r.mu.Unlock()
		h.close()
		_ = s.Hosting(0)
		return
	}
	g.pending = h
	r.mu.Unlock()
	time.AfterFunc(2*room.MoveFor, func() { a.unhost(g, h) })
	if err := s.Hosting(port); err != nil {
		a.unhost(g, h)
	}
}

// unhost closes h, the room opened for the room of g to move to, unless it
// hosts it now.
func (a *App) unhost(g *guestRoom, h *hostedRoom) {
	r := &a.room
	r.mu.Lock()
	mine := g.pending == h
	if mine {
		g.pending = nil
	}
	r.mu.Unlock()
	if mine {
		h.close()
	}
}

// reach tells the host of the session s whether this computer reaches the
// room of key it moves to: where discovery finds it, or at addr.
func (a *App) reach(s *room.Session, key remote.Fingerprint, addr string) {
	id, err := a.identity(true)
	if err != nil {
		_ = s.Reached(false)
		return
	}
	try := func(at string) bool {
		ctx, cancel := context.WithTimeout(context.Background(), room.MoveFor/3)
		defer cancel()
		return a.roomGuest(id).Reach(ctx, at, key) == nil
	}
	ok := false
	if f, found := a.foundRoom(key.ID(), true); found {
		ok = try(f.Addr)
	}
	if !ok && addr != "" {
		ok = try(addr)
	}
	_ = s.Reached(ok)
}

// watchMove has this computer, a guest of the room of g in the session s,
// open a room for it to move to, or try to reach the one it moves to, as
// its host asks, until the session ends: one of each at a time, whatever
// the host asks.
func (a *App) watchMove(g *guestRoom, s *room.Session) {
	var opening, reaching atomic.Bool
	s.Watch(func(e room.Event) {
		switch e.Kind {
		case room.HostAsked:
			if opening.CompareAndSwap(false, true) {
				go func() {
					defer opening.Store(false)
					a.hostAsked(g, s, e.Seats)
				}()
			}
		case room.Unhosted:
			r := &a.room
			r.mu.Lock()
			h := g.pending
			r.mu.Unlock()
			if h != nil {
				go a.unhost(g, h)
			}
		case room.Reach:
			if reaching.CompareAndSwap(false, true) {
				go func() {
					defer reaching.Store(false)
					a.reach(s, e.Key, e.Addr)
				}()
			}
		}
	})
}

// moved takes in that the room of g moved to m, as its session s ended:
// this computer hosts it, when m is it, or comes back to its new host by
// its key, as after a drop.
func (a *App) moved(g *guestRoom, s *room.Session, m room.Move) {
	if id := a.remote.id.Load(); id != nil && id.Fingerprint() == m.Key {
		a.hostMoved(g, s)
		return
	}
	log.Printf("mixmusic: room: moved to %s", m.Name)
	if a.setGuest(g, func() {
		g.id, g.name, g.key, g.hasKey, g.hint = m.Key.ID(), m.Name, m.Key, true, m.Addr
		g.step, g.session, g.tries = RoomAway, nil, 0
	}) {
		a.notifyRooms("The room moved to " + m.Name + "'s computer")
		go a.comeBack(g)
	}
}

// hostMoved has this computer host the room of g, which moved to it as
// its session s with the old host ended: its player takes the room's
// queue, where the old host played it, and the user's own queue from
// before they joined goes, as the room's is now theirs.
func (a *App) hostMoved(g *guestRoom, s *room.Session) {
	r := &a.room
	r.mu.Lock()
	h := g.pending
	if r.guest != g || h == nil || r.hosted != nil {
		r.mu.Unlock()
		g.end(a, room.ErrEnded)
		return
	}
	g.pending = nil
	r.guest, r.hosted = nil, h
	f, retry, cancel := g.follow, g.retry, g.cancel
	g.follow, g.own, g.retry, g.session = nil, nil, nil, nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if retry != nil {
		retry.Stop()
	}
	if f != nil {
		f.stop()
	}
	a.takeRoom(s)
	log.Print("mixmusic: room: hosting the room that moved here")
	a.notifyRooms("You host the room now")
}

// takeRoom has the player take the queue of the room of s, where its host
// played it last.
func (a *App) takeRoom(s *room.Session) {
	p := a.goPlayer.Load()
	if p == nil {
		return
	}
	q := s.Queue()
	st, _ := s.State()
	var tracks []media.Track
	index := 0
	for i, qt := range q.Tracks {
		t, ok := a.roomTrack(qt)
		if !ok {
			continue
		}
		if i <= q.Index {
			index = len(tracks)
		}
		tracks = append(tracks, t)
	}
	if len(tracks) == 0 {
		return
	}
	at := st.Position
	if off, ok := s.Offset(); ok && !st.Live {
		at = st.Playhead().PositionAt(s.Now() + off)
	}
	repeat := player.RepeatOff
	switch st.Repeat {
	case "all":
		repeat = player.RepeatAll
	case "one":
		repeat = player.RepeatOne
	}
	p.TakeOver(tracks, index, at, st.Playing, st.Shuffle, repeat)
}

// openRoom opens a room on the network, as Start does, of a new code and
// invite link, which keeps seats, and returns it and its port: the caller
// makes it the room this computer hosts, or closes it.
func (a *App) openRoom(seats *room.Seats) (*hostedRoom, int, error) {
	id, err := a.identity(true)
	if err != nil {
		return nil, 0, err
	}
	code, err := room.NewCode()
	if err != nil {
		return nil, 0, err
	}
	secret, err := room.NewSecret()
	if err != nil {
		return nil, 0, err
	}
	o := a.roomOptions()
	ln, err := o.Listen()
	if err != nil {
		return nil, 0, fmt.Errorf("The room can't listen on the network: %w", err)
	}
	h := &hostedRoom{key: id.Fingerprint(), code: code, link: room.Invite{Code: code, Host: id.Fingerprint(), Secret: secret}.Link(), told: time.Now()}
	srv := &room.Server{Identity: id, Name: a.roomName(), Code: code, Secret: secret, Host: roomHostPlayer{a}, Clock: a.roomClock(),
		Timing: o.Timing, Changed: func() { a.roomChanged(h) }, Guessing: func() { a.notifyRooms(roomGuessed) }, Guests: a.roomGuests(), Seats: seats}
	h.server = srv
	go func() {
		if err := srv.Serve(ln); err != nil {
			log.Printf("mixmusic: room: %v", err)
		}
	}()
	port := 0
	if addr, ok := ln.Addr().(*net.TCPAddr); ok {
		port = addr.Port
	}
	stop, err := o.Discovery.Announce(room.Announcement{ID: id.Fingerprint().ID(), Name: srv.Name, Port: port})
	if err != nil {
		srv.Close()
		return nil, 0, fmt.Errorf("The room can't be announced on the network: %w", err)
	}
	h.unannounce = stop
	return h, port, nil
}
