package app

// What every scrobbling service's connection shares: its scrobbler, the
// loop that sends what it has waiting, and what the windows hear of it.

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/scrobble"
	"github.com/fzn0x/mixmusic/internal/store"
)

// scrobbling is a scrobbling service's connection: its scrobbler, the
// name it scrobbles as, and whether the service refused the key, which
// a.mu guards.
type scrobbling struct {
	scrobbler *scrobble.Scrobbler
	name      string
	expired   bool
	// service names the service in the log.
	service string
	sends   sendLoop
	waits   waitingTold
}

// scrobble feeds what plays to the scrobbler of each scrobbling service
// that is connected and on, which sends what it has to beside the app.
func (a *App) scrobble(np NowPlaying) {
	a.mu.Lock()
	lf, lb, fm := a.lastfm, a.listenbrainz.conn, a.librefm.conn
	a.mu.Unlock()
	if lf == nil && lb == nil && fm == nil {
		return
	}
	s := a.settings.Get()
	id, t := scrobbleTrack(np)
	if lf != nil && s.Scrobbling {
		a.feed(&lf.scrobbling, np, id, t)
	}
	if lb != nil && s.ListenBrainzScrobbling {
		a.feed(&lb.scrobbling, np, id, t)
	}
	if fm != nil && s.LibrefmScrobbling {
		a.feed(&fm.scrobbling, np, id, t)
	}
}

// feed tells sc's scrobbler what plays: the track of id, t.
func (a *App) feed(sc *scrobbling, np NowPlaying, id string, t scrobble.Track) {
	sc.scrobbler.Observe(time.Now(), id, t, np.Playing, time.Duration(np.PositionMs)*time.Millisecond, np.Speed)
	a.sendScrobbles(sc)
	a.tellWaiting(sc)
}

// The domains of the services tracks play from, as ListenBrainz hears
// them.
const (
	youtubeMusicService = "music.youtube.com"
	spotifyService      = "spotify.com"
)

// scrobbleTrack is what plays as every scrobbling service knows it, and
// its ID, "" when nothing counts: what lastfmTrack's rules say, with where
// it plays from.
func scrobbleTrack(np NowPlaying) (string, scrobble.Track) {
	id, t := lastfmTrack(np)
	if id == "" {
		return id, t
	}
	t.ID, t.ISRC = id, np.Track.ISRC
	source, native, err := media.ParseID(id)
	switch {
	case err != nil:
	case source == media.SourceYouTube:
		t.Service, t.URL = youtubeMusicService, "https://music.youtube.com/watch?v="+url.QueryEscape(native)
	case source == media.SourceSpotify:
		t.Service, t.URL = spotifyService, "https://open.spotify.com/track/"+url.PathEscape(native)
		t.SpotifyURL = t.URL
	}
	return id, t
}

// loadQueue reads the plays a scrobbler's queue kept in file, nil when it
// kept none.
func loadQueue(file string) []scrobble.Play {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var plays []scrobble.Play
	if json.Unmarshal(data, &plays) != nil {
		return nil
	}
	return plays
}

// queueSaver returns what keeps a scrobbler's queue of plays to send in
// file.
func queueSaver(file string) func([]scrobble.Play) {
	return func(plays []scrobble.Play) {
		data, err := json.Marshal(plays)
		if err == nil {
			err = store.WriteFile(file, data)
		}
		if err != nil {
			log.Printf("mixmusic: keeping the scrobbles to send: %v", err)
		}
	}
}

// sendLoop runs a scrobbler's Run beside the app, from the first play the
// app tells it of until stop.
type sendLoop struct {
	mu      sync.Mutex
	stopped bool
	cancel  context.CancelFunc
	done    chan struct{}
}

// start runs run in a goroutine of its own, once, unless the loop
// stopped.
func (l *sendLoop) start(run func(ctx context.Context)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped || l.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	l.cancel, l.done = cancel, done
	go func() {
		defer close(done)
		run(ctx)
	}()
}

// stop ends the loop and waits for it, so that it writes nothing after;
// it doesn't start again. Its caller holds no lock the loop takes.
func (l *sendLoop) stop() {
	l.mu.Lock()
	l.stopped = true
	cancel, done := l.cancel, l.done
	l.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

// waitingTold decides when the windows hear how many plays wait: at each
// change while sends fail or the service refuses the key, and once none
// wait any more. Plays that go out at once, as they do while the service
// answers, tell nothing.
type waitingTold struct {
	mu      sync.Mutex
	failing bool
	told    int
}

// changed takes in that n plays wait, and reports whether the windows
// should hear it.
func (w *waitingTold) changed(n int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if n == w.told || !w.failing && n > 0 {
		return false
	}
	w.told = n
	return true
}

// fail takes in whether the last Send failed and kept the plays.
func (w *waitingTold) fail(failing bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failing = failing
}

// sendScrobbles has sc's scrobbler send what it has waiting, beside the
// app, from now until sc stops: as plays count, when the network comes
// back, and after a failure, on a timer.
func (a *App) sendScrobbles(sc *scrobbling) {
	sc.sends.start(func(ctx context.Context) {
		if a.online != nil {
			cancel := a.online.OnChange(func(ok bool) {
				if ok {
					sc.scrobbler.Retry()
				}
			})
			defer cancel()
		}
		sc.scrobbler.Run(ctx, func(err error) { a.scrobblesSent(sc, err) })
	})
}

// scrobblesSent takes in what a Send of sc returned.
func (a *App) scrobblesSent(sc *scrobbling, err error) {
	refused := errors.Is(err, scrobble.ErrKeyRefused)
	switch {
	case refused:
		a.mu.Lock()
		told := sc.expired
		sc.expired = true
		a.mu.Unlock()
		if !told {
			notify()
		}
	case scrobble.IsRejected(err):
		log.Printf("mixmusic: %s refused a scrobble: %v", sc.service, err)
	}
	sc.waits.fail(refused || err != nil && !scrobble.IsRejected(err))
	a.tellWaiting(sc)
}

// tellWaiting tells the windows how many plays of sc wait, when they
// should hear it.
func (a *App) tellWaiting(sc *scrobbling) {
	if sc.waits.changed(sc.scrobbler.Waiting()) {
		notify()
	}
}

// userAgent is how MixMusic introduces itself to the services it asks of
// lyrics, parts to skip or scrobbles: with its version and its address.
func (a *App) userAgent() string {
	return "MixMusic/" + a.opts.Version + " (https://github.com/fzn0x/mixmusic)"
}

// stopScrobbling stops the scrobblers' sends, as the app closes.
func (a *App) stopScrobbling() {
	a.mu.Lock()
	lf, lb, fm := a.lastfm, a.listenbrainz.conn, a.librefm.conn
	a.mu.Unlock()
	if lf != nil {
		lf.sends.stop()
	}
	if lb != nil {
		lb.sends.stop()
	}
	if fm != nil {
		fm.sends.stop()
	}
}
