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
	"errors"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/volume"
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
	// Live is set for the stream of a live video, as it goes on: it has no
	// length, and can't seek.
	Live bool
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
	// Loading is set from when a track is asked to play until it plays or
	// fails: it can't be paused meanwhile, only left for another track.
	Loading bool
	// Error is why the current track couldn't play.
	Error string
	// Source is how the current track plays, once prepared.
	Source *Source
	// Waiting is set while the current track waits for the network, at
	// Position, or at the end of the queue the radio does; WillPlay tells
	// whether it plays once the network answers, which play and pause
	// change meanwhile.
	Waiting  bool
	WillPlay bool
	// Live is set while the current track is a live stream, once its
	// stream was found: it plays without a length, can't seek, and never
	// ends of itself, but when its broadcast does.
	Live bool
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
	Track   *media.Track
	Playing bool
	// Loading is set while the track is asked to play and doesn't yet: as
	// one track ends and the next loads, it is set and Playing isn't.
	Loading  bool
	Position time.Duration
	Duration time.Duration
	// Live is set for a live stream, which has no length: Position is how
	// long it played.
	Live bool
	// Volume is 0 when muted.
	Volume  float64
	Shuffle bool
	Repeat  Repeat
}

// Saved is the queue as it survives a restart.
type Saved struct {
	// Tracks is the part of the queue kept: the whole queue up to savedMax
	// tracks, else the savedMax around the current one.
	Tracks []media.Track `json:"tracks"`
	// Index is the current track's place in Tracks, or -1.
	Index    int    `json:"index"`
	Position int    `json:"position"` // in milliseconds
	Shuffle  bool   `json:"shuffle"`
	Repeat   Repeat `json:"repeat"`
	// Radio holds the indexes in Tracks of the entries a radio added.
	Radio []int `json:"radio,omitempty"`
	// Order is the queue's own order while it is shuffled, which turning
	// shuffle off puts back: the indexes in Tracks of its entries.
	Order []int `json:"order,omitempty"`
	// Unplayed holds the indexes of the entries before Index that haven't
	// played, as a list started in its middle leaves them.
	Unplayed []int `json:"unplayed,omitempty"`
}

// ErrOffline is why a track can't play while the network is gone. The
// services wrap the error they met with it, as fmt.Errorf("%w: %v",
// ErrOffline, err): the track waits for the network then.
var ErrOffline = errors.New("no network")

// Network is what the player asks of the app of the network, when its
// Services have it too: without, a track never waits for the network.
type Network interface {
	// Online reports whether the network answers.
	Online(ctx context.Context) bool
	// WaitOnline returns once the network answers, or ctx ends.
	WaitOnline(ctx context.Context) error
	// KickOnline has a WaitOnline check the network at once.
	KickOnline()
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
	// SaveVolume keeps the volume in the settings, as volume.Saved gives
	// it.
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
	// retryWaits are how long the player waits before each try of a stream
	// that broke off after it played, as when the network dropped, in
	// turn: once they ran out, the track fails.
	retryWaits = []time.Duration{time.Second, 3 * time.Second, 9 * time.Second}
	// stallCheck is how long a stream buffers before the player asks
	// whether the network answers.
	stallCheck = 8 * time.Second
	// wakeRefresh bounds how long, after the computer woke, the player
	// waits for the network to find the current track's stream again.
	wakeRefresh = 2 * time.Minute
	// slowStart is how long a stream takes to sound before the log tells.
	slowStart = 2 * time.Second
)

const (
	// maxFailures stops the queue skipping on when nothing plays, as
	// offline.
	maxFailures = 3
	// countAfter is how long a track plays before it counts as played, or
	// half of a shorter one.
	countAfter = 30 * time.Second
	// prepareAfter is how long a track plays before the next one's stream
	// is found, so that skipping to it is quick, and prefetchAfter how long
	// before its audio is downloaded too.
	prepareAfter  = time.Second
	prefetchAfter = 10 * time.Second
	// restartAfter is how far into a track Previous goes back to its start.
	restartAfter = 3 * time.Second
	// radioMax is how many tracks a radio adds at most.
	radioMax = 25
	// savedMax is how many tracks of the queue are kept, at most: the
	// current one and those around it.
	savedMax = 500
	// retryHold is how far a stream that was tried again plays on before a
	// later break has all of retryWaits again.
	retryHold = 10 * time.Second
	// savedBefore is how many of the tracks before the current one are
	// kept when those after it would fill what is kept.
	savedBefore = 50
)

// retryState is how the current load tries its stream again after it
// broke off: playAt and crossfadeTo start it over, so that each play of
// an entry has all its tries.
type retryState struct {
	// played is set once the stream played in the load. One that breaks
	// off then is tried len(retryWaits) times, after a longer wait each
	// time; one that never played is tried once, at once, as a stream
	// YouTube refused, or that can't be decoded, seldom plays a moment
	// later.
	played bool
	// n is the tries since the stream last held.
	n int
	// from is where the last try resumed, or where the user seeked to
	// since: the stream holds once it played on to retryHold past it.
	from time.Duration
}

// savedWindow returns the entries kept of a queue of n, whose current
// entry is at index, or -1: all of them up to savedMax, else savedMax
// around the current one, savedBefore of them before it and the rest
// after it, or more before it when fewer come after.
func savedWindow(n, index int) (from, to int) {
	if n <= savedMax {
		return 0, n
	}
	from = max(index-savedBefore, 0)
	to = min(from+savedMax, n)
	return max(to-savedMax, 0), to
}

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
	// version counts the states published, of which snap is the last;
	// snapOrder and snapPlayed are the player's unshuffled and played as
	// they were then, which the queue keeps with it.
	version    uint64
	snapOrder  []Item
	snapPlayed map[string]bool

	// writing orders the writes of the queue: written is the version of
	// the state the last of them kept.
	writing sync.Mutex
	written uint64

	// What follows belongs to the player's goroutine.
	s       State
	events  []Event
	notices []Notice
	// saveAsked is set once a task asked to save the queue, and saveDue
	// once the throttle lets it be written, as the task publishes it.
	saveAsked   bool
	saveDue     bool
	loadToken   int
	retry       retryState
	counted     string
	prepared    string
	prefetched  string
	crossfading bool
	failures    int
	// warned holds what fellBack told, to tell it once.
	warned map[string]bool
	// unshuffled is the queue in its own order while it is shuffled, and
	// played the entries that became current since the queue was filled,
	// by UID. Neither is changed in place, as what is published shares
	// them.
	unshuffled []Item
	played     map[string]bool
	// loaded is set while the stream engine has the current track, and
	// atEnd when the queue ran out at its end, which the engine ended.
	loaded bool
	atEnd  bool
	// paused is set while the engine holds the current track paused, as
	// the user paused it or it loaded paused: a stream that breaks off then
	// stays paused once tried again.
	paused bool
	// stopWait stops the wait for the network under way, if any; radioWaits
	// is set when the radio waits, at the end of the queue, rather than the
	// current track. toldOffline is set once the user was told that the
	// network is gone, until it answers.
	stopWait    context.CancelFunc
	radioWaits  bool
	toldOffline bool
	// stalls counts the stream's Buffering events, so that a check of the
	// network after one tells it is the last.
	stalls int
	// askedAt is when the current track was asked to play, from askedFrom,
	// until it sounds: the log tells when that took long.
	askedAt     time.Time
	askedFrom   time.Duration
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
	p.s = State{Index: -1, Repeat: RepeatOff, Volume: volume.Of(st.Volume)}
	p.spotifyMode = st.SpotifyPlayback
	if len(saved.Tracks) > 0 {
		radio := marks(saved.Radio, len(saved.Tracks))
		p.s.Queue = make([]Item, len(saved.Tracks))
		for i, t := range saved.Tracks {
			p.s.Queue[i] = p.item(t, radio[i])
		}
		p.s.Index = min(max(saved.Index, 0), len(p.s.Queue)-1)
		// The position is the saved current track's. A queue kept before
		// the index was kept within its tracks may name one past them: the
		// track held instead starts from its start.
		if saved.Index >= 0 && saved.Index < len(saved.Tracks) {
			p.s.Position = time.Duration(saved.Position) * time.Millisecond
		}
		p.s.Duration = ms(p.s.Queue[p.s.Index].Track.DurationMs)
		p.s.Shuffle = saved.Shuffle
		if saved.Repeat == RepeatAll || saved.Repeat == RepeatOne {
			p.s.Repeat = saved.Repeat
		}
		// What comes before the current entry played, unless the queue
		// says otherwise.
		unplayed := marks(saved.Unplayed, len(p.s.Queue))
		p.played = map[string]bool{}
		for i, it := range p.s.Queue[:p.s.Index+1] {
			if !unplayed[i] {
				p.played[it.UID] = true
			}
		}
		if p.s.Shuffle && len(saved.Order) > 0 {
			p.unshuffled = ordered(p.s.Queue, saved.Order)
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
			p.saveAsked = false
			fn()
			if !p.saveAsked && slices.Contains(p.events, QueueChange) {
				// An edit of the queue is kept, also while nothing plays to
				// report it.
				p.save()
			}
			p.publish()
			if p.saveDue {
				p.saveDue = false
				go p.writeQueue()
			}
		}
	}
}

// publish shows readers the state, and tells subscribers what changed.
func (p *Player) publish() {
	events, notices := p.events, p.notices
	p.events, p.notices = nil, nil
	p.mu.Lock()
	p.snap = p.s
	p.snapOrder, p.snapPlayed = p.unshuffled, p.played
	p.version++
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

// marks is which of n entries a saved queue's indexes name; it leaves out
// those past them.
func marks(indexes []int, n int) []bool {
	out := make([]bool, n)
	for _, i := range indexes {
		if i >= 0 && i < n {
			out[i] = true
		}
	}
	return out
}

// ordered is q in the order of a saved queue's Order: the entries it names,
// once each, then those it leaves out.
func ordered(q []Item, order []int) []Item {
	out := make([]Item, 0, len(q))
	seen := make([]bool, len(q))
	for _, i := range order {
		if i >= 0 && i < len(q) && !seen[i] {
			seen[i] = true
			out = append(out, q[i])
		}
	}
	for i, it := range q {
		if !seen[i] {
			out = append(out, it)
		}
	}
	return out
}

func ms(v int) time.Duration { return time.Duration(v) * time.Millisecond }
