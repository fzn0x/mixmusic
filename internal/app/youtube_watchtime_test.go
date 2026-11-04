package app

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// mossEpisode is a YouTube Music podcast's episode, an hour and 40
// minutes long.
var mossEpisode = media.Track{ID: "yt:ep12", Source: media.SourceYouTube, Title: "Episode 12: Moss and the long winter",
	DurationMs: 6000000, Video: true, Episode: &media.Episode{Show: media.Ref{ID: "yt:MPSPx", Name: "Moss Radio"}}}

// fakeWatchtime is a watch that keeps the reports it's given, on YouTube's
// player's schedule without its jitter.
type fakeWatchtime struct {
	mu      sync.Mutex
	reports []youtube.WatchReport
}

func (f *fakeWatchtime) Gap(n int) time.Duration {
	return []time.Duration{10, 10, 10, 40}[min(n, 3)] * time.Second
}

func (f *fakeWatchtime) Report(_ context.Context, r youtube.WatchReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, r)
	return nil
}

func (f *fakeWatchtime) got() []youtube.WatchReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.reports)
}

// watching is an App that reports the listening to episodes to fake, or
// would: signed in to YouTube Music or not, with the history setting on or
// off. Its clock stands still until the test moves it, by tick.
type watching struct {
	a     *App
	fake  *fakeWatchtime
	clock time.Time
	// starts are the videos whose watch started.
	mu     sync.Mutex
	starts []string
	// hold, when set, keeps a watch from starting until it's closed.
	hold chan struct{}
}

func newWatching(t *testing.T, signedIn, history bool) *watching {
	t.Helper()
	w := &watching{fake: &fakeWatchtime{}, clock: time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)}
	w.a = &App{youtube: youtube.New(), settings: testSettings(t, func(s *settings.Settings) { s.YouTubeHistory = history })}
	if signedIn {
		w.a.youtube.SetCookies(testCookies)
	}
	w.a.watchtime.now = func() time.Time { return w.clock }
	w.a.watchtime.start = func(_ context.Context, videoID string) (watchReporter, error) {
		w.mu.Lock()
		w.starts = append(w.starts, videoID)
		hold := w.hold
		w.mu.Unlock()
		if hold != nil {
			<-hold
		}
		return w.fake, nil
	}
	t.Cleanup(w.settle)
	return w
}

// at has the player report np, d after the clock started, with the
// history setting as the settings have it.
func (w *watching) at(d time.Duration, np NowPlaying) {
	w.clock = time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC).Add(d)
	w.a.reportWatchtime(np, w.a.settings.Get().YouTubeHistory)
}

// playing is the episode playing at the second s in, or paused there.
func episodeNow(s float64, plays bool) NowPlaying {
	ep := mossEpisode
	return NowPlaying{Track: &ep, Playing: plays, PositionMs: int(s * 1000), DurationMs: ep.DurationMs}
}

// counted waits for the watch the play's count starts.
func (w *watching) counted(t *testing.T) {
	t.Helper()
	w.a.reportYouTubePlay(mossEpisode)
	deadline := time.Now().Add(5 * time.Second)
	for {
		w.a.watchtime.mu.Lock()
		p := w.a.watchtime.cur
		ready := p != nil && (p.watch != nil || p.failed)
		w.a.watchtime.mu.Unlock()
		if ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the watch didn't start")
		}
		time.Sleep(time.Millisecond)
	}
}

// settle waits for the reports sent to go.
func (w *watching) settle() {
	w.a.watchtime.mu.Lock()
	last := w.a.watchtime.last
	w.a.watchtime.mu.Unlock()
	if last != nil {
		<-last
	}
}

func (w *watching) started() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.starts)
}

func inSeconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// stretch is what was heard from the second from to the second to.
func heardFrom(from, to float64) youtube.Stretch {
	return youtube.Stretch{From: inSeconds(from), To: inSeconds(to), Speed: 1}
}

// TestWatchtimeReports reports the listening to an episode once its play
// counts, as YouTube's player does: what was heard until then, then on its
// schedule, with what was heard, a seek apart; as it pauses and plays
// again; and a last time as another track plays.
func TestWatchtimeReports(t *testing.T) {
	w := newWatching(t, true, true)
	w.at(0, NowPlaying{Track: episodeNow(1200, false).Track, Loading: true, PositionMs: 1200000})
	w.at(0, episodeNow(1200, true))
	w.counted(t)
	for s := 1.0; s <= 14; s++ {
		w.at(inSeconds(s), episodeNow(1200+s, true))
	}
	// A seek to 25 minutes, 15 s in.
	for s := 15.0; s <= 20; s++ {
		w.at(inSeconds(s), episodeNow(1500+s-15, true))
	}
	w.at(inSeconds(21), episodeNow(1506, false))
	w.at(inSeconds(30), episodeNow(1506, false))
	w.at(inSeconds(60), episodeNow(1506, true))
	w.at(inSeconds(61), episodeNow(1507, true))
	song := media.Track{ID: "yt:song", Source: media.SourceYouTube, Title: "A made-up song", DurationMs: 200000}
	w.at(inSeconds(62), NowPlaying{Track: &song, Loading: true})
	w.settle()

	want := []youtube.WatchReport{
		{Heard: []youtube.Stretch{heardFrom(1200, 1201)}, At: inSeconds(1201), Elapsed: inSeconds(1), Next: inSeconds(10)},
		{Heard: []youtube.Stretch{heardFrom(1201, 1210)}, At: inSeconds(1210), Elapsed: inSeconds(10), Next: inSeconds(20)},
		{Heard: []youtube.Stretch{heardFrom(1210, 1214), heardFrom(1500, 1505)}, At: inSeconds(1505), Elapsed: inSeconds(20), Next: inSeconds(30)},
		{Heard: []youtube.Stretch{heardFrom(1505, 1506)}, At: inSeconds(1506), Elapsed: inSeconds(21), Paused: true},
		// Playing again, it waits the schedule's next gap: the first of 40.
		{At: inSeconds(1506), Elapsed: inSeconds(60), Next: inSeconds(100)},
		{Heard: []youtube.Stretch{heardFrom(1506, 1507)}, At: inSeconds(1507), Elapsed: inSeconds(62), Final: true},
	}
	got := w.fake.got()
	if len(got) != len(want) {
		t.Fatalf("%d reports, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !slices.Equal(got[i].Heard, want[i].Heard) || got[i].At != want[i].At || got[i].Elapsed != want[i].Elapsed ||
			got[i].Next != want[i].Next || got[i].Paused != want[i].Paused || got[i].Final != want[i].Final {
			t.Errorf("report %d = %+v\nwant %+v", i, got[i], want[i])
		}
	}
	if s := w.started(); !slices.Equal(s, []string{"ep12"}) {
		t.Errorf("watches started: %v", s)
	}
}

// TestWatchtimeSpeed tells the speed of what was heard, and a new stretch
// as it changes.
func TestWatchtimeSpeed(t *testing.T) {
	w := newWatching(t, true, true)
	w.at(0, episodeNow(60, true))
	w.counted(t)
	fast := func(s float64) NowPlaying { np := episodeNow(s, true); np.Speed = 1.5; return np }
	w.at(inSeconds(1), episodeNow(61, true))
	w.at(inSeconds(2), fast(62.5))
	w.at(inSeconds(10), fast(74.5))
	w.settle()
	got := w.fake.got()
	if len(got) != 2 {
		t.Fatalf("reports = %+v", got)
	}
	if want := []youtube.Stretch{heardFrom(61, 62.5), {From: inSeconds(62.5), To: inSeconds(74.5), Speed: 1.5}}; !slices.Equal(got[1].Heard, want) {
		t.Errorf("heard %+v, want %+v", got[1].Heard, want)
	}
}

// TestWatchtimeEndsFirst sends a playback's last report once its watch
// comes, when another track played meanwhile.
func TestWatchtimeEndsFirst(t *testing.T) {
	w := newWatching(t, true, true)
	w.hold = make(chan struct{})
	w.at(0, episodeNow(60, true))
	w.a.reportYouTubePlay(mossEpisode)
	w.at(inSeconds(5), episodeNow(65, true))
	w.at(inSeconds(6), NowPlaying{})
	close(w.hold)
	deadline := time.Now().Add(5 * time.Second)
	for len(w.fake.got()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	w.settle()
	got := w.fake.got()
	if len(got) != 1 || !got[0].Final || got[0].At != inSeconds(65) || !slices.Equal(got[0].Heard, []youtube.Stretch{heardFrom(60, 65)}) {
		t.Errorf("reports = %+v", got)
	}
}

// TestWatchtimeNotSent sends nothing signed out, with the history setting
// off, or once it turns off, nor of a song or a Spotify episode.
func TestWatchtimeNotSent(t *testing.T) {
	play := func(w *watching) {
		w.at(0, episodeNow(60, true))
		w.a.reportYouTubePlay(mossEpisode)
		for s := 1.0; s <= 30; s++ {
			w.at(inSeconds(s), episodeNow(60+s, true))
		}
		w.at(inSeconds(31), NowPlaying{})
		w.settle()
	}
	for _, c := range []struct {
		name              string
		signedIn, history bool
	}{{"signed out", false, true}, {"history off", true, false}, {"signed out, history off", false, false}} {
		w := newWatching(t, c.signedIn, c.history)
		play(w)
		if s, got := w.started(), w.fake.got(); len(s) != 0 || len(got) != 0 {
			t.Errorf("%s: watches %v, reports %+v", c.name, s, got)
		}
	}

	// Turned off as the episode plays, nothing more goes, not even its
	// last report.
	w := newWatching(t, true, true)
	w.at(0, episodeNow(60, true))
	w.counted(t)
	w.at(inSeconds(1), episodeNow(61, true))
	w.settle()
	sent := len(w.fake.got())
	w.a.settings.Update(func(s *settings.Settings) error { s.YouTubeHistory = false; return nil })
	for s := 2.0; s <= 30; s++ {
		w.at(inSeconds(s), episodeNow(60+s, true))
	}
	w.at(inSeconds(31), NowPlaying{})
	w.settle()
	if got := w.fake.got(); sent != 1 || len(got) != sent {
		t.Errorf("after the setting turned off: %d reports, then %d", sent, len(got))
	}

	// A song, a live stream and a Spotify episode aren't followed.
	w = newWatching(t, true, true)
	song := media.Track{ID: "yt:song", Source: media.SourceYouTube, Title: "A made-up song", DurationMs: 200000}
	live := episodeNow(60, true)
	live.Live = true
	sp := spotifyEpisode
	for _, np := range []NowPlaying{{Track: &song, Playing: true, PositionMs: 60000}, live, {Track: &sp, Playing: true, PositionMs: 60000}} {
		w.at(0, np)
		w.a.watchtime.mu.Lock()
		cur := w.a.watchtime.cur
		w.a.watchtime.mu.Unlock()
		if cur != nil {
			t.Errorf("%s is followed", np.Track.ID)
		}
	}
}
