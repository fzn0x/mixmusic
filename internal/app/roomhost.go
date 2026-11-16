package app

// The host's side of Listen with friends: the room this computer opens,
// its code and invite link, its guests, whom the host can remove or block,
// and its player, whose track and queue the guests follow. At a track
// change the player's start waits for the guests (roomStream).

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/room"
)

// hostedRoom is the room this computer hosts: its server, its code, its
// invite link, and the announcement that has the others find it.
type hostedRoom struct {
	server     *room.Server
	key        remote.Fingerprint
	unannounce func()
	// code and link are the room's, which NewCode changes.
	codeMu     sync.Mutex
	code, link string
	// told is when the last activity the windows were told of happened.
	toldMu sync.Mutex
	told   time.Time
}

func (h *hostedRoom) close() {
	h.unannounce()
	h.server.Close()
}

// HostedRoom is the room this computer hosts, for its page: its code, its
// invite link, its guests, those blocked, and what happened in it.
type HostedRoom struct {
	Code, Link string
	Guests     []RoomMember
	Blocked    []RoomMember
	Activity   []room.Activity
	// Waiting is set while the next track waits for the guests to have it
	// ready.
	Waiting bool
}

// RoomMember is a guest of a room: its key, as Remove and Block name it,
// its name in rooms, and whether it is connected now.
type RoomMember struct {
	Key, Name string
	Here      bool
}

func membersOf(ms []room.Member) []RoomMember {
	out := make([]RoomMember, len(ms))
	for i, m := range ms {
		out[i] = RoomMember{Key: m.Key.String(), Name: m.Name, Here: m.Here}
	}
	return out
}

// roomGuessed is what the host is told as its room takes no code for a
// while, after too many wrong ones.
const roomGuessed = "Someone on the network is guessing your room's code. Friends can still join with the invite link, or give the room a new code."

// errInRoom: this computer is a guest of a room, and hosts none meanwhile.
var errInRoom = errors.New("Leave the room you're in to start one")

// Start opens a room on the network, of a new code and invite link, unless
// one is open.
func (rs *Rooms) Start() error {
	a := rs.a
	// The key first, with no lock held: see identity.
	id, err := a.identity(false)
	if err != nil {
		return err
	}
	a.keepRooms()
	r := &a.room
	r.mu.Lock()
	busy, guest := r.hosted != nil, r.guest != nil && r.guest.step != RoomEnded
	r.mu.Unlock()
	switch {
	case busy:
		return nil
	case guest:
		return errInRoom
	}
	code, err := room.NewCode()
	if err != nil {
		return err
	}
	secret, err := room.NewSecret()
	if err != nil {
		return err
	}
	o := a.roomOptions()
	ln, err := o.Listen()
	if err != nil {
		return fmt.Errorf("The room can't listen on the network: %w", err)
	}
	h := &hostedRoom{key: id.Fingerprint(), code: code, link: room.Invite{Code: code, Host: id.Fingerprint(), Secret: secret}.Link(), told: time.Now()}
	srv := &room.Server{Identity: id, Name: a.roomName(), Code: code, Secret: secret, Host: roomHostPlayer{a}, Clock: a.roomClock(),
		Timing: o.Timing, Changed: func() { a.roomChanged(h) }, Guessing: func() { a.notifyRooms(roomGuessed) }}
	h.server = srv
	srv.Guests = a.roomGuests()
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
		return fmt.Errorf("The room can't be announced on the network: %w", err)
	}
	h.unannounce = stop
	r.mu.Lock()
	if r.hosted != nil {
		r.mu.Unlock()
		h.close()
		return nil
	}
	r.hosted = h
	r.mu.Unlock()
	log.Printf("mixmusic: room: open, at port %d, as %s", port, srv.Name)
	a.notifyRooms("")
	return nil
}

// End ends the room this computer hosts: its guests are told so, and it
// listens no more.
func (rs *Rooms) End() {
	a := rs.a
	r := &a.room
	r.mu.Lock()
	h := r.hosted
	r.hosted = nil
	r.mu.Unlock()
	if h == nil {
		return
	}
	// A start that waits for the guests waits no more.
	r.held.release(true)
	h.close()
	log.Print("mixmusic: room: ended")
	a.notifyRooms("")
}

// Hosted returns the room this computer hosts, and whether it hosts one.
func (rs *Rooms) Hosted() (HostedRoom, bool) {
	h := rs.a.hostedRoom()
	if h == nil {
		return HostedRoom{}, false
	}
	h.codeMu.Lock()
	code, link := h.code, h.link
	h.codeMu.Unlock()
	// With the addresses that reach it from outside (roomreach.go).
	link = rs.a.reachLink(link)
	return HostedRoom{Code: code, Link: link, Guests: membersOf(h.server.Members()), Blocked: membersOf(h.server.Blocked()),
		Activity: h.server.Activity(), Waiting: h.server.Waiting()}, true
}

// NewCode gives the room this computer hosts a new code and invite link:
// the old ones let no one in from then on, a computer removed or blocked
// that comes back under another key too. The guests stay.
func (rs *Rooms) NewCode() error {
	a := rs.a
	h := a.hostedRoom()
	if h == nil {
		return nil
	}
	code, err := room.NewCode()
	if err != nil {
		return err
	}
	secret, err := room.NewSecret()
	if err != nil {
		return err
	}
	h.server.SetCode(code, secret)
	h.codeMu.Lock()
	h.code, h.link = code, room.Invite{Code: code, Host: h.key, Secret: secret}.Link()
	h.codeMu.Unlock()
	log.Print("mixmusic: room: a new code")
	a.notifyRooms("")
	return nil
}

func (a *App) hostedRoom() *hostedRoom {
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hosted
}

// Remove removes the guest of key from the room: it joins again only by
// the code or the link.
func (rs *Rooms) Remove(key string) {
	if h, fp, ok := rs.member(key); ok {
		h.server.Remove(fp)
	}
}

// Block removes the guest of key from the room, and keeps it out until
// Unblock.
func (rs *Rooms) Block(key string) {
	if h, fp, ok := rs.member(key); ok {
		h.server.Block(fp)
	}
}

// Unblock lets the computer of key join again.
func (rs *Rooms) Unblock(key string) {
	if h, fp, ok := rs.member(key); ok {
		h.server.Unblock(fp)
	}
}

func (rs *Rooms) member(key string) (*hostedRoom, remote.Fingerprint, bool) {
	h := rs.a.hostedRoom()
	fp, err := remote.ParseFingerprint(key)
	return h, fp, h != nil && err == nil
}

// roomChanged tells the windows that the guests of h changed, and what
// happened since they were told last.
func (a *App) roomChanged(h *hostedRoom) {
	h.toldMu.Lock()
	var notes []string
	for _, act := range h.server.Activity() {
		if !act.At.After(h.told) {
			continue
		}
		h.told = act.At
		switch act.What {
		case room.Joined:
			notes = append(notes, act.Name+" joined the room")
		case room.Left:
			notes = append(notes, act.Name+" left the room")
		case room.Added:
			notes = append(notes, act.Name+" added "+act.Song)
		}
	}
	h.toldMu.Unlock()
	if len(notes) == 0 {
		a.notifyRooms("")
	}
	for _, n := range notes {
		log.Printf("mixmusic: room: %s", n)
		a.notifyRooms(n)
	}
}

// roomHostPlayer is the host's player, as the server's room.Host.
type roomHostPlayer struct{ a *App }

// State is what the player plays: where its track is heard now, at this
// computer's clock's now.
func (h roomHostPlayer) State() room.State {
	a := h.a
	p := a.goPlayer.Load()
	if p == nil {
		return room.State{}
	}
	st := p.State()
	out := room.State{Volume: st.Volume, Shuffle: st.Shuffle, Repeat: repeatOf(st.Repeat), Speed: st.Speed, Live: st.Live}
	if st.Muted {
		out.Volume = 0
	}
	cur, ok := st.Current()
	if !ok {
		return out
	}
	t := queueTrackOf(cur.Track)
	out.Entry, out.Track = cur.UID, &t
	out.Playing = st.Playing || st.Loading
	out.Loading = st.Loading || st.Buffering && st.Playing || st.Waiting
	out.Position, out.At = a.heardNow(st), a.roomClock().Now()
	a.fadeHeld(&out)
	return out
}

// repeatOf is how the queue repeats, as a hand-over tells it.
func repeatOf(r player.Repeat) string {
	switch r {
	case player.RepeatAll:
		return "all"
	case player.RepeatOne:
		return "one"
	}
	return ""
}

// Queue is the player's queue around what plays, as the guests see it.
func (h roomHostPlayer) Queue() room.Queue {
	p := h.a.goPlayer.Load()
	if p == nil {
		return room.Queue{}
	}
	st := p.State()
	from := max(st.Index-room.MaxQueue/10, 0)
	to := min(from+room.MaxQueue, len(st.Queue))
	q := room.Queue{Index: max(st.Index-from, 0)}
	for _, it := range st.Queue[from:to] {
		q.Tracks = append(q.Tracks, queueTrackOf(it.Track))
		q.Entries = append(q.Entries, it.UID)
	}
	return q
}

// Watch has fn told as the player changes: another track, a play or a
// pause, the queue, or a seek, which a position far from where the track
// would be tells.
func (h roomHostPlayer) Watch(fn func()) func() {
	p := h.a.goPlayer.Load()
	if p == nil {
		return func() {}
	}
	var (
		mu       sync.Mutex
		lastPos  time.Duration
		lastAt   time.Time
		lastPlay bool
	)
	return p.Subscribe(func(e player.Event) {
		st := p.State()
		now := time.Now()
		mu.Lock()
		want := lastPos
		if lastPlay {
			want += time.Duration(float64(now.Sub(lastAt)) * max(st.Speed, 0.25))
		}
		jumped := st.Position-want > time.Second || want-st.Position > time.Second
		lastPos, lastAt, lastPlay = st.Position, now, st.Playing
		mu.Unlock()
		if e != player.Progress || jumped {
			fn()
		}
	})
}

// heardNow is where the player's track is heard now: the engine's
// position, while it plays the track, else the player's.
func (a *App) heardNow(st player.State) time.Duration {
	if o := a.opts.Room; o != nil && o.Heard != nil {
		return o.Heard(st)
	}
	if e := a.nativeEngine(); e != nil && st.Source != nil && st.Source.Mode == player.ModeStream && !st.Remote && !st.Loading {
		return e.Position()
	}
	return st.Position
}

// heldStart is the start of the host's track that waits for its guests:
// the engine has it loaded, paused, and plays it at the moment the room
// starts all of them at, unless the player loads another meanwhile.
type heldStart struct {
	mu   sync.Mutex
	seq  int
	play func() error
}

// hold keeps play for the start, and returns its number.
func (h *heldStart) hold(play func() error) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	h.play = play
	return h.seq
}

// start plays the start seq, unless another came after it.
func (h *heldStart) start(seq int) {
	h.mu.Lock()
	play := h.play
	if seq != h.seq {
		play = nil
	}
	h.play = nil
	h.mu.Unlock()
	if play != nil {
		_ = play()
	}
}

// release lets go of the start that waits: it plays now, when now is set,
// as the room ended, else not at all, as the player loads another.
func (h *heldStart) release(now bool) {
	h.mu.Lock()
	play := h.play
	h.play = nil
	h.seq++
	h.mu.Unlock()
	if now && play != nil {
		_ = play()
	}
}

// roomStream is the player's stream, whose starts wait for the guests of
// the room this computer hosts: a track the player loads to play loads
// paused, its guests ready it, and all start at one moment, a moment
// ahead (internal/room's Prepare).
type roomStream struct {
	player.Stream
	a *App
}

func (s roomStream) Load(url string, cue audio.Cue) error {
	a := s.a
	held := &a.room.held
	held.release(false)
	h := a.hostedRoom()
	if !cue.Autoplay || cue.Track == nil || h == nil || h.server.Listeners() == 0 {
		return s.Stream.Load(url, cue)
	}
	paused := cue
	paused.Autoplay = false
	if err := s.Stream.Load(url, paused); err != nil {
		return err
	}
	entry := ""
	if p := a.goPlayer.Load(); p != nil {
		if cur, ok := p.State().Current(); ok && cur.Track.ID == cue.Track.ID {
			entry = cur.UID
		}
	}
	seq := held.hold(s.Stream.Play)
	clock := a.roomClock()
	h.server.Prepare(entry, queueTrackOf(*cue.Track), cue.Start, func(at time.Duration) {
		time.AfterFunc(max(at-clock.Now(), 0), func() { held.start(seq) })
	})
	return nil
}

func (s roomStream) Unload() {
	s.a.room.held.release(false)
	s.Stream.Unload()
}

func (s roomStream) Stop() {
	s.a.room.held.release(false)
	s.Stream.Stop()
}

// SetSpeed plays the track at speed, unless this computer follows a host,
// whose speed its track takes (roomfollow.go).
func (s roomStream) SetSpeed(speed float64) {
	if s.a.room.following.Load() {
		return
	}
	s.Stream.SetSpeed(speed)
}
