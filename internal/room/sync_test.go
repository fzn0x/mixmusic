package room

import (
	"math/rand/v2"
	"testing"
	"time"
)

// fakeEngine plays the host's track as a guest's engine would, by the
// guest's clock: a play sounds playLag after it is asked, a seek lands
// seekLag after, and the device's clock runs at drift of the computer's,
// as a sound card's does.
type fakeEngine struct {
	pos      time.Duration
	playing  bool
	speed    float64
	drift    float64
	onDevice bool
	playLag  time.Duration
	seekLag  time.Duration

	playAt, seekAt time.Duration
	playDue        bool
	seekDue        bool
	seekTo         time.Duration
	seeks          int
	minSpeed       float64
	maxSpeed       float64
}

func newFakeEngine(pos time.Duration, playing bool) *fakeEngine {
	return &fakeEngine{pos: pos, playing: playing, speed: 1, drift: 1, playLag: 120 * time.Millisecond,
		seekLag: 80 * time.Millisecond, minSpeed: 100}
}

func (e *fakeEngine) advance(now, dt time.Duration) {
	if e.seekDue && now >= e.seekAt {
		e.pos, e.seekDue = e.seekTo, false
	}
	if e.playDue && now >= e.playAt {
		e.playing, e.playDue = true, false
	}
	if e.playing && !e.seekDue {
		e.pos += time.Duration(float64(dt) * e.speed * e.drift)
	}
}

func (e *fakeEngine) local() Local {
	return Local{Has: true, Playing: e.playing, Position: e.pos, OnDevice: e.onDevice}
}

func (e *fakeEngine) apply(now time.Duration, c Correction) {
	if c.Seek {
		e.seekDue, e.seekAt, e.seekTo = true, now+e.seekLag, c.To
		e.seeks++
	}
	if c.Pause {
		e.playing, e.playDue = false, false
	}
	if c.Play && !e.playing && !e.playDue {
		e.playDue, e.playAt = true, now+e.playLag
	}
	if c.Speed != 0 {
		if e.onDevice {
			panic("a speed was set on a Spotify device")
		}
		e.speed = c.Speed
		e.minSpeed, e.maxSpeed = min(e.minSpeed, c.Speed), max(e.maxSpeed, c.Speed)
	}
}

// world is a host and a guest, by the guest's clock: the host's clock is
// offset ahead, and the network holds each message 1 to 20 ms, each way
// apart.
type world struct {
	t        *testing.T
	rng      *rand.Rand
	offset   time.Duration
	host     Playhead
	engine   *fakeEngine
	follower Follower
	est      Offset
	// told is the playhead the guest has, which reaches it a moment after
	// the host tells it.
	told     Playhead
	tellAt   time.Duration
	pending  *Playhead
	now      time.Duration
	nextStep time.Duration
	nextPing time.Duration
	nextTell time.Duration
	due      bool
}

func newWorld(t *testing.T, seed uint64, host Playhead, engine *fakeEngine) *world {
	w := &world{t: t, rng: rand.New(rand.NewPCG(seed, 1)), offset: 3*time.Hour + 123456789, host: host, engine: engine}
	w.told = host
	return w
}

func (w *world) delay() time.Duration {
	return time.Millisecond + time.Duration(w.rng.Int64N(int64(19*time.Millisecond)))
}

// hostPos is where the host's track is heard now.
func (w *world) hostPos() time.Duration { return w.host.PositionAt(w.now + w.offset) }

// run runs the world for d in steps of a millisecond, and calls check
// after each.
func (w *world) run(d time.Duration, check func()) {
	end := w.now + d
	for w.now < end {
		w.now += time.Millisecond
		w.engine.advance(w.now, time.Millisecond)
		if w.now >= w.nextPing {
			// A ping: there, the host answers; back, the guest takes it in.
			there, back := w.delay(), w.delay()
			w.est.Add(w.now, w.now+there+w.offset, w.now+there+back)
			w.nextPing = w.now + 2*time.Second
		}
		if w.now >= w.nextTell {
			// The host tells where it is every 2 s.
			p := w.host
			p.Position, p.At = w.hostPos(), w.now+w.offset
			if w.host.waits(w.now + w.offset) {
				p = w.host
			}
			p.Starts = p.Starts && p.waits(w.now+w.offset)
			w.pending, w.tellAt = &p, w.now+w.delay()
			w.nextTell = w.now + 2*time.Second
		}
		if w.pending != nil && w.now >= w.tellAt {
			w.told, w.pending = *w.pending, nil
		}
		// The guest steps at the moment a start is due, as a timer has it.
		_, due := Due(w.now, w.estimate(), w.told)
		if w.due && !due {
			w.step()
		}
		w.due = due
		if w.now >= w.nextStep {
			w.step()
			w.nextStep = w.now + FollowEvery
		}
		if check != nil {
			check()
		}
	}
}

func (w *world) estimate() time.Duration {
	off, _, ok := w.est.Get()
	if !ok {
		w.t.Fatal("no ping came back")
	}
	return off
}

func (w *world) step() {
	w.engine.apply(w.now, w.follower.Step(w.now, w.estimate(), w.told, w.engine.local()))
}

// tell has the host tell a change at once, as it does.
func (w *world) tell(p Playhead) {
	w.host = p
	w.pending, w.tellAt = &p, w.now+w.delay()
	w.nextTell = w.now + 2*time.Second
}

func (w *world) off() time.Duration {
	d := w.engine.pos - w.hostPos()
	return max(d, -d)
}

// TestFollowerCatchesUpByPlayingFaster has a guest 300 ms behind the host
// catch up by playing faster, never more than 2 percent, without seeking,
// and stay within 50 ms once it did.
func TestFollowerCatchesUpByPlayingFaster(t *testing.T) {
	for _, ahead := range []time.Duration{-300 * time.Millisecond, 300 * time.Millisecond, 700 * time.Millisecond} {
		host := Playhead{Entry: "a", Playing: true, Position: 30 * time.Second, At: 0, Speed: 1}
		w := newWorld(t, 1, host, nil)
		w.host.At = w.offset
		w.told = w.host
		w.engine = newFakeEngine(30*time.Second+ahead, true)
		// At 2 percent, it makes up 20 ms each second.
		w.run(time.Duration(float64(max(ahead, -ahead))/MaxAdjust)+5*time.Second, nil)
		w.run(60*time.Second, func() {
			if w.off() > InStep {
				t.Fatalf("%v ahead: %v off at %v", ahead, w.off(), w.now)
			}
		})
		if w.engine.seeks != 0 {
			t.Errorf("%v ahead: %d seeks", ahead, w.engine.seeks)
		}
		if w.engine.minSpeed < 1-MaxAdjust-1e-9 || w.engine.maxSpeed > 1+MaxAdjust+1e-9 {
			t.Errorf("%v ahead: speeds from %v to %v", ahead, w.engine.minSpeed, w.engine.maxSpeed)
		}
	}
}

// TestFollowerSeeksWhenFarOff has a guest 3 s off seek once, rest, and then
// stay within 50 ms, its sound card's clock 200 ppm fast.
func TestFollowerSeeksWhenFarOff(t *testing.T) {
	host := Playhead{Entry: "a", Playing: true, Position: 10 * time.Second, Speed: 1.25}
	w := newWorld(t, 2, host, newFakeEngine(7*time.Second, true))
	w.host.At = w.offset
	w.told = w.host
	w.engine.drift = 1.0002
	w.run(SeekRest+10*time.Second, nil)
	if w.engine.seeks != 1 {
		t.Fatalf("%d seeks", w.engine.seeks)
	}
	w.run(5*time.Minute, func() {
		if w.off() > InStep {
			t.Fatalf("%v off at %v", w.off(), w.now)
		}
	})
	if w.engine.seeks != 1 {
		t.Fatalf("%d seeks in all", w.engine.seeks)
	}
	if w.engine.minSpeed < 1.25*(1-MaxAdjust)-1e-9 || w.engine.maxSpeed > 1.25*(1+MaxAdjust)+1e-9 {
		t.Errorf("speeds from %v to %v, about the host's 1.25", w.engine.minSpeed, w.engine.maxSpeed)
	}
}

// TestFollowerStartsAtTheHostsMoment has a guest wait, paused at the
// start, for the moment the host starts all at, and play from then within
// 50 ms, without seeking.
func TestFollowerStartsAtTheHostsMoment(t *testing.T) {
	w := newWorld(t, 3, Playhead{Entry: "a"}, newFakeEngine(0, false))
	w.engine.playLag = 30 * time.Millisecond
	w.run(time.Second, nil)
	start := Playhead{Entry: "a", Playing: true, Starts: true, At: w.now + w.offset + 350*time.Millisecond, Speed: 1}
	w.tell(start)
	w.run(340*time.Millisecond, func() {
		if w.engine.playing || w.engine.pos != 0 {
			t.Fatalf("played at %v before the start", w.engine.pos)
		}
	})
	w.run(30*time.Second, func() {
		if w.now+w.offset > start.At+100*time.Millisecond && w.off() > InStep {
			t.Fatalf("%v off at %v", w.off(), w.now)
		}
	})
	if w.engine.seeks != 0 {
		t.Errorf("%d seeks", w.engine.seeks)
	}
}

// TestFollowerPausesAndSeeksWithTheHost pauses a guest as the host pauses,
// where the host paused, and seeks it as the host seeks.
func TestFollowerPausesAndSeeksWithTheHost(t *testing.T) {
	w := newWorld(t, 4, Playhead{Entry: "a", Playing: true, Position: time.Minute, Speed: 1}, newFakeEngine(time.Minute, true))
	w.host.At = w.offset
	w.told = w.host
	w.run(10*time.Second, nil)
	at := w.hostPos()
	w.tell(Playhead{Entry: "a", Position: at, At: w.now + w.offset, Speed: 1})
	w.run(500*time.Millisecond, nil)
	if w.engine.playing || w.off() > InStep {
		t.Fatalf("playing %v, %v off, after the host paused", w.engine.playing, w.off())
	}
	w.tell(Playhead{Entry: "a", Playing: true, Position: 2 * time.Minute, At: w.now + w.offset, Speed: 1})
	w.run(SeekRest+10*time.Second, nil)
	if !w.engine.playing || w.off() > InStep {
		t.Fatalf("playing %v, %v off, after the host seeked", w.engine.playing, w.off())
	}
}

// TestFollowerOnlySeeksASpotifyDevice leaves a track a Spotify device
// plays at its own speed: 300 ms off stays, 2 s off seeks.
func TestFollowerOnlySeeksASpotifyDevice(t *testing.T) {
	w := newWorld(t, 5, Playhead{Entry: "a", Playing: true, Position: time.Minute, Speed: 1}, newFakeEngine(time.Minute+300*time.Millisecond, true))
	w.host.At = w.offset
	w.told = w.host
	w.engine.onDevice = true
	w.run(20*time.Second, nil)
	if w.engine.seeks != 0 || w.off() < 250*time.Millisecond {
		t.Fatalf("%d seeks, %v off", w.engine.seeks, w.off())
	}
	// A seek lands behind by how long it takes, until the next one leads
	// by that.
	for i := range 2 {
		w.engine.pos += 2 * time.Second
		w.run(SeekRest+time.Second, nil)
		if w.engine.seeks != i+1 || w.off() > 2*InStep {
			t.Fatalf("%d seeks, %v off", w.engine.seeks, w.off())
		}
	}
	if w.off() > InStep {
		t.Fatalf("%v off after a seek that led", w.off())
	}
}

// TestOffsetTakesTheQuickestAnswer estimates the host's clock from pings
// the network held a while: within half the quickest's round trip.
func TestOffsetTakesTheQuickestAnswer(t *testing.T) {
	var o Offset
	if _, _, ok := o.Get(); ok {
		t.Fatal("an offset of no ping")
	}
	truth := 5 * time.Second
	rng := rand.New(rand.NewPCG(9, 9))
	now := time.Duration(0)
	for range 20 {
		there := time.Duration(rng.Int64N(int64(80 * time.Millisecond)))
		back := time.Duration(rng.Int64N(int64(80 * time.Millisecond)))
		o.Add(now, now+there+truth, now+there+back)
		now += 2 * time.Second
	}
	off, rtt, ok := o.Get()
	if d := off - truth; !ok || d > rtt/2 || d < -rtt/2 {
		t.Fatalf("offset %v of %v, round trip %v", off, truth, rtt)
	}
	// One that came back before it left tells nothing.
	o.Add(10, 0, 5)
	if got, _, _ := o.Get(); got != off {
		t.Fatalf("offset %v after a bad ping", got)
	}
}

// TestPlayheadMovesOnWhilePlaying moves on at the host's speed from At,
// and waits at Position before it, or while paused.
func TestPlayheadMovesOnWhilePlaying(t *testing.T) {
	h := Playhead{Playing: true, Position: time.Second, At: 10 * time.Second, Speed: 2}
	for now, want := range map[time.Duration]time.Duration{
		9900 * time.Millisecond: 800 * time.Millisecond,
		5 * time.Second:         0,
		10 * time.Second:        time.Second,
		11 * time.Second:        3 * time.Second,
	} {
		if got := h.PositionAt(now); got != want {
			t.Errorf("at %v: %v, want %v", now, got, want)
		}
	}
	// A start waits at its position until it.
	h.Starts = true
	for now, want := range map[time.Duration]time.Duration{
		5 * time.Second:  time.Second,
		11 * time.Second: 3 * time.Second,
	} {
		if got := h.PositionAt(now); got != want {
			t.Errorf("start, at %v: %v, want %v", now, got, want)
		}
	}
	h.Playing = false
	if got := h.PositionAt(time.Hour); got != time.Second {
		t.Errorf("paused: %v", got)
	}
	h = Playhead{Playing: true, At: 0}
	if got := h.PositionAt(time.Second); got != time.Second {
		t.Errorf("speed 0, as its own: %v", got)
	}
}
