package main

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// slowPages is a service's playlist of tracks, read two at a time, a page
// once the test lets it: nil reads the page, an error fails it. key is the
// fetch of the page whose reading the test follows.
type slowPages struct {
	tracks []media.Track
	gate   chan error
	key    string
}

func (f *slowPages) read(ctx context.Context, token string) ([]media.Track, string, error) {
	select {
	case err := <-f.gate:
		if err != nil {
			return nil, "", err
		}
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	from, _ := strconv.Atoi(token)
	to := min(from+2, len(f.tracks))
	next := ""
	if to < len(f.tracks) {
		next = strconv.Itoa(to)
	}
	return f.tracks[from:to], next, nil
}

const longPlaylistID = "yt:PLlong"

// slowReading has the shell read the playlists of the six test tracks
// whose first page holds two, as a playlist's are read, a page that fails
// tried again, the pages after it coming as the test lets them. It follows
// the reading of the fetch of key.
func slowReading(t *testing.T, s *shell, key string) *slowPages {
	t.Helper()
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	f := &slowPages{tracks: testTracks, gate: make(chan error), key: key}
	read := readPlaylistOn
	readPlaylistOn = func(s *shell, ctx context.Context, p media.Playlist, onPage func(media.Playlist)) (media.Playlist, error) {
		err := p.ReadOn(ctx, f.read, onPage)
		return p, err
	}
	t.Cleanup(func() {
		readPlaylistOn, media.PageWaits = read, waits
		if r := f.reading(s); r != nil && r.cancel != nil {
			r.cancel()
		}
	})
	return f
}

// longHead is the first page of the playlist of slowReading.
func longHead(id string) *loaded[media.Playlist] {
	return &loaded[media.Playlist]{done: true, value: media.Playlist{ID: id, Source: media.SourceYouTube, Title: "Long",
		Owner: "Someone", TrackCount: len(testTracks), Tracks: testTracks[:2], More: "2"}}
}

// longPage shows the page of the playlist of slowReading.
func longPage(t *testing.T) (*shell, *ui.Tester, *slowPages) {
	t.Helper()
	s := newTestShell(t)
	key := "playlist:" + longPlaylistID
	f := slowReading(t, s, key)
	s.loads[key] = longHead(longPlaylistID)
	tt := tester(t, s)
	show(s, tt, playlistPath(longPlaylistID))
	return s, tt, f
}

// reading is the reading the test follows.
func (f *slowPages) reading(s *shell) *playlistRead {
	r, _ := s.loads[f.key+" read"].(*playlistRead)
	return r
}

// let lets the reading of the page take the next page, or a try of it fail
// with err.
func (f *slowPages) let(t *testing.T, err error) {
	t.Helper()
	select {
	case f.gate <- err:
	case <-time.After(5 * time.Second):
		t.Fatal("nothing reads the playlist")
	}
}

// page lets the next page come, and draws the page once it took it.
func (f *slowPages) page(t *testing.T, s *shell, tt *ui.Tester) {
	t.Helper()
	r := f.reading(s)
	n := len(r.p.Tracks)
	f.let(t, nil)
	waitFor(t, s, func() bool { return len(r.p.Tracks) > n || !r.reading })
	tt.Frame()
}

// fail fails each try of the next page with err, and draws the page once
// it took the failure.
func (f *slowPages) fail(t *testing.T, s *shell, tt *ui.Tester, err error) {
	t.Helper()
	for range len(media.PageWaits) + 1 {
		f.let(t, err)
	}
	r := f.reading(s)
	waitFor(t, s, func() bool { return r.err != nil })
	tt.Frame()
}

// waitFor runs what the reading told the page until ok.
func waitFor(t *testing.T, s *shell, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the page didn't take what the reading told it")
		}
		s.drain()
	}
}

// TestLongPlaylistReadsOn shows a playlist's first page of tracks at once,
// says that the rest load, and shows them as they come.
func TestLongPlaylistReadsOn(t *testing.T) {
	s, tt, f := longPage(t)
	if !tt.HasText("Loading tracks · 2 of 6") || !tt.HasText(testTracks[1].Title) || tt.HasText(testTracks[2].Title) {
		t.Fatalf("the first page shows %q", tt.Texts())
	}
	if tt.HasText("7 min") {
		t.Errorf("the length of the first page shows as the playlist's: %q", tt.Texts())
	}
	f.page(t, s, tt)
	if !tt.HasText("Loading tracks · 4 of 6") || !tt.HasText(testTracks[3].Title) {
		t.Errorf("the second page shows %q", tt.Texts())
	}
	f.page(t, s, tt)
	if tt.HasText("Loading tracks") || !tt.HasText(testTracks[5].Title) || !tt.HasText("21 min") {
		t.Errorf("the whole playlist shows %q", tt.Texts())
	}
}

// TestLongPlaylistPlayWaits has Shuffle, pressed while the tracks load,
// shuffle them all once they're here, and Play play them at once after.
func TestLongPlaylistPlayWaits(t *testing.T) {
	s, tt, f := longPage(t)
	click(t, s, tt, "shuffle")
	if !tt.HasText("Loading tracks · 2 of 6 · shuffles once they're all here") {
		t.Errorf("the page shows %q", tt.Texts())
	}
	if q := s.p.State().Queue; len(q) != 0 {
		t.Fatalf("shuffled the first page: %d tracks", len(q))
	}
	f.page(t, s, tt)
	f.page(t, s, tt)
	s.settle(t, func(st player.State) bool { return len(st.Queue) == len(testTracks) && st.Shuffle && st.Playing })
	tt.Frame()
	s.p.SetShuffle(false)
	click(t, s, tt, "play")
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == testTracks[0].ID && len(st.Queue) == len(testTracks) && !st.Shuffle && st.Playing
	})
}

// TestLongPlaylistPlayGivesWay leaves what the user played while the
// tracks Play waited for loaded.
func TestLongPlaylistPlayGivesWay(t *testing.T) {
	s, tt, f := longPage(t)
	click(t, s, tt, "play")
	s.p.PlayTracks(testTracks[5:], 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	f.page(t, s, tt)
	f.page(t, s, tt)
	s.drain()
	if q := s.p.State().Queue; len(q) != 1 {
		t.Errorf("the playlist took the place of what the user played: %d tracks", len(q))
	}
}

// TestLongPlaylistPlayWaitsThroughRefresh plays what Play waited for once
// a refresh of the page, which reads it again, has it.
func TestLongPlaylistPlayWaitsThroughRefresh(t *testing.T) {
	s, tt, _ := longPage(t)
	click(t, s, tt, "play")
	head := s.loads["playlist:"+longPlaylistID].(*loaded[media.Playlist])
	head.value.Tracks, head.value.More = testTracks, ""
	head.answers++
	tt.Frame()
	s.settle(t, func(st player.State) bool { return len(st.Queue) == len(testTracks) && st.Index == 0 && st.Playing })
}

// TestLongPlaylistPageFails says how many tracks loaded when a page fails
// after its tries, and loads the rest when asked to try again.
func TestLongPlaylistPageFails(t *testing.T) {
	s, tt, f := longPage(t)
	f.page(t, s, tt)
	f.fail(t, s, tt, errors.New("youtube music: HTTP 503"))
	if !tt.HasText("Only 4 of this playlist's 6 tracks could be loaded: youtube music: HTTP 503") || tt.HasText("Loading tracks") {
		t.Fatalf("the page shows %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if !tt.HasText("Loading tracks · 4 of 6") {
		t.Errorf("trying again shows %q", tt.Texts())
	}
	f.page(t, s, tt)
	r := f.reading(s)
	if ids := func(ts []media.Track) []string {
		out := make([]string, len(ts))
		for i, tr := range ts {
			out[i] = tr.ID
		}
		return out
	}; !slices.Equal(ids(r.p.Tracks), ids(testTracks)) || r.err != nil || r.reading {
		t.Errorf("read on: %d tracks, %v", len(r.p.Tracks), r.err)
	}
	if tt.HasText("could be loaded") || !tt.HasText(testTracks[5].Title) {
		t.Errorf("the playlist read whole shows %q", tt.Texts())
	}
}

// TestLongPlaylistCut says where a playlist longer than MixMusic reads was
// cut.
func TestLongPlaylistCut(t *testing.T) {
	s := newTestShell(t)
	tracks := make([]media.Track, 0, media.MaxTracks)
	for i := range media.MaxTracks {
		tracks = append(tracks, testTrack("yt:cut"+strconv.Itoa(i), "Song "+strconv.Itoa(i), "Someone", "Something", media.SourceYouTube))
	}
	s.loads["playlist:yt:PLcut"] = &loaded[media.Playlist]{done: true, value: media.Playlist{ID: "yt:PLcut", Source: media.SourceYouTube,
		Title: "Everything", TrackCount: 25000, Tracks: tracks, Cut: true}}
	tt := tester(t, s)
	show(s, tt, playlistPath("yt:PLcut"))
	if !tt.HasText("This playlist has 25,000 tracks: MixMusic shows and plays its first 20,000.") {
		t.Errorf("the page shows %q", tt.Texts())
	}
}

// TestYouTubeLikedReadsOn shows the first page of the songs the user liked
// on YouTube Music at once, in the library, and the rest as they come.
func TestYouTubeLikedReadsOn(t *testing.T) {
	s := newTestShell(t)
	youtubeLibraryOf(s)
	f := slowReading(t, s, "youtube:liked")
	s.loads["youtube:liked"] = longHead("yt:LM")
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/library/youtube")
	if !tt.HasText("Loading tracks · 2 of 6") || !tt.HasText(testTracks[1].Title) || tt.HasText(testTracks[2].Title) {
		t.Fatalf("the first page shows %q", tt.Texts())
	}
	f.page(t, s, tt)
	f.page(t, s, tt)
	if tt.HasText("Loading tracks") || !tt.HasText(testTracks[5].Title) {
		t.Errorf("the whole list shows %q", tt.Texts())
	}
}

// TestLongQueueBuildsNearRows lists a queue of thousands of tracks, as
// Play makes of a long playlist, building only the entries near the view.
func TestLongQueueBuildsNearRows(t *testing.T) {
	s := newTestShell(t)
	tracks := make([]media.Track, 3000)
	for i := range tracks {
		tracks[i] = testTrack("yt:q"+strconv.Itoa(i), "Song "+strconv.Itoa(i), "Someone", "Something", media.SourceYouTube)
	}
	s.p.PlayTracks(tracks, 0)
	s.settle(t, func(st player.State) bool { return len(st.Queue) == len(tracks) && st.Playing })
	s.panel = "queue"
	tt := tester(t, s)
	tt.Frame()
	if !tt.HasText("Song 1") || tt.HasText("Song 2999") {
		t.Errorf("the queue shows %d texts, the last entry's %v", len(tt.Texts()), tt.HasText("Song 2999"))
	}
	if n := len(tt.Texts()); n > 1000 {
		t.Errorf("the queue built %d texts", n)
	}
	// Scrolled to its end, it shows the last.
	r, _ := tt.Find("Song 1")
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 1e6)
	tt.Frame()
	if !tt.HasText("Song 2999") || tt.HasText("Song 1") {
		t.Errorf("scrolled to its end, the queue shows the last entry %v, the first %v", tt.HasText("Song 2999"), tt.HasText("Song 1"))
	}
}
