// Package player plays MixMusic's queue: one queue of tracks of every
// service, played by MixMusic's own audio engine, or on a Spotify device,
// a Spotify Connect device or MixMusic itself, as each track needs. It
// plays as the web interface's player did, of which it is the port.
//
// One goroutine owns the player's state: commands and the engines' events
// take turns on it, and what waits on the network, as preparing a track,
// runs beside it and comes back to it.
package player

import (
	"context"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// Repeat is how the queue repeats.
type Repeat string

const (
	RepeatOff Repeat = "off"
	RepeatAll Repeat = "all"
	RepeatOne Repeat = "one"
)

// Mode is how a track plays.
type Mode string

const (
	// ModeStream plays a track's audio in MixMusic's own engine.
	ModeStream Mode = "stream"
	// ModeConnect plays a Spotify track on a Spotify Connect device.
	ModeConnect Mode = "connect"
	// ModeSpotify plays a Spotify track in MixMusic, as a Spotify device of
	// its own, which needs a web page: the native interface plays it from
	// YouTube Music instead.
	ModeSpotify Mode = "spotify"
)

// Source is how a track plays.
type Source struct {
	Mode Mode
	// URL is the audio of a stream.
	URL string
	// Track is the track that plays: for a Spotify track played from
	// YouTube Music, its recording there.
	Track media.Track
}

// Item is an entry of the queue.
type Item struct {
	// UID tells apart two entries of the same track.
	UID   string
	Track media.Track
	// Radio is set for tracks autoplay added after the queue ran out.
	Radio bool
}

// State is what the player does. Its Queue is shared: don't change it.
type State struct {
	Queue []Item
	// Index is the current entry's, or -1.
	Index     int
	Playing   bool
	Buffering bool
	Position  time.Duration
	Duration  time.Duration
	Shuffle   bool
	Repeat    Repeat
	// Volume is from 0 to 1, without Muted.
	Volume float64
	Muted  bool
	// Error is why the current track couldn't play.
	Error string
	// Source is how the current track plays, once prepared.
	Source *Source
}

// Current returns the current entry.
func (s State) Current() (Item, bool) {
	if s.Index < 0 || s.Index >= len(s.Queue) {
		return Item{}, false
	}
	return s.Queue[s.Index], true
}

// UpNext returns the entry that plays after the current one.
func (s State) UpNext() (Item, bool) {
	if s.Repeat == RepeatOne {
		return s.Current()
	}
	if s.Index+1 >= 0 && s.Index+1 < len(s.Queue) {
		return s.Queue[s.Index+1], true
	}
	if s.Repeat == RepeatAll && len(s.Queue) > 0 {
		return s.Queue[0], true
	}
	return Item{}, false
}

// Event is what changed.
type Event int

const (
	// TrackChange: another entry is current.
	TrackChange Event = iota
	// StateChange: the player plays, pauses or failed.
	StateChange
	// Progress: the position moved.
	Progress
	// QueueChange: the queue changed.
	QueueChange
)

// Notice is something to tell the user.
type Notice struct {
	Message string
	Error   bool
	// Settings is set when the playback settings change what it tells.
	Settings bool
}

// Now is what plays, as the app tells the tray, notifications and the
// system's media controls.
type Now struct {
	Track    *media.Track
	Playing  bool
	Position time.Duration
	Duration time.Duration
	// Volume is 0 when muted.
	Volume  float64
	Shuffle bool
	Repeat  Repeat
}

// Saved is the queue as it survives a restart.
type Saved struct {
	Tracks   []media.Track `json:"tracks"`
	Index    int           `json:"index"`
	Position int           `json:"position"` // in milliseconds
	Shuffle  bool          `json:"shuffle"`
	Repeat   Repeat        `json:"repeat"`
}

// Services are what the player asks of the app.
type Services interface {
	// Prepare finds how track plays; PrepareStream how it plays from
	// YouTube Music.
	Prepare(ctx context.Context, track media.Track) (Source, error)
	PrepareStream(ctx context.Context, track media.Track) (Source, error)
	// StreamFailure tells why the stream of track failed, or "".
	StreamFailure(track media.Track) string
	// RecordPlay counts a play of track.
	RecordPlay(track media.Track)
	// Radio returns tracks like track.
	Radio(ctx context.Context, track media.Track) ([]media.Track, error)
	Settings() settings.Settings
	// SaveVolume keeps the volume in the settings.
	SaveVolume(v float64)
	// Report tells the app what plays.
	Report(now Now)
	// SaveQueue keeps the queue for the next start.
	SaveQueue(saved Saved)
}

// Stream is MixMusic's own audio engine, as *audio.Engine, whose events
// the player takes as an audio.Events.
type Stream interface {
	Load(url string, start time.Duration, autoplay bool) error
	Preload(url string)
	Crossfade(url string, d time.Duration) error
	Play() error
	Pause()
	Stop()
	Unload()
	Seek(t time.Duration)
	SetVolume(v float64)
}

// These are the times the player keeps; tests shorten them.
var (
	// failDelay is how long a track that couldn't play shows its error
	// before the next one plays.
	failDelay = 2500 * time.Millisecond
	// reportEvery and saveEvery are how often the position is reported
	// and the queue saved, at most.
	reportEvery = time.Second
	saveEvery   = 2 * time.Second
	// saveVolumeAfter is how long the volume stays still before the
	// settings keep it.
	saveVolumeAfter = 600 * time.Millisecond
)

const (
	// maxFailures stops the queue skipping on when nothing plays, as
	// offline.
	maxFailures = 3
	// countAfter is how long a track plays before it counts as played, or
	// half of a shorter one.
	countAfter = 30 * time.Second
	// prefetchAfter is how long a track plays before the next is prepared.
	prefetchAfter = 10 * time.Second
	// restartAfter is how far into a track Previous goes back to its start.
	restartAfter = 3 * time.Second
	// radioMax is how many tracks a radio adds at most.
	radioMax = 25
	// savedMax is how many tracks of the queue are kept, at most.
	savedMax = 500
)

// Player plays the queue. Its methods are safe for concurrent use, and
// return at once: the player does what they ask on its own goroutine.
type Player struct {
	svc     Services
	stream  Stream
	connect *connectEngine
	// remote is the Spotify Connect device Spotify tracks play on, and
	// here MixMusic's own Spotify device, nil where it can't play Spotify
	// itself; connect plays on one of them.
	remote, here Device
	ctx          context.Context
	cancel       context.CancelFunc

	mu      sync.Mutex
	tasks   []func()
	closed  bool
	snap    State
	subs    []*func(Event)
	noticed []*func(Notice)
	wake    chan struct{}
	done    chan struct{}

	// What follows belongs to the player's goroutine.
	s           State
	events      []Event
	notices     []Notice
	loadToken   int
	retried     string
	counted     string
	prefetched  string
	crossfading bool
	failures    int
	// warned holds what fellBack told, to tell it once.
	warned     map[string]bool
	unshuffled []Item
	// loaded is set while the stream engine has the current track, and
	// atEnd when the queue ran out at its end, which the engine ended.
	loaded      bool
	atEnd       bool
	uidSeq      int
	lastToggle  time.Time
	reporting   throttle
	saving      throttle
	volumeTimer *time.Timer
	// spotifyMode is the settings' Spotify playback the current track
	// started with.
	spotifyMode settings.SpotifyPlayback
}

// New returns a player of the queue saved, which plays through stream and
// device. It doesn't play until asked.
func New(svc Services, stream Stream, device Device, saved Saved) *Player {
	p := &Player{
		svc:       svc,
		stream:    stream,
		wake:      make(chan struct{}, 1),
		done:      make(chan struct{}),
		reporting: throttle{every: reportEvery},
		saving:    throttle{every: saveEvery},
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.remote = device
	p.connect = &connectEngine{p: p, device: device, tickEvery: connectTick, askEvery: connectAskEvery, askDefault: connectAskEvery,
		settle: connectSettle}
	st := svc.Settings()
	p.s = State{Index: -1, Repeat: RepeatOff, Volume: st.Volume}
	p.spotifyMode = st.SpotifyPlayback
	if len(saved.Tracks) > 0 {
		p.s.Queue = make([]Item, len(saved.Tracks))
		for i, t := range saved.Tracks {
			p.s.Queue[i] = p.item(t, false)
		}
		p.s.Index = min(max(saved.Index, 0), len(p.s.Queue)-1)
		p.s.Position = time.Duration(saved.Position) * time.Millisecond
		p.s.Duration = ms(p.s.Queue[p.s.Index].Track.DurationMs)
		p.s.Shuffle = saved.Shuffle
		if saved.Repeat == RepeatAll || saved.Repeat == RepeatOne {
			p.s.Repeat = saved.Repeat
		}
	}
	p.snap = p.s
	go p.run()
	p.do(p.apply)
	return p
}

// SpotifyHere has the player play the Spotify tracks that play in
// MixMusic itself on d, MixMusic's own Spotify device; nil, as at first,
// has them play from YouTube Music.
func (p *Player) SpotifyHere(d Device) { p.do(func() { p.here = d }) }

// Close stops the player's goroutine and its following of a Spotify
// device. The engines stay as they are.
func (p *Player) Close() {
	p.do(func() { p.connect.stop(false) })
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.cancel()
	p.poke()
	<-p.done
}

// State returns what the player does.
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.snap
}

// Subscribe has fn told of every change, on the player's goroutine, until
// the returned function is called. fn must return quickly.
func (p *Player) Subscribe(fn func(Event)) (cancel func()) {
	f := &fn
	p.mu.Lock()
	p.subs = append(p.subs, f)
	p.mu.Unlock()
	return func() {
		p.mu.Lock()
		p.subs = slices.DeleteFunc(slices.Clone(p.subs), func(g *func(Event)) bool { return g == f })
		p.mu.Unlock()
	}
}

// Notices has fn told what the user should know, on the player's
// goroutine, until the returned function is called.
func (p *Player) Notices(fn func(Notice)) (cancel func()) {
	f := &fn
	p.mu.Lock()
	p.noticed = append(p.noticed, f)
	p.mu.Unlock()
	return func() {
		p.mu.Lock()
		p.noticed = slices.DeleteFunc(slices.Clone(p.noticed), func(g *func(Notice)) bool { return g == f })
		p.mu.Unlock()
	}
}

// do runs fn on the player's goroutine, after what came before it.
func (p *Player) do(fn func()) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.tasks = append(p.tasks, fn)
	p.mu.Unlock()
	p.poke()
}

func (p *Player) poke() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *Player) run() {
	defer close(p.done)
	for {
		p.mu.Lock()
		tasks := p.tasks
		p.tasks = nil
		closed := p.closed
		p.mu.Unlock()
		if len(tasks) == 0 {
			if closed {
				return
			}
			<-p.wake
			continue
		}
		for _, fn := range tasks {
			fn()
			p.publish()
		}
	}
}

// publish shows readers the state, and tells subscribers what changed.
func (p *Player) publish() {
	events, notices := p.events, p.notices
	p.events, p.notices = nil, nil
	p.mu.Lock()
	p.snap = p.s
	subs, noticed := p.subs, p.noticed
	p.mu.Unlock()
	for _, e := range events {
		for _, fn := range subs {
			(*fn)(e)
		}
	}
	for _, n := range notices {
		for _, fn := range noticed {
			(*fn)(n)
		}
	}
}

func (p *Player) emit(e Event) {
	if !slices.Contains(p.events, e) {
		p.events = append(p.events, e)
	}
}

func (p *Player) notify(n Notice) { p.notices = append(p.notices, n) }

// await runs work beside the player's goroutine, and then on it with what
// work returned. Nothing comes back once the player closed.
func await[T any](p *Player, work func(ctx context.Context) (T, error), then func(T, error)) {
	go func() {
		v, err := work(p.ctx)
		p.do(func() { then(v, err) })
	}()
}

// after runs fn on the player's goroutine after d.
func (p *Player) after(d time.Duration, fn func()) *time.Timer {
	return time.AfterFunc(d, func() { p.do(fn) })
}

// throttle runs a function at most once every interval, the last call
// included, on the player's goroutine.
type throttle struct {
	every   time.Duration
	last    time.Time
	timer   *time.Timer
	pending bool
}

func (t *throttle) call(p *Player, fn func()) {
	now := time.Now()
	if now.Sub(t.last) >= t.every {
		t.last, t.pending = now, false
		fn()
		return
	}
	t.pending = true
	if t.timer == nil {
		t.timer = p.after(t.every-now.Sub(t.last), func() {
			t.timer = nil
			if t.pending {
				t.last, t.pending = time.Now(), false
				fn()
			}
		})
	}
}

func (p *Player) item(t media.Track, radio bool) Item {
	p.uidSeq++
	return Item{UID: strconv.FormatInt(time.Now().UnixMilli(), 36) + "-" + strconv.Itoa(p.uidSeq), Track: t, Radio: radio}
}

func ms(v int) time.Duration { return time.Duration(v) * time.Millisecond }
