package app

// The lyrics of a music video, timed to it (#565). When a video's lyrics
// are its song's, from LRCLIB, timed for the song, MixMusic finds where
// the song starts in the video, comparing stretches of the two
// recordings' audio on this computer (internal/align), and moves each
// line by that shift: a video that opens with a scene before the song
// gets its lyrics in time, and one longer only by its outro keeps their
// times. Both recordings come from YouTube, a block at a time as they are
// read (stream.Ranges), never the whole song, and reading them reports
// no play. One measurement runs at a time; its result is kept in
// lyrics-timing.json in the cache folder, by video and song: a shift for
// timedFor, no clear match for unmatchedFor. A measurement that fails, as
// a download that fails or takes over timingWait, keeps nothing, and the
// video's lyrics show as the song has them for timingHold.
//
// YouTube Music's own map of a video and its song, the counterpart and
// segmentMap of the "next" answer signed in, isn't read: no recorded
// answer holds it yet.

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"math"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/internal/align"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/lyrics"
)

// timingNote is the note of a music video's lyrics while they're timed to
// it.
const timingNote = "Timing these lyrics to the video…"

// timedNote is the note of lyrics timed to a music video the song starts
// shift into.
func timedNote(shift time.Duration) string {
	s := math.Round(shift.Seconds()*10) / 10
	if s == 0 {
		s = 0 // not -0
	}
	return fmt.Sprintf("Timed to this video (%+.1f s)", s)
}

// These are the times of the timing of lyrics; tests shorten some.
var (
	// timingWait is how long a measurement takes at most.
	timingWait = 30 * time.Second
	// timingHold is how long the lyrics of a video whose measurement
	// failed show as the song has them, before one is tried again.
	timingHold = 10 * time.Minute
	// timedFor is how long a shift found is kept, and unmatchedFor that
	// the song has no clear match in the video.
	timedFor     = 180 * 24 * time.Hour
	unmatchedFor = 7 * 24 * time.Hour
)

// timingsKept is how many videos' timings the file keeps at most, the
// latest.
const timingsKept = 400

// keptTiming is a music video's timing to its song, as the file keeps it.
type keptTiming struct {
	// At is when it was measured, in Unix seconds.
	At int64 `json:"at"`
	// Matched is set when the song was found in the video, starting
	// ShiftMs milliseconds into it.
	Matched bool  `json:"matched"`
	ShiftMs int64 `json:"shiftMs,omitempty"`
}

func (k keptTiming) shift() time.Duration { return time.Duration(k.ShiftMs) * time.Millisecond }

// fresh reports whether k is still taken as it is at now.
func (k keptTiming) fresh(now time.Time) bool {
	keep := unmatchedFor
	if k.Matched {
		keep = timedFor
	}
	return now.Sub(time.Unix(k.At, 0)) < keep
}

// timingRun is a measurement of where a song starts in its music video:
// of the video, the track trackID, and the song, by their video IDs.
type timingRun struct {
	key, trackID, video, song string
	// open opens the audio of a video.
	open func(ctx context.Context, videoID string) (io.ReadSeekCloser, error)
	// ctx ends timingWait after the run starts, or as cancel cancels it;
	// done closes as the run ends.
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// lyricsTiming is what times the lyrics of music videos to them: the
// timings found, the measurement under way and the one asked for after
// it, and who is told as one ends.
type lyricsTiming struct {
	// open opens the audio of a video, to read stretches of it: the
	// proxy's, but in tests.
	open func(ctx context.Context, videoID string) (io.ReadSeekCloser, error)
	// version counts the measurements started and ended, which a lookup of
	// lyrics that saw none of them change may keep.
	version atomic.Int64

	mu      sync.Mutex
	loaded  bool
	file    *store.File[map[string]keptTiming]
	kept    map[string]keptTiming
	held    map[string]time.Time
	running *timingRun
	next    *timingRun
	told    []*func(trackID string)
	// closed is set once the app closes: no measurement starts then.
	closed bool
}

// timingState is how a music video's lyrics are timed to it.
type timingState int

const (
	// timingNone: they keep the song's times, as the switch is off, the
	// song has no clear match in the video, or it can't be measured now.
	timingNone timingState = iota
	// timingFound: moved by the shift found.
	timingFound
	// timingUnderWay: being measured.
	timingUnderWay
)

// OnLyricsTimed has fn told, from any goroutine, the ID of a music video
// whose lyrics change as the measurement of their timing ends, until
// cancel: the windows look them up again then.
func (a *App) OnLyricsTimed(fn func(trackID string)) (cancel func()) {
	t := &a.lyricsTiming
	f := &fn
	t.mu.Lock()
	t.told = append(slices.Clone(t.told), f)
	t.mu.Unlock()
	return func() {
		t.mu.Lock()
		t.told = slices.DeleteFunc(slices.Clone(t.told), func(g *func(string)) bool { return g == f })
		t.mu.Unlock()
	}
}

// videoLyrics returns the synced lyrics l of song, a music video's song,
// as they show with the video: moved by where the song starts in it once
// that is measured, with a note that says so; while it is, as the song
// has them, with timingNote; else as the song has them while they keep
// time with the video, and without their times, with videoNote, when
// they don't.
func (c *Catalog) videoLyrics(ctx context.Context, video, song media.Track, l lyrics.Lyrics) lyrics.Lyrics {
	if c.a.settings.Get().LyricsTimeVideos {
		switch shift, state := c.a.videoTiming(video, song); state {
		case timingFound:
			l = l.Shifted(shift)
			l.Note = timedNote(shift)
			return l
		case timingUnderWay:
			if !c.keepsTime(ctx, video, song) {
				l = l.Untimed()
			}
			l.Note = timingNote
			return l
		}
	}
	if !c.keepsTime(ctx, video, song) {
		l = l.Untimed()
		l.Note = videoNote
	}
	return l
}

// videoTiming returns how the lyrics of song are timed to video, its music
// video: the shift found, else whether it is being measured, which it
// starts when nothing is known of it. A measurement asked for while
// another runs waits for it while the other's video plays, and the other
// gives way else; one that waited gives way to it.
func (a *App) videoTiming(video, song media.Track) (time.Duration, timingState) {
	_, videoID, err := media.ParseID(video.ID)
	_, songID, err2 := media.ParseID(song.ID)
	if err != nil || err2 != nil || !stream.ValidVideoID(videoID) || !stream.ValidVideoID(songID) {
		return 0, timingNone
	}
	playing := a.playingID()
	t := &a.lyricsTiming
	t.mu.Lock()
	shift, state, dropped := a.timingLocked(videoID+" "+songID, video.ID, videoID, songID, playing)
	told := t.told
	t.mu.Unlock()
	if dropped != nil {
		// Its lyrics show as the song has them once looked up again.
		for _, fn := range told {
			(*fn)(dropped.trackID)
		}
	}
	return shift, state
}

// timingLocked is videoTiming of the timing of key, with t.mu held, which
// returns the measurement that waited and gives way, if any.
func (a *App) timingLocked(key, trackID, videoID, songID, playing string) (time.Duration, timingState, *timingRun) {
	t := &a.lyricsTiming
	a.loadTimings()
	now := time.Now()
	if k, ok := t.kept[key]; ok && k.fresh(now) {
		if k.Matched {
			return k.shift(), timingFound, nil
		}
		return 0, timingNone, nil
	}
	for _, r := range []*timingRun{t.running, t.next} {
		if r != nil && r.key == key {
			return 0, timingUnderWay, nil
		}
	}
	if until, ok := t.held[key]; ok && now.Before(until) || t.closed {
		return 0, timingNone, nil
	}
	open := t.open
	if open == nil && a.proxy != nil {
		open = func(ctx context.Context, videoID string) (io.ReadSeekCloser, error) {
			return a.proxy.OpenRanges(ctx, videoID)
		}
	}
	if open == nil {
		return 0, timingNone, nil
	}
	r := &timingRun{key: key, trackID: trackID, video: videoID, song: songID, open: open}
	if t.running == nil {
		t.start(a, r)
		return 0, timingUnderWay, nil
	}
	if t.running.trackID != playing {
		t.running.cancel() // it no longer plays: it gives way
	}
	dropped := t.next
	t.next = r
	return 0, timingUnderWay, dropped
}

// lyricsStay reports whether the lyrics of the track trackID, looked up
// since the timing's version was seen, may be kept: no measurement of a
// music video's timing started or ended meanwhile, nor is one under way of
// that track's, whose lyrics change as it ends.
func (a *App) lyricsStay(trackID string, seen int64) bool {
	t := &a.lyricsTiming
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, r := range []*timingRun{t.running, t.next} {
		if r != nil && r.trackID == trackID {
			return false
		}
	}
	return t.version.Load() == seen
}

// playingID returns the ID of the track that plays, "" for none.
func (a *App) playingID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if np := a.nowPlaying.Track; np != nil {
		return np.ID
	}
	return ""
}

// loadTimings reads the timings kept, once. Its caller holds t.mu.
func (a *App) loadTimings() {
	t := &a.lyricsTiming
	if t.loaded {
		return
	}
	t.loaded = true
	t.kept, t.held = map[string]keptTiming{}, map[string]time.Time{}
	if a.opts.CacheDir == "" {
		return // a test's app keeps them in memory
	}
	f, err := store.Open(filepath.Join(a.opts.CacheDir, "lyrics-timing.json"), map[string]keptTiming{})
	if err != nil {
		log.Printf("mixmusic: reading the timings of lyrics to music videos: %v", err)
		return
	}
	t.file = f
	maps.Copy(t.kept, f.Get())
}

// start runs r. Its caller holds t.mu.
func (t *lyricsTiming) start(a *App, r *timingRun) {
	r.ctx, r.cancel = context.WithTimeout(context.Background(), timingWait)
	r.done = make(chan struct{})
	t.running = r
	t.version.Add(1)
	go a.runTiming(r)
}

// runTiming measures where r's song starts in its video, keeps what it
// found, starts the measurement asked for meanwhile, and tells the
// windows the video's lyrics changed.
func (a *App) runTiming(r *timingRun) {
	defer close(r.done)
	defer r.cancel()
	start := time.Now()
	found, err := measureTiming(r.ctx, r.open, r.video, r.song)
	t := &a.lyricsTiming
	t.mu.Lock()
	now := time.Now()
	stored := true
	switch {
	case err == nil:
		t.kept[r.key] = keptTiming{At: now.Unix(), Matched: true, ShiftMs: found.Shift.Milliseconds()}
		log.Printf("mixmusic: timed the lyrics of %s to it in %v: its song, %s, starts %+.3fs into it (%s)", r.video, time.Since(start).Round(time.Millisecond), r.song, found.Shift.Seconds(), found.Windows)
	case errors.Is(err, align.ErrNoMatch):
		t.kept[r.key] = keptTiming{At: now.Unix()}
		log.Printf("mixmusic: the lyrics of %s keep the times of its song, %s: %v", r.video, r.song, err)
	case errors.Is(r.ctx.Err(), context.Canceled):
		stored = false // another track's came first: the next play tries again
	default:
		stored = false
		maps.DeleteFunc(t.held, func(_ string, until time.Time) bool { return now.After(until) })
		t.held[r.key] = now.Add(timingHold)
		log.Printf("mixmusic: timing the lyrics of %s to it: %v", r.video, err)
	}
	var save map[string]keptTiming
	if stored && t.file != nil {
		save = t.latest(now)
	}
	if t.running == r {
		t.running = nil
		if next := t.next; next != nil && !t.closed {
			t.next = nil
			t.start(a, next)
		}
	}
	t.version.Add(1)
	told := t.told
	file := t.file
	t.mu.Unlock()
	if save != nil {
		if err := file.Set(save); err != nil {
			log.Printf("mixmusic: keeping the timings of lyrics to music videos: %v", err)
		}
	}
	for _, fn := range told {
		(*fn)(r.trackID)
	}
}

// stopLyricsTiming stops the measurement under way, and drops the one
// that waits, as the app closes: it waits for the one under way to end,
// so that nothing is written once the app's folders may go.
func (a *App) stopLyricsTiming() {
	t := &a.lyricsTiming
	t.mu.Lock()
	t.closed, t.next = true, nil
	r := t.running
	if r != nil {
		r.cancel()
	}
	t.mu.Unlock()
	if r != nil {
		<-r.done
	}
}

// latest returns the timings to keep in the file: the fresh ones,
// timingsKept of them at most, the latest. Its caller holds t.mu.
func (t *lyricsTiming) latest(now time.Time) map[string]keptTiming {
	maps.DeleteFunc(t.kept, func(_ string, k keptTiming) bool { return !k.fresh(now) })
	keys := slices.Collect(maps.Keys(t.kept))
	slices.SortFunc(keys, func(a, b string) int { return cmp.Compare(t.kept[b].At, t.kept[a].At) })
	for _, k := range keys[min(len(keys), timingsKept):] {
		delete(t.kept, k)
	}
	return maps.Clone(t.kept)
}

// measureTiming finds where the song starts in the video, by their video
// IDs, reading of each the stretches align's plan asks for, the song's
// first, and closing its audio once they're read.
func measureTiming(ctx context.Context, open func(context.Context, string) (io.ReadSeekCloser, error), videoID, songID string) (align.Result, error) {
	song, songAudio, err := openStretches(ctx, open, songID)
	if err != nil {
		return align.Result{}, fmt.Errorf("the song's audio: %w", err)
	}
	defer song.Close()
	video, videoAudio, err := openStretches(ctx, open, videoID)
	if err != nil {
		return align.Result{}, fmt.Errorf("the video's audio: %w", err)
	}
	defer video.Close()
	plan, ok := align.NewPlan(songAudio.Length(), videoAudio.Length())
	if !ok {
		return align.Result{}, fmt.Errorf("%w: a song of %v, a video of %v", align.ErrNoMatch, songAudio.Length(), videoAudio.Length())
	}
	windows := make([]align.Envelope, len(plan.Windows))
	for i, w := range plan.Windows {
		if windows[i], err = envelopeOf(songAudio, w); err != nil {
			return align.Result{}, fmt.Errorf("the song's audio: %w", err)
		}
	}
	song.Close()
	whole, err := envelopeOf(videoAudio, plan.Video)
	if err != nil {
		return align.Result{}, fmt.Errorf("the video's audio: %w", err)
	}
	return plan.Shift(windows, whole)
}

// openStretches opens the audio of a video to read stretches of it.
func openStretches(ctx context.Context, open func(context.Context, string) (io.ReadSeekCloser, error), videoID string) (io.Closer, *audio.Stretches, error) {
	src, err := open(ctx, videoID)
	if err != nil {
		return nil, nil, err
	}
	s, err := audio.OpenStretches(src)
	if err != nil {
		src.Close()
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, err
	}
	return src, s, nil
}

// envelopeOf returns the envelope of the stretch w of s.
func envelopeOf(s *audio.Stretches, w align.Stretch) (align.Envelope, error) {
	l := align.NewLevels(audio.Rate, w.From)
	err := s.Read(w.From, w.To, l.Write)
	return l.Envelope(), err
}
