package room

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// testTiming is the protocol's times, shortened.
var testTiming = Timing{Ping: 200 * time.Millisecond, Silence: 3 * time.Second, Keep: time.Second, Gate: 600 * time.Millisecond,
	Lead: 100 * time.Millisecond, Wait: 400 * time.Millisecond, Changes: 20 * time.Millisecond}

// fakeHost is a host's player: it plays a track from At, by the server's
// clock.
type fakeHost struct {
	clock Clock
	mu    sync.Mutex
	st    State
	q     Queue
	fns   []*func()
}

func (h *fakeHost) State() State {
	h.mu.Lock()
	defer h.mu.Unlock()
	st := h.st
	now := h.clock.Now()
	st.Position, st.At = st.Playhead().PositionAt(now), now
	st.Starts = false
	return st
}

func (h *fakeHost) Queue() Queue {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.q
}

func (h *fakeHost) Watch(fn func()) func() {
	f := &fn
	h.mu.Lock()
	h.fns = append(h.fns, f)
	h.mu.Unlock()
	return func() {}
}

// play plays the track of entry from from, now.
func (h *fakeHost) play(entry, title string, from time.Duration) {
	h.mu.Lock()
	h.st = State{Entry: entry, Track: &remote.QueueTrack{Source: "youtube", ID: "yt:" + entry, Title: title}, Playing: true,
		Position: from, At: h.clock.Now(), Speed: 1, Volume: 0.5}
	h.q = Queue{Tracks: []remote.QueueTrack{*h.st.Track}, Entries: []string{entry}}
	fns := h.fns
	h.mu.Unlock()
	for _, fn := range fns {
		(*fn)()
	}
}

// testRoom is a room hosted on 127.0.0.1.
type testRoom struct {
	t      *testing.T
	server *Server
	host   *fakeHost
	addr   string
	key    remote.Fingerprint
}

func newTestRoom(t *testing.T) *testRoom { return newTestRoomWith(t, nil) }

// newTestRoomWith is newTestRoom of a server that set changes before it
// serves.
func newTestRoomWith(t *testing.T, set func(*Server)) *testRoom {
	t.Helper()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	clock := NewSystemClock()
	host := &fakeHost{clock: clock}
	host.play("e1", "First song", 0)
	secret, _ := NewSecret()
	srv := &Server{Identity: id, Name: "Desk", Code: "ABC123", Secret: secret, Host: host, Clock: clock, Timing: testTiming}
	if set != nil {
		set(srv)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return &testRoom{t: t, server: srv, host: host, addr: ln.Addr().String(), key: id.Fingerprint()}
}

func newGuest(t *testing.T, name string) *Guest {
	t.Helper()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	return &Guest{Identity: id, Name: name, Clock: NewSystemClock(), Timing: testTiming}
}

// join knocks with the code and joins.
func (r *testRoom) join(g *Guest) *Session {
	r.t.Helper()
	s, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false)
	if err != nil {
		r.t.Fatalf("%s knocking: %v", g.Name, err)
	}
	if err := s.Join(context.Background()); err != nil {
		r.t.Fatalf("%s joining: %v", g.Name, err)
	}
	r.t.Cleanup(s.Close)
	return s
}

// waitFor polls cond until it holds, for up to 5 s.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("waited 5 s for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestJoinByCode joins a room by its code: the guest sees who hosts, what
// plays and how many listen first, then what plays, and who listens.
func TestJoinByCode(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Ana's laptop")
	s, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if p := s.Preview(); p.Host != "Desk" || p.Title != "First song" || p.Listeners != 0 {
		t.Fatalf("preview %+v", p)
	}
	if len(r.server.Members()) != 0 {
		t.Fatal("a guest that only knocked is among the guests")
	}
	if err := s.Join(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the state", func() bool { st, ok := s.State(); return ok && st.Entry == "e1" })
	waitFor(t, "the members", func() bool { m := s.Members(); return len(m) == 2 && m[0] == "Desk" && m[1] == "Ana's laptop" })
	waitFor(t, "the queue", func() bool { return len(s.Queue().Tracks) == 1 })
	if s.Host() != r.key {
		t.Fatal("the session's host isn't the room's key")
	}
	m := r.server.Members()
	if len(m) != 1 || m[0].Name != "Ana's laptop" || !m[0].Here || m[0].Key != g.Identity.Fingerprint() {
		t.Fatalf("members %+v", m)
	}
	if a := r.server.Activity(); len(a) != 1 || a[0].What != Joined {
		t.Fatalf("activity %+v", a)
	}
	// The host plays another track: the guests hear of it.
	r.host.play("e2", "Second song", 0)
	waitFor(t, "the next track", func() bool { st, _ := s.State(); return st.Entry == "e2" })
}

// TestWrongCodesWait refuses wrong codes, one try each, then the address
// that tried 5, even with the right code, until the wait is over.
func TestWrongCodesWait(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Guesser")
	for i := range Tries {
		if _, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC12"+string(rune('4'+i))), false); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("wrong code %d: %v", i, err)
		}
	}
	if _, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false); !errors.Is(err, ErrWait) {
		t.Fatalf("the right code after %d wrong ones: %v", Tries, err)
	}
	time.Sleep(testTiming.Wait)
	s, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false)
	if err != nil {
		t.Fatalf("after the wait: %v", err)
	}
	s.Close()
}

// TestJoinByLink joins by the invite link's secret, at the host its key
// names: another key there, or another secret, is refused.
func TestJoinByLink(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Bo")
	s, err := g.Knock(context.Background(), r.addr, &r.key, r.server.Secret, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Join(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Close()
	other, _ := remote.NewIdentity()
	key := other.Fingerprint()
	if _, err := g.Knock(context.Background(), r.addr, &key, r.server.Secret, true); !errors.Is(err, ErrNotSame) {
		t.Fatalf("another key: %v", err)
	}
	if _, err := g.Knock(context.Background(), r.addr, &r.key, []byte("ABC123"), true); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("the code as the secret: %v", err)
	}
}

// TestComeBackByKey drops a guest off: it comes back by its key, as long
// as the host keeps its place, and then only by the code.
func TestComeBackByKey(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Cy")
	s := r.join(g)
	s.Close()
	waitFor(t, "the guest away", func() bool { m := r.server.Members(); return len(m) == 1 && !m[0].Here })
	back, err := g.Rejoin(context.Background(), r.addr, r.key)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the state", func() bool { _, ok := back.State(); return ok })
	if m := r.server.Members(); len(m) != 1 || !m[0].Here {
		t.Fatalf("members %+v", m)
	}
	back.Close()
	// Gone longer than Keep, its place is no longer kept.
	waitFor(t, "the place let go", func() bool { return len(r.server.Members()) == 0 })
	if _, err := g.Rejoin(context.Background(), r.addr, r.key); !errors.Is(err, ErrNotGuest) {
		t.Fatalf("after Keep: %v", err)
	}
	stranger := newGuest(t, "Stranger")
	if _, err := stranger.Rejoin(context.Background(), r.addr, r.key); !errors.Is(err, ErrNotGuest) {
		t.Fatalf("a computer that never joined: %v", err)
	}
}

// TestRemoveAndBlock removes a guest, which joins again only by the code,
// and blocks one, which can't, until unblocked.
func TestRemoveAndBlock(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Di")
	s := r.join(g)
	r.server.Remove(g.Identity.Fingerprint())
	<-s.Done()
	if !errors.Is(s.Err(), ErrRemoved) {
		t.Fatalf("removed: %v", s.Err())
	}
	if _, err := g.Rejoin(context.Background(), r.addr, r.key); !errors.Is(err, ErrNotGuest) {
		t.Fatalf("coming back once removed: %v", err)
	}
	s = r.join(g)
	r.server.Block(g.Identity.Fingerprint())
	<-s.Done()
	if !errors.Is(s.Err(), ErrBlocked) {
		t.Fatalf("blocked: %v", s.Err())
	}
	if _, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false); !errors.Is(err, ErrBlocked) {
		t.Fatalf("knocking once blocked: %v", err)
	}
	if b := r.server.Blocked(); len(b) != 1 || b[0].Name != "Di" {
		t.Fatalf("blocked %+v", b)
	}
	r.server.Unblock(g.Identity.Fingerprint())
	r.join(g)
	want := []string{Joined, Removed, Joined, Blocked, Joined}
	a := r.server.Activity()
	if len(a) != len(want) {
		t.Fatalf("activity %+v", a)
	}
	for i := range want {
		if a[i].What != want[i] || a[i].Name != "Di" {
			t.Fatalf("activity %+v", a)
		}
	}
}

// TestLeave leaves the room: the host no longer keeps the guest.
func TestLeave(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Ed")
	s := r.join(g)
	s.Leave()
	waitFor(t, "the guest gone", func() bool { return len(r.server.Members()) == 0 })
	if a := r.server.Activity(); len(a) != 2 || a[1].What != Left {
		t.Fatalf("activity %+v", a)
	}
}

// TestEndRoom ends the room: each guest hears it, and nothing listens.
func TestEndRoom(t *testing.T) {
	r := newTestRoom(t)
	s := r.join(newGuest(t, "Fay"))
	r.server.Close()
	<-s.Done()
	if !errors.Is(s.Err(), ErrEnded) {
		t.Fatalf("ended: %v", s.Err())
	}
	if c, err := net.DialTimeout("tcp", r.addr, time.Second); err == nil {
		c.Close()
		t.Fatal("the room's port still takes connections")
	}
}

// TestPrepareStartsAllAtOneMoment changes track: the host waits for each
// guest to have it ready, then starts all Lead ahead; a guest that never
// tells is waited for no longer than Gate.
func TestPrepareStartsAllAtOneMoment(t *testing.T) {
	r := newTestRoom(t)
	var sessions []*Session
	prepared := make(chan int64, 4)
	for _, name := range []string{"Gil", "Hal"} {
		s := r.join(newGuest(t, name))
		s.Watch(func(e Event) {
			if e.Kind == Prepare && e.Entry == "e2" && e.Track.Title == "Second song" {
				prepared <- e.Seq
			}
		})
		sessions = append(sessions, s)
	}
	waitFor(t, "both guests here", func() bool { return r.server.Listeners() == 2 })
	started := make(chan time.Duration, 1)
	r.server.Prepare("e2", remote.QueueTrack{Source: "youtube", ID: "yt:e2", Title: "Second song"}, 0, func(at time.Duration) { started <- at })
	seq := <-prepared
	if <-prepared != seq {
		t.Fatal("the guests were told of other changes")
	}
	if err := sessions[0].Ready(seq); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
		t.Fatal("started before the second guest was ready")
	case <-time.After(100 * time.Millisecond):
	}
	asked := r.server.Clock.Now()
	if err := sessions[1].Ready(seq); err != nil {
		t.Fatal(err)
	}
	at := <-started
	if at < asked+testTiming.Lead || at > asked+testTiming.Lead+time.Second {
		t.Fatalf("starts at %v, asked at %v", at, asked)
	}
	for _, s := range sessions {
		waitFor(t, "the start", func() bool {
			st, _ := s.State()
			return st.Entry == "e2" && st.Starts && st.At == at && st.Seq == seq && st.Playing
		})
	}
	// One that never tells is waited for Gate.
	asked = r.server.Clock.Now()
	r.server.Prepare("e3", remote.QueueTrack{Source: "youtube", ID: "yt:e3", Title: "Third"}, 0, func(at time.Duration) { started <- at })
	at = <-started
	if at < asked+testTiming.Gate {
		t.Fatalf("started at %v, %v after asking: before Gate", at, at-asked)
	}
}

// TestPrepareWithNoGuestStartsAtOnce starts a track change at once in a
// room no guest is in.
func TestPrepareWithNoGuestStartsAtOnce(t *testing.T) {
	r := newTestRoom(t)
	done := make(chan struct{})
	r.server.Prepare("e2", remote.QueueTrack{Source: "youtube", ID: "yt:e2", Title: "Second"}, 0, func(time.Duration) { close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("it waited for guests that aren't there")
	}
}

// TestManyGuestsKnowTheHostsClock joins 100 guests, each of a clock that
// started apart, as on computers of their own: each hears where the
// host's track is, and its estimate of the host's clock is within a few
// milliseconds, under the race detector or a busy computer too.
func TestManyGuestsKnowTheHostsClock(t *testing.T) {
	// All of them come from 127.0.0.1, at once: the room takes them all.
	r := newTestRoomWith(t, func(s *Server) { s.caps = &caps{unproven: 1000, unprovenFrom: 1000, tries: 1000, roomTries: 1000} })
	hostStart := time.Now().Add(-r.server.Clock.Now())
	const guests = 100
	type joined struct {
		s     *Session
		start time.Time
	}
	out := make([]joined, guests)
	var wg sync.WaitGroup
	errs := make(chan error, guests)
	for i := range guests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := remote.NewIdentity()
			if err != nil {
				errs <- err
				return
			}
			clock := NewSystemClock()
			g := &Guest{Identity: id, Name: "Guest", Clock: clock, Timing: testTiming}
			code, _ := ParseCode("abc-123")
			s, err := g.Knock(context.Background(), r.addr, nil, []byte(code), false)
			if err == nil {
				err = s.Join(context.Background())
			}
			if err != nil {
				errs <- err
				return
			}
			out[i] = joined{s: s, start: time.Now().Add(-clock.Now())}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for _, j := range out {
		t.Cleanup(j.s.Close)
	}
	waitFor(t, "every guest here", func() bool { return r.server.Listeners() == guests })
	var worst, worstRTT time.Duration
	for _, j := range out {
		waitFor(t, "the state and the clock", func() bool {
			_, ok := j.s.State()
			_, has := j.s.Offset()
			return ok && has
		})
		// Both clocks count from their starts: the host's is ahead of the
		// guest's by how much earlier it started.
		// The estimate is within half the round trip of the ping it took,
		// however long a busy computer held it.
		off, _ := j.s.Offset()
		rtt := j.s.RoundTrip()
		d := off - j.start.Sub(hostStart)
		d = max(d, -d)
		if d > rtt/2+time.Millisecond || d > 250*time.Millisecond {
			t.Errorf("an estimate of the host's clock off by %v, over a round trip of %v", d, rtt)
		}
		worst, worstRTT = max(worst, d), max(worstRTT, rtt)
	}
	t.Logf("the worst estimate of the host's clock was off by %v; the longest round trip it took was %v", worst, worstRTT)
}
