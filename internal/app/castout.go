package app

// A cast device as the player's output: the player's stream commands become
// the session's, one after another beside the player's goroutine, and the
// device's statuses the engine's events. The device fetches the audio from
// MixMusic, through castServer.

import (
	"context"
	"errors"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// These times are the cast output's; tests shorten them.
var (
	// castTick is how often the position moves on while the device plays,
	// between its statuses, which come as it changes.
	castTick = 500 * time.Millisecond
	// castAskEvery is how often the device is asked where it is.
	castAskEvery = 5 * time.Second
	// castEcho is how long after MixMusic set the device's volume the
	// device's word of its volume is taken as the echo of it.
	castEcho = 1500 * time.Millisecond
	// castCommandTimeout bounds a command of the device.
	castCommandTimeout = 20 * time.Second
)

// castSession is what the output asks of a cast session, as *cast.Session.
type castSession interface {
	Load(ctx context.Context, m cast.Media) (cast.MediaStatus, error)
	Play(ctx context.Context) error
	Pause(ctx context.Context) error
	Stop(ctx context.Context) error
	Seek(ctx context.Context, t time.Duration) error
	SetRate(ctx context.Context, rate float64) error
	Status(ctx context.Context) error
	SetVolume(ctx context.Context, level float64) error
	SetMuted(ctx context.Context, muted bool) error
	Volume() cast.Volume
}

// castOut is a cast device as the player's stream. Its stream methods
// return at once.
type castOut struct {
	session castSession
	// served is the address the device fetches a stream's audio at, and
	// fetched how many requests of audio the device made; contentType the
	// MIME type of a stream's audio.
	served      func(path string) string
	fetched     func() int64
	contentType func(videoID string, surround bool) string
	// events is told what the device does: the player, while the output is
	// its own.
	events audio.Events
	// native tells the system's media controls whether the output has the
	// player's track, as the engine does.
	native func(bool)

	ctx    context.Context
	cancel context.CancelFunc
	queue  jobs

	mu sync.Mutex
	// media is the media session of the last load, while loading is set
	// from when a load is asked until the device answers it: what the
	// device tells of other media is stale. gen counts the loads and the
	// unloads asked: the answer to a load asked before the last is stale.
	media   int
	loading bool
	gen     int
	// loadedURL and loadedCue are the last load's, which loads again, from
	// at, to play once the media ended or stopped (idle).
	loadedURL string
	loadedCue audio.Cue
	idle      bool
	at        time.Duration
	// state, pos, rate and duration are the device's player's, as it told
	// them at posAt; playing and buffering what the player was told.
	state     cast.PlayerState
	pos       time.Duration
	posAt     time.Time
	rate      float64
	duration  time.Duration
	playing   bool
	buffering bool
	// volumeSet is when MixMusic last set the device's volume, and
	// pendingVolume the volume to set once the command before is done;
	// muted is whether the device is muted, as it last told.
	volumeSet     time.Time
	pendingVolume *float64
	muted         bool
	closed        bool
}

// newCastOut makes an output that tells events what the device does; its
// session comes before start.
func newCastOut(events audio.Events) *castOut {
	o := &castOut{events: events, rate: 1, native: func(bool) {}}
	o.ctx, o.cancel = context.WithCancel(context.Background())
	return o
}

// start runs the output's commands and ticks, on session.
func (o *castOut) start(session castSession) {
	o.session = session
	o.muted = session.Volume().Muted
	go o.queue.run(o.ctx)
	go o.tick()
}

// close stops the output: nothing is told any more.
func (o *castOut) close() {
	o.mu.Lock()
	o.closed = true
	o.mu.Unlock()
	o.cancel()
}

// do runs fn after the commands before it, with a context that bounds it.
func (o *castOut) do(fn func(ctx context.Context)) {
	o.queue.add(func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(ctx, castCommandTimeout)
		defer cancel()
		fn(ctx)
	})
}

// errLive is why a live stream doesn't play on a cast device.
var errLive = errors.New("live streams don't play on cast devices yet")

// videoOf returns the video of the address of a stream PrepareStream
// gave, and whether it is the video's 5.1 audio.
func videoOf(rawURL string) (videoID string, surround bool, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false, err
	}
	if strings.HasPrefix(u.Path, stream.LivePath("")) {
		return "", false, errLive
	}
	id, ok := strings.CutPrefix(u.Path, "/yt/")
	id, surround = strings.CutSuffix(id, stream.SurroundSuffix)
	if !ok || !stream.ValidVideoID(id) {
		return "", false, errors.New("this track's audio can't play on a cast device")
	}
	return id, surround, nil
}

func (o *castOut) Load(rawURL string, cue audio.Cue) error {
	videoID, surround, err := videoOf(rawURL)
	if err != nil {
		go o.events.Error(err.Error())
		return nil
	}
	u, _ := url.Parse(rawURL)
	m := cast.Media{URL: o.served(u.Path) + query(u), ContentType: o.contentType(videoID, surround),
		Start: cue.Start, Autoplay: cue.Autoplay}
	if t := cue.Track; t != nil {
		m.Title, m.Artist, m.Image = t.Title, t.ArtistNames(), coverOf(*t)
		if t.Album != nil {
			m.Album = t.Album.Name
		}
		m.Duration = time.Duration(t.DurationMs) * time.Millisecond
	}
	o.mu.Lock()
	o.gen++
	gen := o.gen
	o.loading, o.idle = true, false
	o.loadedURL, o.loadedCue, o.at = rawURL, cue, cue.Start
	o.pos, o.posAt, o.state = cue.Start, time.Now(), cast.Buffering
	o.mu.Unlock()
	o.native(true)
	o.load(gen, m, cue.Speed)
	return nil
}

// query is the query of u, with its "?", or "".
func query(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}
	return "?" + u.RawQuery
}

// coverOf returns the URL of the largest artwork of t the device can
// fetch, or "".
func coverOf(t media.Track) string {
	best, area := "", -1
	for _, im := range t.Artwork {
		if !strings.HasPrefix(im.URL, "https://") && !strings.HasPrefix(im.URL, "http://") {
			continue
		}
		if a := im.Width * im.Height; a > area {
			best, area = im.URL, a
		}
	}
	return best
}

// load has the device play m, at speed, as the load gen, and tells the
// player how it went, unless another load or an unload came since.
func (o *castOut) load(gen int, m cast.Media, speed float64) {
	o.do(func(ctx context.Context) {
		before := o.fetched()
		st, err := o.session.Load(ctx, m)
		o.mu.Lock()
		if o.closed || o.gen != gen {
			o.mu.Unlock()
			return
		}
		o.loading = false
		if err != nil {
			o.idle = true
			o.mu.Unlock()
			o.events.Error(o.loadFailure(err, o.fetched() > before))
			return
		}
		o.media = st.Session
		o.mu.Unlock()
		o.take(st)
		if speed > 0 && speed != 1 {
			if err := o.session.SetRate(ctx, speed); err != nil {
				log.Printf("mixmusic: the cast device didn't take the speed %v: %v", speed, err)
			}
		}
	})
}

// loadFailure tells why a load failed, in the interface's words: a device
// that never fetched the audio can't reach MixMusic.
func (o *castOut) loadFailure(err error, fetched bool) string {
	var re *cast.RequestError
	switch {
	case errors.As(err, &re) && re.Type == "LOAD_FAILED" && !fetched:
		return "The cast device couldn't reach MixMusic: let MixMusic take connections from your network in the firewall"
	case errors.As(err, &re) && re.Type == "LOAD_FAILED":
		return "The cast device couldn't play this track's audio"
	case errors.Is(err, context.DeadlineExceeded):
		return "The cast device didn't answer"
	}
	return "The cast device couldn't play the track: " + err.Error()
}

func (o *castOut) Preload(string) {}

// Crossfade plays url at once: a cast device plays one stream at a time.
// The player doesn't crossfade on one.
func (o *castOut) Crossfade(url string, _ time.Duration, cue audio.Cue) error {
	cue.Start, cue.Autoplay = 0, true
	return o.Load(url, cue)
}

func (o *castOut) Play() error {
	o.mu.Lock()
	idle, rawURL, cue := o.idle, o.loadedURL, o.loadedCue
	cue.Start, cue.Autoplay = o.at, true
	o.mu.Unlock()
	if idle && rawURL != "" {
		// The media ended, or stopped: it loads again, from where it was
		// asked to be.
		return o.Load(rawURL, cue)
	}
	o.do(func(ctx context.Context) { o.command(o.session.Play(ctx)) })
	return nil
}

func (o *castOut) Pause() { o.do(func(ctx context.Context) { o.command(o.session.Pause(ctx)) }) }

// Stop stops the media, as a Spotify device takes over: the system's
// media controls let go of it, and the player hears it paused.
func (o *castOut) Stop() {
	o.unload()
	o.native(false)
	o.mu.Lock()
	was := o.playing
	o.playing = false
	o.mu.Unlock()
	if was {
		o.events.Playing(false)
	}
}

// Unload stops the media, as the player gets another track.
func (o *castOut) Unload() { o.unload() }

func (o *castOut) unload() {
	o.mu.Lock()
	had := o.loading || o.media != 0 && !o.idle
	o.gen++
	o.media, o.idle, o.loading, o.loadedURL = 0, true, false, ""
	o.mu.Unlock()
	if had {
		o.do(func(ctx context.Context) { _ = o.session.Stop(ctx) })
	}
}

func (o *castOut) Seek(t time.Duration) {
	o.mu.Lock()
	o.at = t
	o.pos, o.posAt = t, time.Now()
	idle := o.idle
	o.mu.Unlock()
	if !idle {
		o.do(func(ctx context.Context) { o.command(o.session.Seek(ctx, t)) })
	}
}

// SetVolume sets the device's volume, unmuting it for one above 0. While a
// command of the volume waits, the last volume asked takes its place.
func (o *castOut) SetVolume(v float64) {
	o.mu.Lock()
	o.volumeSet = time.Now()
	waiting := o.pendingVolume != nil
	o.pendingVolume = &v
	o.mu.Unlock()
	if waiting {
		return
	}
	o.do(func(ctx context.Context) {
		o.mu.Lock()
		v := *o.pendingVolume
		o.pendingVolume = nil
		muted := o.muted
		o.mu.Unlock()
		if muted && v > 0 {
			if err := o.session.SetMuted(ctx, false); err == nil {
				o.mu.Lock()
				o.muted = false
				o.mu.Unlock()
			}
		}
		if err := o.session.SetVolume(ctx, v); err != nil {
			log.Printf("mixmusic: setting the cast device's volume: %v", err)
		}
		o.mu.Lock()
		o.volumeSet = time.Now()
		o.mu.Unlock()
	})
}

func (o *castOut) SetSpeed(speed float64) {
	o.do(func(ctx context.Context) {
		if speed <= 0 {
			speed = 1
		}
		if err := o.session.SetRate(ctx, speed); err != nil {
			log.Printf("mixmusic: the cast device didn't take the speed %v: %v", speed, err)
		}
	})
}

// command logs a command the device turned down: what it does tells the
// player, as it tells what the device does.
func (o *castOut) command(err error) {
	if err != nil {
		log.Printf("mixmusic: the cast device turned down a command: %v", err)
	}
}

// onMedia takes in a status of the media from the device, on the
// session's goroutine, and tells the player what changed.
func (o *castOut) onMedia(st cast.MediaStatus) {
	o.mu.Lock()
	stale := o.closed || o.loading || st.Session != o.media || o.media == 0
	o.mu.Unlock()
	if !stale {
		o.take(st)
	}
}

// take takes in a status of the media loaded.
func (o *castOut) take(st cast.MediaStatus) {
	o.mu.Lock()
	o.state, o.pos, o.posAt, o.rate = st.State, st.Position, st.At, st.Rate
	if st.Duration > 0 {
		o.duration = st.Duration
	}
	playing := st.State == cast.Playing || st.State == cast.Buffering
	buffering := st.State == cast.Buffering
	var ended, failed bool
	if st.State == cast.Idle {
		o.idle = true
		o.at = st.Position
		switch st.IdleReason {
		case cast.Finished:
			ended = true
		case cast.Failed:
			failed = true
		}
		// The media that ends is told ended alone: the next loads.
		if ended {
			o.playing = false
		}
	}
	tellBuffering := buffering != o.buffering
	tellPlaying := playing != o.playing && !ended
	o.buffering, o.playing = buffering, playing
	pos, dur := o.pos, o.duration
	o.mu.Unlock()
	switch {
	case ended:
		o.events.Ended()
		return
	case failed:
		o.events.Error("The cast device stopped playing this track's audio")
		return
	}
	if tellBuffering {
		o.events.Buffering(buffering)
	}
	if tellPlaying {
		o.events.Playing(playing)
	}
	o.events.Time(pos, dur)
}

// onVolume takes in the device's volume, which the player shows: unless it
// is the echo of the volume MixMusic just set.
func (o *castOut) onVolume(v cast.Volume, show func(level float64, muted bool)) {
	o.mu.Lock()
	o.muted = v.Muted
	echo := time.Since(o.volumeSet) < castEcho || o.pendingVolume != nil
	closed := o.closed
	o.mu.Unlock()
	if !echo && !closed {
		show(v.Level, v.Muted)
	}
}

// position is where the media is now, as the device last told it, moved
// on at its rate while it plays.
func (o *castOut) position() (time.Duration, time.Duration, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.state != cast.Playing || o.loading || o.idle || o.closed {
		return 0, 0, false
	}
	pos := o.pos + time.Duration(float64(time.Since(o.posAt))*o.rate)
	if o.duration > 0 {
		pos = min(pos, o.duration)
	}
	return pos, o.duration, true
}

// tick tells the player the position every castTick while the device
// plays, and asks the device where it is every castAskEvery.
func (o *castOut) tick() {
	t := time.NewTicker(castTick)
	defer t.Stop()
	asked := time.Now()
	for {
		select {
		case <-o.ctx.Done():
			return
		case <-t.C:
		}
		if pos, dur, ok := o.position(); ok {
			o.events.Time(pos, dur)
		}
		if time.Since(asked) >= castAskEvery {
			asked = time.Now()
			o.mu.Lock()
			has := o.media != 0 && !o.idle
			o.mu.Unlock()
			if has {
				o.do(func(ctx context.Context) { _ = o.session.Status(ctx) })
			}
		}
	}
}

// jobs is a queue of functions that run one after another, which adding to
// never waits.
type jobs struct {
	mu   sync.Mutex
	fns  []func(context.Context)
	wake chan struct{}
	once sync.Once
}

func (q *jobs) init() { q.once.Do(func() { q.wake = make(chan struct{}, 1) }) }

func (q *jobs) add(fn func(context.Context)) {
	q.init()
	q.mu.Lock()
	q.fns = append(q.fns, fn)
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// run runs the functions added, until ctx ends.
func (q *jobs) run(ctx context.Context) {
	q.init()
	for {
		q.mu.Lock()
		fns := q.fns
		q.fns = nil
		q.mu.Unlock()
		for _, fn := range fns {
			if ctx.Err() != nil {
				return
			}
			fn(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		}
	}
}

var _ player.Stream = (*castOut)(nil)
