package audio

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/internal/volume"
)

// Output is a device stream the engine renders into, as output.Stream.
type Output interface {
	// Stop stops the stream; render is not called after it returns.
	Stop()
	// Delay returns how long what render makes now waits to be heard.
	Delay() time.Duration
}

// OutputStarter starts an output that calls render for each period of the
// device, and failed when it stops of itself.
type OutputStarter func(render func([]float32), failed func(error)) (Output, error)

// Events are what the engine tells the player, as the web player's engine
// does. They are called from several goroutines, at times at once.
type Events interface {
	Time(position, duration time.Duration)
	Ended()
	Error(message string)
	Playing(playing bool)
	Buffering(buffering bool)
}

const (
	// pauseFade is how long playing and pausing fade, so as not to click.
	pauseFade = 5 * time.Millisecond
	// idleTime is how long the output plays silence before it stops, so
	// that the computer may sleep.
	idleTime = time.Second
	// tick is how often the engine reports the time while it plays, as an
	// audio element's timeupdate.
	tick = 250 * time.Millisecond
)

// ErrNoOutput is why an engine can't play when the output can't start.
var ErrNoOutput = errors.New("no audio output device")

// Engine plays tracks on two decks, so that one can fade into the next,
// each turned down to an even loudness when its track is loud, through the
// preamp, equalizer, levelling, volume and analyser of the web player: the
// API of its AudioEngine. Its methods are safe for concurrent use.
type Engine struct {
	open   Opener
	start  OutputStarter
	events Events

	// outMu serializes starting and stopping the output, which mu can't
	// cover: the output calls render, which takes mu.
	outMu sync.Mutex

	mu        sync.Mutex
	out       Output
	decks     [2]*deck
	levels    [2]ramp // fade the decks in and out as they play and pause
	active    int
	fading    bool
	fadeEnd   time.Time
	paused    bool
	normalize bool
	loudness  bool
	eq        *equalizer
	comp      *compressor
	master    smoother
	// The audio thread's buffers: a quantum, the rest of the last one, and
	// a deck's frames.
	quantumBuf [2 * quantum]float32
	pending    []float32
	deckBuf    [2 * quantum]float32
	silentFor  int64 // the frames rendered since a deck last played
	// What the audio thread saw, for the event loop.
	underrun   bool
	outFailure error

	an     *analyser
	notify chan struct{}
	closed chan struct{}
	wg     sync.WaitGroup
	// reported is what the event loop told the player last.
	reported struct {
		ended, failed *ring
		buffering     bool
	}
	lastTime atomic.Int64
}

// NewEngine returns an engine that opens tracks with open, plays them
// through outputs that start makes, and tells events what happens.
func NewEngine(open Opener, start OutputStarter, events Events) *Engine {
	e := &Engine{
		open:   open,
		start:  start,
		events: events,
		paused: true,
		eq:     newEqualizer(),
		comp:   newCompressor(),
		master: newSmoother(0.64, glide), // the gain of the volume 0.8 saved
		an:     newAnalyser(),
		notify: make(chan struct{}, 1),
		closed: make(chan struct{}),
	}
	e.wg.Add(1)
	go e.loop()
	return e
}

// Close stops the engine, its decks and its output.
func (e *Engine) Close() {
	e.Stop()
	close(e.closed)
	e.wg.Wait()
}

func (e *Engine) wake() {
	select {
	case e.notify <- struct{}{}:
	default:
	}
}

// Load plays url on the active deck from start, or holds it there until
// Play when autoplay is false. A track that Preload loaded starts at once.
func (e *Engine) Load(url string, start time.Duration, autoplay bool) error {
	e.mu.Lock()
	var stale []*deck
	e.fading = false
	other := 1 - e.active
	if d := e.decks[other]; d != nil && d.url == url && start == 0 && d.position() == 0 {
		// The preloaded track: the decks swap.
		stale = append(stale, e.decks[e.active])
		e.decks[e.active] = nil
		e.active = other
	} else {
		stale = append(stale, e.decks[e.active], e.decks[other])
		e.decks[other] = nil
		e.decks[e.active] = newDeck(url, start, e.open, e.wake)
	}
	d := e.decks[e.active]
	d.gain = ramp{value: 1, target: 1}
	e.levels[e.active] = ramp{}
	e.levels[1-e.active] = ramp{}
	e.reported.ended, e.reported.failed = nil, nil
	e.paused = true
	e.mu.Unlock()
	closeDecks(stale)
	e.wake()
	if autoplay {
		return e.Play()
	}
	return nil
}

// Preload starts loading url on the idle deck, for the next track.
func (e *Engine) Preload(url string) {
	e.mu.Lock()
	other := 1 - e.active
	if (e.fading) || (e.decks[other] != nil && e.decks[other].url == url) {
		e.mu.Unlock()
		return
	}
	stale := e.decks[other]
	e.decks[other] = newDeck(url, 0, e.open, e.wake)
	e.levels[other] = ramp{}
	e.mu.Unlock()
	closeDecks([]*deck{stale})
}

// Crossfade plays url on the idle deck while the active one fades out,
// over d: the idle deck becomes the active one.
func (e *Engine) Crossfade(url string, d time.Duration) error {
	e.mu.Lock()
	from, to := e.active, 1-e.active
	var stale []*deck
	if dk := e.decks[to]; dk == nil || dk.url != url {
		stale = append(stale, dk)
		e.decks[to] = newDeck(url, 0, e.open, e.wake)
	} else if dk.position() != 0 {
		dk.seek(0)
	}
	if e.decks[from] != nil {
		e.decks[from].gain.to(0, d)
	}
	e.decks[to].gain = ramp{}
	e.decks[to].gain.to(1, d)
	e.levels[to] = ramp{value: 1, target: 1}
	e.fading = true
	e.fadeEnd = time.Now().Add(d + 50*time.Millisecond)
	e.active = to
	e.reported.ended, e.reported.failed = nil, nil
	e.paused = true
	e.mu.Unlock()
	closeDecks(stale)
	return e.Play()
}

// Play plays the active deck, and the one fading out with it.
func (e *Engine) Play() error {
	e.mu.Lock()
	if e.decks[e.active] == nil {
		e.mu.Unlock()
		return nil
	}
	e.levels[e.active].to(1, pauseFade)
	if e.fading && e.decks[1-e.active] != nil {
		e.levels[1-e.active].to(1, pauseFade)
	}
	wasPaused := e.paused
	e.paused = false
	e.mu.Unlock()
	if err := e.ensureOutput(); err != nil {
		e.mu.Lock()
		e.levels = [2]ramp{}
		e.paused = true
		e.mu.Unlock()
		e.events.Playing(false)
		e.events.Error(err.Error())
		return err
	}
	if wasPaused {
		e.events.Playing(true)
	}
	e.wake()
	return nil
}

// Pause holds the decks where they are.
func (e *Engine) Pause() {
	e.mu.Lock()
	wasPaused := e.paused
	e.paused = true
	for i := range e.levels {
		e.levels[i].to(0, pauseFade)
	}
	e.mu.Unlock()
	if !wasPaused {
		e.events.Playing(false)
	}
	e.wake()
}

// Stop silences both decks and forgets their tracks.
func (e *Engine) Stop() {
	e.mu.Lock()
	stale := []*deck{e.decks[0], e.decks[1]}
	e.decks = [2]*deck{}
	e.levels = [2]ramp{}
	e.fading = false
	wasPaused := e.paused
	e.paused = true
	e.mu.Unlock()
	closeDecks(stale)
	e.stopOutput()
	e.an.silence()
	if !wasPaused {
		e.events.Playing(false)
	}
}

// Unload silences the active deck and forgets its track, as the player
// moves to a track it must find first, which may take a while or fail. A
// track that Preload loaded stays, for Load to take.
func (e *Engine) Unload() {
	e.mu.Lock()
	stale := []*deck{e.decks[e.active]}
	e.decks[e.active] = nil
	if e.fading {
		// The track fading out goes too.
		other := 1 - e.active
		stale = append(stale, e.decks[other])
		e.decks[other] = nil
		e.fading = false
	}
	e.levels = [2]ramp{}
	e.mu.Unlock()
	closeDecks(stale)
}

// Seek moves the active deck to t. A live stream plays on where it is.
func (e *Engine) Seek(t time.Duration) {
	e.mu.Lock()
	d := e.decks[e.active]
	if d != nil && d.live.Load() {
		d = nil
	}
	if d != nil {
		e.reported.ended, e.reported.failed = nil, nil
	}
	e.mu.Unlock()
	if d != nil {
		d.seek(t)
		e.lastTime.Store(int64(max(0, t)))
		e.wake()
	}
}

// SetVolume sets the volume, from 0 to 1, on volume's curve: each tenth of
// it is 6 dB.
func (e *Engine) SetVolume(v float64) {
	e.mu.Lock()
	e.master.target = volume.Gain(v)
	e.mu.Unlock()
}

// SetEqualizer sets the gains of the bands of Bands and the preamp, in dB,
// or makes the equalizer flat when it is off.
func (e *Engine) SetEqualizer(enabled bool, gains [len(Bands)]float64, preamp float64) {
	e.mu.Lock()
	e.eq.set(enabled, gains, preamp)
	e.mu.Unlock()
}

// SetNormalize turns the levelling of the volume on or off.
func (e *Engine) SetNormalize(on bool) {
	e.mu.Lock()
	if on && !e.normalize {
		e.comp.reset()
	}
	e.normalize = on
	e.mu.Unlock()
}

// SetLoudness turns Even loudness on or off: each track louder than
// loudnessTarget plays turned down to it, as its Measured source tells.
func (e *Engine) SetLoudness(on bool) {
	e.mu.Lock()
	e.loudness = on
	e.mu.Unlock()
}

// trimOf returns the trim of deck d, toward its target: the gain that
// turns its track down to an even loudness, while Even loudness is on and
// the track's loudness is known, else 1. A deck takes its trim at once with
// its first frames, so a track starts at its level; after, the trim glides
// to a new target, as the volume does. On the audio thread.
func (e *Engine) trimOf(d *deck, frames bool) *smoother {
	target := 1.0
	if lkfs := math.Float64frombits(d.lkfs.Load()); e.loudness && !math.IsNaN(lkfs) {
		target = math.Pow(10, loudnessTrim(lkfs)/20)
	}
	switch {
	case d.trimmed:
		d.trim.target = target
		d.trim.settled()
	case frames:
		d.trim = newSmoother(target, glide)
		d.trimmed = true
	}
	return &d.trim
}

// Analyse writes what the analyser gives now into out.
func (e *Engine) Analyse(out *Analysis) {
	e.an.analyse(out)
}

// Position returns the time of the active deck's track that is heard now.
func (e *Engine) Position() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.position()
}

func (e *Engine) position() time.Duration {
	d := e.decks[e.active]
	if d == nil {
		return 0
	}
	pos := duration(d.position())
	if e.out != nil && !e.paused {
		pos -= e.out.Delay()
	}
	return max(0, pos)
}

func closeDecks(decks []*deck) {
	for _, d := range decks {
		if d != nil {
			go d.close()
		}
	}
}

// ensureOutput starts the output if it doesn't play.
func (e *Engine) ensureOutput() error {
	e.outMu.Lock()
	defer e.outMu.Unlock()
	e.mu.Lock()
	running := e.out != nil
	e.silentFor = 0
	e.outFailure = nil
	e.mu.Unlock()
	if running {
		return nil
	}
	out, err := e.start(e.render, e.outputFailed)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.out = out
	e.mu.Unlock()
	return nil
}

// stopOutput stops the output, if it plays.
func (e *Engine) stopOutput() {
	e.outMu.Lock()
	defer e.outMu.Unlock()
	e.mu.Lock()
	out := e.out
	e.out = nil
	e.mu.Unlock()
	if out != nil {
		out.Stop()
	}
}

// outputFailed is called by an output that stopped of itself.
func (e *Engine) outputFailed(err error) {
	e.mu.Lock()
	e.outFailure = err
	e.mu.Unlock()
	e.wake()
}

// render fills out, interleaved stereo, with what the decks play: the
// output's callback, on the audio thread.
func (e *Engine) render(out []float32) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for len(out) > 0 {
		if len(e.pending) == 0 {
			e.renderQuantum()
			e.pending = e.quantumBuf[:]
		}
		n := copy(out, e.pending)
		out = out[n:]
		e.pending = e.pending[n:]
	}
}

// renderQuantum renders a quantum into quantumBuf: the decks mixed, each at
// its loudness, through the equalizer, the compressor, the volume, and into
// the analyser.
func (e *Engine) renderQuantum() {
	buf := e.quantumBuf[:]
	clear(buf)
	sounding := false
	for i, d := range e.decks {
		level := &e.levels[i]
		if d == nil || (level.value == 0 && level.target == 0) {
			continue
		}
		sounding = true
		r := d.ring.Load()
		n := r.readTo(e.deckBuf[:])
		trim := e.trimOf(d, n > 0)
		for f := range quantum {
			g := float32(d.gain.next() * level.next() * trim.next())
			if f < n {
				buf[2*f] += e.deckBuf[2*f] * g
				buf[2*f+1] += e.deckBuf[2*f+1] * g
			}
		}
		if i == e.active {
			e.underrun = n < quantum && !r.done.Load() && level.target > 0
			if n < quantum && r.done.Load() {
				e.wake() // the end, or a failure
			}
		}
	}
	if sounding {
		e.silentFor = 0
	} else {
		e.silentFor += quantum
	}
	e.eq.process(buf)
	if e.normalize {
		e.comp.process(buf, levelThreshold, levelKnee, levelRatio, levelAttack, levelRelease)
	}
	for i := 0; i < len(buf); i += 2 {
		g := float32(e.master.next())
		buf[i] *= g
		buf[i+1] *= g
	}
	e.an.writeInput(buf)
}

// loop reports the engine's state to events, and stops the output when it
// has played silence for idleTime.
func (e *Engine) loop() {
	defer e.wg.Done()
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-e.closed:
			return
		case <-t.C:
		case <-e.notify:
		}
		e.report()
	}
}

func (e *Engine) report() {
	e.mu.Lock()
	var stale *deck
	if e.fading && time.Now().After(e.fadeEnd) {
		e.fading = false
		other := 1 - e.active
		stale, e.decks[other] = e.decks[other], nil
		e.levels[other] = ramp{}
	}
	d := e.decks[e.active]
	var (
		position, total time.Duration
		ended, failed   bool
		failure         error
		outFailure      = e.outFailure
		buffering       = e.underrun && !e.paused
		idle            = e.out != nil && e.silentFor >= frames(idleTime) && !e.fading
		playing         = !e.paused
	)
	e.outFailure = nil
	if d != nil {
		position = e.position()
		total, _ = d.info()
		r := d.ring.Load()
		if r.done.Load() && r.readable() == 0 {
			if failure = d.failure(); failure != nil {
				failed = e.reported.failed != r
				e.reported.failed = r
			} else {
				ended = e.reported.ended != r && !e.fading
				if ended {
					e.reported.ended = r
				}
			}
		}
	}
	reportBuffering := buffering != e.reported.buffering
	e.reported.buffering = buffering
	if ended || failed || outFailure != nil {
		e.paused = true
		e.levels = [2]ramp{}
	}
	e.mu.Unlock()
	closeDecks([]*deck{stale})

	if outFailure != nil {
		e.stopOutput()
		e.events.Playing(false)
		e.events.Error(ErrNoOutput.Error())
		return
	}
	if d != nil && (playing || ended || time.Duration(e.lastTime.Load()) != position) {
		e.lastTime.Store(int64(position))
		e.events.Time(position, total)
	}
	if reportBuffering {
		e.events.Buffering(buffering)
	}
	switch {
	case failed:
		e.events.Playing(false)
		e.events.Error(failure.Error())
	case ended:
		e.events.Playing(false)
		e.events.Ended()
	}
	if idle {
		e.outMu.Lock()
		e.mu.Lock()
		still := e.out != nil && e.silentFor >= frames(idleTime) && !e.fading
		out := e.out
		if still {
			e.out = nil
		}
		e.mu.Unlock()
		if still {
			out.Stop()
		}
		e.outMu.Unlock()
	}
}
