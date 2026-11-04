package app

// The listening to YouTube Music's episodes, reported back to the history
// of the user's account there, which YouTube Music resumes an episode from
// on any device. As YouTube's own player does, MixMusic reports a playback
// on a schedule while it plays, as it pauses and plays again, and a last
// time as another track plays or none does. It starts once the play
// counts, with the play itself, and tells what was heard until then. The
// history setting governs it, as it does the plays: off, or signed out,
// nothing goes.

import (
	"context"
	"errors"
	"log"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// seekSlack is how far from where the report before and the time since
// put it a playback may be and still have gone on: further, it moved.
const seekSlack = 3 * time.Second

// maxHeard is how many stretches heard wait for a report at most: the
// earliest go.
const maxHeard = 20

// watchtime follows the player's reports of the YouTube Music episode that
// plays, and reports its listening through a youtube.Watch.
type watchtime struct {
	mu  sync.Mutex
	cur *episodePlay
	// counted is the episode whose play counted before its playback was
	// seen, which starts its watch as it is.
	counted string
	// start starts the watch of a playback: the YouTube client's
	// StartWatch, or a test's.
	start func(ctx context.Context, videoID string) (watchReporter, error)
	// now is the clock: tests set it.
	now func() time.Time
	// last is closed once the report sent last went, or failed.
	last chan struct{}
}

// watchReporter reports a playback to YouTube Music: a youtube.Watch.
type watchReporter interface {
	Gap(n int) time.Duration
	Report(ctx context.Context, r youtube.WatchReport) error
}

// episodePlay is a playback of a YouTube Music episode, from its start
// until another track plays, or none does.
type episodePlay struct {
	trackID, videoID string
	// started is when it started, and seen when the report before was.
	started, seen time.Time
	// at is where it was at the report before; playing tells whether it
	// played, and speed at what speed.
	at      time.Duration
	playing bool
	speed   float64
	// heard is what was heard since the watch's report before, the last
	// stretch going on while it plays.
	heard []youtube.Stretch
	watch watchReporter
	// asking is set while its watch starts, failed once it couldn't, fresh
	// once it came until its first report, and dropped once the history
	// setting turned off.
	asking, failed, fresh, dropped bool
	// gaps counts the gaps of the schedule waited, and due is when the
	// next report goes: zero while none waits, as while it's paused.
	gaps int
	due  time.Time
	// warned is set once a report failed, which the log tells once.
	warned atomic.Bool
}

func (w *watchtime) clock() time.Time {
	if w.now != nil {
		return w.now()
	}
	return time.Now()
}

// watchedEpisode returns the video of the YouTube Music episode np tells
// of, or "" for any other track, a live stream, or none.
func watchedEpisode(np NowPlaying) string {
	t := np.Track
	if t == nil || !t.IsEpisode() || t.Source != media.SourceYouTube || np.Live {
		return ""
	}
	if _, id, err := media.ParseID(t.ID); err == nil {
		return id
	}
	return ""
}

// reportWatchtime follows what plays, as the player reports it, for the
// watch time of YouTube Music's episodes; on is the history setting.
func (a *App) reportWatchtime(np NowPlaying, on bool) {
	w := &a.watchtime
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock()
	if !on {
		// Turned off, nothing more goes, not even a last report.
		if w.cur != nil {
			w.cur.dropped = true
		}
		w.cur, w.counted = nil, ""
		return
	}
	id := watchedEpisode(np)
	if p := w.cur; p != nil && (id == "" || p.trackID != np.Track.ID) {
		a.endPlay(p, now)
		w.cur = nil
	}
	if id == "" {
		return
	}
	pos := time.Duration(np.PositionMs) * time.Millisecond
	if w.cur == nil {
		w.cur = &episodePlay{trackID: np.Track.ID, videoID: id, started: now, seen: now, at: pos, speed: np.speed()}
		if w.counted == np.Track.ID {
			a.startWatch(w.cur)
		}
		w.counted = ""
	}
	if np.Loading {
		return
	}
	p := w.cur
	paused, resumed := p.follow(now, pos, np.Playing, np.speed())
	if p.watch == nil {
		return
	}
	switch {
	case p.fresh:
		// Its first report: what was heard since it started, and the next
		// on the schedule from then.
		p.fresh = false
		if p.playing {
			p.due = p.started
			for p.gaps = 0; !p.due.After(now) && p.gaps < 10000; p.gaps++ {
				p.due = p.due.Add(p.watch.Gap(p.gaps))
			}
		}
	case paused:
		p.due = time.Time{}
	case resumed:
		p.due = now.Add(p.watch.Gap(p.gaps))
		p.gaps++
	case p.playing && !p.due.IsZero() && !now.Before(p.due):
		if p.due = p.due.Add(p.watch.Gap(p.gaps)); !p.due.After(now) {
			p.due = now.Add(p.watch.Gap(p.gaps))
		}
		p.gaps++
	default:
		return
	}
	a.sendWatchtime(p, p.report(now, false))
}

// follow takes the player's report that the playback is at pos, playing or
// not, at speed, now: what was heard goes on, or ends where it moved or
// paused. It tells whether it paused or played again since the report
// before.
func (p *episodePlay) follow(now time.Time, pos time.Duration, playing bool, speed float64) (paused, resumed bool) {
	if p.playing && len(p.heard) > 0 {
		last := &p.heard[len(p.heard)-1]
		expected := p.at + time.Duration(float64(now.Sub(p.seen))*p.speed)
		moved := pos-expected > seekSlack || expected-pos > seekSlack
		if moved {
			last.To = p.at
		} else {
			last.To = pos
		}
		if playing && (moved || speed != p.speed) {
			p.hear(pos, speed)
		}
	} else if playing {
		p.hear(pos, speed)
	}
	paused, resumed = p.playing && !playing, !p.playing && playing
	p.at, p.seen, p.playing, p.speed = pos, now, playing, speed
	return paused, resumed
}

// hear starts a stretch heard at pos.
func (p *episodePlay) hear(pos time.Duration, speed float64) {
	p.heard = append(p.heard, youtube.Stretch{From: pos, To: pos, Speed: speed})
	if len(p.heard) > maxHeard {
		p.heard = slices.Delete(p.heard, 0, len(p.heard)-maxHeard)
	}
}

// report is the playback's report now, the last one when final. What was
// heard goes in it, and what plays on is heard from where it is.
func (p *episodePlay) report(now time.Time, final bool) youtube.WatchReport {
	r := youtube.WatchReport{At: p.at, Elapsed: now.Sub(p.started), Paused: !p.playing, Final: final}
	for _, s := range p.heard {
		if s.To > s.From {
			r.Heard = append(r.Heard, s)
		}
	}
	if p.playing && !final && !p.due.IsZero() {
		r.Next = p.due.Sub(p.started)
	}
	p.heard = p.heard[:0]
	if p.playing && !final {
		p.hear(p.at, p.speed)
	}
	return r
}

// endPlay sends a playback's last report, as another track plays or none
// does, once its watch came.
func (a *App) endPlay(p *episodePlay, now time.Time) {
	if p.watch == nil || p.dropped {
		return
	}
	a.sendWatchtime(p, p.report(now, true))
}

// countWatchtime starts the watch of the playback of a YouTube Music
// episode whose play counted: its play joins the history with it, and its
// listening is reported from then on.
func (a *App) countWatchtime(t media.Track) {
	w := &a.watchtime
	w.mu.Lock()
	defer w.mu.Unlock()
	if p := w.cur; p != nil && p.trackID == t.ID {
		a.startWatch(p)
		return
	}
	w.counted = t.ID
}

// startWatch starts the watch of a playback, once. a.watchtime.mu is held.
func (a *App) startWatch(p *episodePlay) {
	w := &a.watchtime
	if p.watch != nil || p.asking || p.failed {
		return
	}
	p.asking = true
	start := w.start
	if start == nil {
		start = a.youtubeWatch
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		watch, err := start(ctx, p.videoID)
		w.mu.Lock()
		defer w.mu.Unlock()
		p.asking = false
		if err != nil {
			p.failed = true
			if !errors.Is(err, youtube.ErrSignedOut) && !errors.Is(err, youtube.ErrSessionRejected) {
				log.Printf("mixmusic: reporting the listening to an episode to YouTube Music: %v", err)
			}
			return
		}
		p.watch, p.fresh = watch, true
		if w.cur != p {
			// It ended while its watch started: its last report goes now.
			p.fresh = false
			a.endPlay(p, w.clock())
		}
	}()
}

// youtubeWatch starts the watch of a playback with the YouTube client.
func (a *App) youtubeWatch(ctx context.Context, videoID string) (watchReporter, error) {
	watch, err := a.youtube.StartWatch(ctx, videoID)
	if err != nil {
		return nil, err
	}
	return watch, nil
}

// sendWatchtime sends a report of a playback, after the one sent before:
// they go in their order. a.watchtime.mu is held.
func (a *App) sendWatchtime(p *episodePlay, r youtube.WatchReport) {
	w := &a.watchtime
	watch, prev, done := p.watch, w.last, make(chan struct{})
	w.last = done
	go func() {
		defer close(done)
		if prev != nil {
			<-prev
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := watch.Report(ctx, r)
		if err != nil && !errors.Is(err, youtube.ErrSessionRejected) && !p.warned.Swap(true) {
			log.Printf("mixmusic: reporting the listening to an episode to YouTube Music: %v", err)
		}
	}()
}
